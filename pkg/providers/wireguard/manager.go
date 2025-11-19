package wireguard

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/signaling"
	"github.com/pion/webrtc/v4"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type SignallingMessageSender func(msgType string, from *string, to *string, data any) error

// Type aliases for signaling package types
type MessageHandler = signaling.MessageHandler
type OnConnectHandler = signaling.OnConnectHandler
type SignallingMessage = signaling.SignallingMessage

type NetworkService interface {
	SetInterfaceIPs(ips map[string]string)
	AddInterface(name string, ip string)
	RemoveInterface(name string)
}

type ConnectRequest struct {
	PeerID    string     `json:"peer_id"`
	AccountID string     `json:"account_id"`
	Config    PeerConfig `json:"config"`
}

type PeerConfig struct {
	Index     int    `json:"index,omitempty"`
	ID        string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	EdgeIP    string `json:"edge_ip,omitempty"`
	ClientIP  string `json:"client_ip,omitempty"`
}

type Manager struct {
	id       string
	peerType string

	sendSignalingMessage SignallingMessageSender

	privateKey  wgtypes.Key
	publicKey   wgtypes.Key
	clientPeers map[string]*PeerConfig
	wgConns     map[string]*WireGuardPeerToPeer
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc

	networkService  NetworkService
	netServiceMutex sync.RWMutex

	turnTicker *time.Ticker
	turnCreds  *TurnCredentials

	logger *logger.Logger
}

func NewManager(id string, ssender SignallingMessageSender, log *logger.Logger) (*Manager, error) {
	// Cleanup any stale WireGuard interfaces from previous runs
	cleanupStaleWireGuardInterfaces()

	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	app := &Manager{
		id:                   id,
		peerType:             "edge",
		sendSignalingMessage: ssender,
		privateKey:           privateKey,
		publicKey:            publicKey,
		wgConns:              make(map[string]*WireGuardPeerToPeer),
		clientPeers:          make(map[string]*PeerConfig),
		ctx:                  ctx,
		cancel:               cancel,
		turnTicker:           time.NewTicker(24 * time.Hour),
		logger:               log,
	}

	// Start periodic TURN credentials updater
	go app.updateTurnCreds()

	return app, nil
}

// fetchTurnCredentials requests TURN credentials from the cloud server
func (m *Manager) fetchTurnCredentials() {
	m.logger.Println("[WireGuard/Manager] Requesting TURN credentials...")

	// Send request for TURN credentials
	if err := m.sendSignalingMessageInternal("turn-request", nil, nil); err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to request TURN credentials: %v", err)
	}
}

// handleTurnResponse processes TURN credentials response from cloud server
func (m *Manager) handleTurnResponse(ctx context.Context, msg *SignallingMessage) error {
	var creds TurnCredentials
	if err := json.Unmarshal(msg.Data, &creds); err != nil {
		return fmt.Errorf("failed to unmarshal TURN credentials: %w", err)
	}

	m.turnCreds = &creds
	m.logger.Println("[WireGuard/Manager] Received TURN credentials")
	return nil
}

// updateTurnCreds periodically refreshes TURN credentials
func (m *Manager) updateTurnCreds() {
	m.logger.Println("[WireGuard/Manager] Starting TURN credentials updater...")

	// Initial fetch is now done via OnSignallingConnect handler

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.turnTicker.C:
			m.fetchTurnCredentials()
		}
	}
}

func (m *Manager) SetNetworkService(service NetworkService) {
	m.netServiceMutex.Lock()
	defer m.netServiceMutex.Unlock()
	m.networkService = service
	if service != nil {
		m.logger.Printf("[WireGuard/Manager] Network service set, interfaces: %v", m.GetInterfaceIPs())
		m.networkService.SetInterfaceIPs(m.GetInterfaceIPs())
	} else {
		m.logger.Printf("[WireGuard/Manager] Network service cleared")
	}
}

func (m *Manager) RegisterOnConnectHandler(register func(handler OnConnectHandler)) {
	register(m.OnSignallingConnect)
}

func (m *Manager) sendSignalingMessageInternal(msgType string, to *string, data any) error {
	return m.sendSignalingMessage(msgType, &m.id, to, data)
}

func (m *Manager) OnSignallingConnect(ctx context.Context) error {
	m.logger.Println("[WireGuard/Manager] Register with signaling server...")

	// Fetch TURN credentials on connect/reconnect
	m.fetchTurnCredentials()

	return nil
}

func (m *Manager) RegisterHandlers(register func(msgType string, handler MessageHandler)) {
	register("connect-request", m.handleConnectRequest)
	register("api-connect-request", m.handleAPIConnectRequest)
	register("offer", m.handleOffer)
	register("answer", m.handleAnswer)
	register("ice-candidate", m.handleICECandidate)
	register("turn-response", m.handleTurnResponse)
}

func (m *Manager) GetInterfacesNames() []*string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	var names []*string
	for _, conn := range m.wgConns {
		if conn.tunDevice != nil {
			name, err := conn.tunDevice.Name()
			if err == nil {
				names = append(names, &name)
			}
		}
	}
	return names
}

func (m *Manager) GetInterfaceIPs() map[string]string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]string)

	for peerID, conn := range m.wgConns {
		if conn.tunDevice != nil {
			name, err := conn.tunDevice.Name()
			if err == nil {
				// Get the peer config to find the IP address
				if peer, exists := m.clientPeers[peerID]; exists {
					result[name] = peer.EdgeIP
				}
			}
		}
	}

	return result
}

func (m *Manager) notifyInterfaceAdded(name, ip string) {
	m.netServiceMutex.RLock()
	service := m.networkService
	m.netServiceMutex.RUnlock()

	if service != nil {
		// Run in goroutine to avoid blocking
		go func() {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Printf("[WireGuard/Manager] Interface manager notification panic: %v", r)
				}
			}()
			service.AddInterface(name, ip)
		}()
	}
}

func (m *Manager) notifyInterfaceRemoved(name string) {
	m.netServiceMutex.RLock()
	service := m.networkService
	m.netServiceMutex.RUnlock()

	if service != nil {
		// Run in goroutine to avoid blocking
		go func() {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Printf("[WireGuard/Manager] Interface manager notification panic: %v", r)
				}
			}()
			service.RemoveInterface(name)
		}()
	}
}

func (m *Manager) handleConnectRequest(ctx context.Context, msg *SignallingMessage) error {
	return m.handleConnectRequestInner(ctx, msg, "connect-response")
}

func (m *Manager) handleAPIConnectRequest(ctx context.Context, msg *SignallingMessage) error {
	return m.handleConnectRequestInner(ctx, msg, "api-connect-response")
}

func (m *Manager) handleConnectRequestInner(_ context.Context, msg *SignallingMessage, resType string) error {
	peer := &PeerConfig{}
	copyStruct(msg.Data, peer)

	if existingPeer, exist := m.clientPeers[peer.ID]; exist {
		peer.Index = existingPeer.Index
		peer.EdgeIP = existingPeer.EdgeIP
		peer.ClientIP = existingPeer.ClientIP

		// Knowed issue: look like, android client lost the reference to the OS VPN connection
		// close if the connection exist from client peer
		m.closeConnectionFromPeer(peer.ID)
	}
	if peer.EdgeIP == "" {
		peer.Index = m.findAvailableIndex()
		peer.EdgeIP = m.generateIP(peer.Index, false)
		peer.ClientIP = m.generateIP(peer.Index, true)
	}
	m.clientPeers[peer.ID] = peer

	// send response
	if err := m.sendSignalingMessageInternal(
		resType,
		&peer.ID,
		&PeerConfig{
			Index:     peer.Index,
			ID:        m.id,
			Type:      "edge",
			PublicKey: m.publicKey.String(),
			ClientIP:  peer.ClientIP,
			EdgeIP:    peer.EdgeIP,
		},
	); err != nil {
		return err
	}

	m.logger.Printf("[WireGuard/Manager] new connect request: %v", "{...}")
	m.logger.Printf("[WireGuard/Manager] client peer list updated: %d", len(m.clientPeers))
	return nil
}

func (m *Manager) handleOffer(ctx context.Context, msg *SignallingMessage) error {
	m.logger.Printf("[WireGuard/Manager] Received offer from %s", *msg.From)

	if msg.From == nil || *msg.From == "" {
		m.logger.Printf("[WireGuard/Manager] Invalid offer from empty peer ID")
		return fmt.Errorf("invalid offer from empty peer ID")
	}

	// Check if we already have a connection to this peer
	m.mutex.RLock()
	wgConn, exists := m.wgConns[*msg.From]
	m.mutex.RUnlock()

	if exists {
		if wgConn.connSate == webrtc.PeerConnectionStateConnected {
			m.logger.Printf("[WireGuard/Manager] Already connected to peer %s", *msg.From)
			return nil
		}
		m.closeConnectionFromPeer(*msg.From)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get peer info
	clientPeer, exists := m.clientPeers[*msg.From]
	if !exists {
		m.logger.Printf("[WireGuard/Manager] Unknown peer %s", *msg.From)
		return fmt.Errorf("unknown peer %s", *msg.From)
	}

	wgConn, err := newWireGuardPeerToPeer(m, clientPeer)
	if err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to create peer connection for %s: %v", clientPeer.ID, err)
		return fmt.Errorf("failed to create peer connection for %s: %v", clientPeer.ID, err)
	}

	pc := wgConn.peerConnection

	// Set up WebRTC event handlers
	connectCallback := func() {
		if name, err := wgConn.tunDevice.Name(); err == nil {
			m.notifyInterfaceAdded(name, clientPeer.EdgeIP)
			m.logger.Printf("[WireGuard/Manager] TUN device %s is ready for peer %s", name, wgConn.targetID)
		}
	}
	wgConn.setupWebRTCHandlersForAnswer(clientPeer, connectCallback)
	m.wgConns[clientPeer.ID] = wgConn

	// Parse and set remote description (offer)
	var offerData map[string]any
	if err := json.Unmarshal(msg.Data, &offerData); err != nil {
		m.logger.Printf("[WireGuard/Manager] Invalid offer data from %s: %v", *msg.From, err)
		return fmt.Errorf("invalid offer data from %s: %v", *msg.From, err)
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerData["sdp"].(string),
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to set remote description: %v", err)
		return fmt.Errorf("failed to set remote description: %v", err)
	}

	// Create and send answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to create answer: %v", err)
		return fmt.Errorf("failed to create answer: %v", err)
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to set local description: %v", err)
		return fmt.Errorf("failed to set local description: %v", err)
	}

	if err := m.sendSignalingMessageInternal(
		"answer",
		msg.From,
		answer,
	); err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to send answer: %v", err)
		return fmt.Errorf("failed to send answer: %v", err)
	}

	m.logger.Printf("[WireGuard/Manager] Sent answer to %s", *msg.From)
	return nil
}

func (m *Manager) handleAnswer(ctx context.Context, msg *SignallingMessage) error {
	m.mutex.RLock()
	wgConn := m.wgConns[*msg.From]
	m.mutex.RUnlock()

	if wgConn == nil {
		return fmt.Errorf("not connected to peer %s", *msg.From)
	}

	var answerData map[string]any
	if err := json.Unmarshal(msg.Data, &answerData); err != nil {
		return fmt.Errorf("invalid answer data from %s: %v", *msg.From, err)
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerData["sdp"].(string),
	}

	if err := wgConn.peerConnection.SetRemoteDescription(answer); err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to set remote description: %v", err)
		return fmt.Errorf("failed to set remote description: %v", err)
	}

	return nil
}

func (m *Manager) handleICECandidate(ctx context.Context, msg *SignallingMessage) error {
	m.mutex.RLock()
	wgConn := m.wgConns[*msg.From]
	m.mutex.RUnlock()

	if wgConn == nil {
		return fmt.Errorf("not connected to peer %s", *msg.From)
	}

	var candidateData map[string]any
	if err := json.Unmarshal(msg.Data, &candidateData); err != nil {
		m.logger.Printf("[WireGuard/Manager] Invalid ICE candidate data from %s: %v", *msg.From, err)
		return fmt.Errorf("invalid ICE candidate data from %s: %v", *msg.From, err)
	}

	// Handle potentially nil fields safely
	var sdpMid *string
	var sdpMLineIndex *uint16

	if mid, exists := candidateData["sdpMid"]; exists && mid != nil {
		if midStr, ok := mid.(string); ok {
			sdpMid = &midStr
		}
	}

	if mlineIdx, exists := candidateData["sdpMLineIndex"]; exists && mlineIdx != nil {
		if idx, ok := mlineIdx.(float64); ok {
			idxUint16 := uint16(idx)
			sdpMLineIndex = &idxUint16
		}
	}

	candidateStr, ok := candidateData["candidate"].(string)
	if !ok {
		m.logger.Printf("[WireGuard/Manager] Invalid candidate string from %s", *msg.From)
		return fmt.Errorf("invalid candidate string from %s", *msg.From)
	}

	candidate := webrtc.ICECandidateInit{
		Candidate:     candidateStr,
		SDPMid:        sdpMid,
		SDPMLineIndex: sdpMLineIndex,
	}

	if err := wgConn.peerConnection.AddICECandidate(candidate); err != nil {
		m.logger.Printf("[WireGuard/Manager] Failed to add ICE candidate from %s: %v", *msg.From, err)
		return err
	}

	return nil
}

func (m *Manager) closeConnectionFromPeer(targetID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	wgConn, exists := m.wgConns[targetID]
	if exists {
		m.logger.Printf("[WireGuard/Manager] Closing peer connection %s", targetID)
		wgConn.close()

		var interfaceName string
		if wgConn.tunDevice != nil {
			if name, err := wgConn.tunDevice.Name(); err == nil {
				interfaceName = name
			}
		}

		delete(m.wgConns, targetID)
		delete(m.clientPeers, targetID)

		if interfaceName != "" {
			m.notifyInterfaceRemoved(interfaceName)
		}

		m.logger.Printf("[WireGuard/Manager] peer %s disconected", targetID)
	}
}

func (m *Manager) findAvailableIndex() int {
	used := make(map[int]bool)
	for _, pc := range m.clientPeers {
		used[pc.Index] = true
	}
	for i := 0; i < 255; i++ {
		if !used[i] {
			return i
		}
	}
	return 0
}

func (m *Manager) generateIP(index int, isClient bool) string {
	if isClient {
		return fmt.Sprintf("10.0.%d.2", index)
	}
	return fmt.Sprintf("10.0.%d.1", index)
}

func (m *Manager) Stop() {
	m.logger.Printf("[WireGuard/Manager] closing...")
	m.cancel()

	// Close peer connections first
	for id, wgConn := range m.wgConns {
		m.logger.Printf("[WireGuard/Manager] Closing peer connection %s", id)
		wgConn.close()
	}

	m.logger.Printf("[WireGuard/Manager] closed")
}

// GetConnectedPeers returns a list of connected peer IDs
func (m *Manager) GetConnectedPeers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	peers := make([]string, 0, len(m.wgConns))
	for peerID := range m.wgConns {
		peers = append(peers, peerID)
	}
	return peers
}

// GetPeerInfo returns information about a specific peer
func (m *Manager) GetPeerInfo(peerID string) (*PeerInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	peer, exists := m.clientPeers[peerID]
	if !exists {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}

	return &PeerInfo{
		ID:        peer.ID,
		PublicKey: peer.PublicKey,
		EdgeIP:    peer.EdgeIP,
		ClientIP:  peer.ClientIP,
		Index:     peer.Index,
	}, nil
}

// DisconnectPeer disconnects a specific peer
func (m *Manager) DisconnectPeer(peerID string) error {
	m.mutex.RLock()
	_, exists := m.wgConns[peerID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("peer not connected: %s", peerID)
	}

	m.closeConnectionFromPeer(peerID)
	return nil
}
