package wireguard

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type WireGuardPeerToPeer struct {
	targetID       string
	targetPeer     *PeerConfig
	manager        *Manager
	tunDevice      tun.Device
	wgDevice       *device.Device
	peerConnection *webrtc.PeerConnection
	dataChannel    *webrtc.DataChannel
	webrtcBind     *WebRTCBind
	logger         *device.Logger
	connSate       webrtc.PeerConnectionState
	mutex          sync.RWMutex
}

func newWireGuardPeerToPeer(manager *Manager, peer *PeerConfig) (*WireGuardPeerToPeer, error) {
	// Create peer connection
	pc, err := createWebrtcPeerConnection(manager.turnCreds)
	if err != nil {
		return nil, err
	}

	tunName := createTunNameFromPeerID(peer.ID)
	logger := device.NewLogger(device.LogLevelError, "["+tunName+"]")
	bind := NewWebRTCBind(logger)

	return &WireGuardPeerToPeer{
		manager:        manager,
		peerConnection: pc,
		targetID:       peer.ID,
		targetPeer:     peer,
		logger:         logger,
		webrtcBind:     bind,
	}, nil
}

func (p *WireGuardPeerToPeer) setupWebRTCHandlersForAnswer(targetPeer *PeerConfig, callBack func()) {
	pc := p.peerConnection

	pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		log.Printf("WG WebRTC: ICE state with %s: %s", p.targetID, s)
		switch s {
		case webrtc.ICEConnectionStateFailed:
			log.Println("WG WebRTC: Direct connection failed. Fallback to TURN might be attempted if available...")
		case webrtc.ICEConnectionStateConnected:
			_, _ = pc.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
			log.Printf("WG WebRTC: Connection succeeded! It could be via STUN (P2P) or TURN (Relay).")
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("----------------------------------------------------------------------")
		log.Printf("WG WebRTC: Connection state with %s: %s", p.targetID, state)
		log.Println("----------------------------------------------------------------------")

		p.mutex.Lock()
		p.connSate = state
		p.mutex.Unlock()

		switch state {
		case webrtc.PeerConnectionStateConnected:
			// no-op; wait for DataChannel open path
		case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed:
			// Stop traffic first so WG doesn’t try to send on a dead DC
			if p.webrtcBind != nil {
				_ = p.webrtcBind.Close()
			}
			p.manager.closeConnectionFromPeer(p.targetID)
		}
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			// Parse the candidate string to get a Candidate object
			// Alternatively, you can use c.ToJSON() to get a ICECandidateInit struct
			// log.Printf("New ICE Candidate: %s \n", "{...}")
			p.manager.sendSignalingMessageInternal("ice-candidate", &p.targetID, candidate.ToJSON())
		}
	})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("WG WebRTC: Received data channel from %s", p.targetID)
		if dc.Label() != "wireguard" {
			return
		}
		p.dataChannel = dc

		dc.OnError(func(err error) {
			log.Printf("WG WebRTC: Data channel error with %s: %v", p.targetID, err)
		})

		dc.OnOpen(func() {
			log.Printf("WG WebRTC: Data channel with %s opened", p.targetID)
			p.webrtcBind.SetDataChannel(dc)

			go func() {
				// Small settle delay, then verify still open/connected
				time.Sleep(200 * time.Millisecond)

				p.mutex.RLock()
				state := p.connSate
				p.mutex.RUnlock()
				if dc.ReadyState() != webrtc.DataChannelStateOpen || state != webrtc.PeerConnectionStateConnected {
					return // session already gone or not stable
				}

				if p.wgDevice == nil {
					if err := p.setupWireGuardConn(targetPeer, func() {
						// Only announce interface if still connected & DC open
						p.mutex.RLock()
						s2 := p.connSate
						p.mutex.RUnlock()
						if dc.ReadyState() == webrtc.DataChannelStateOpen && s2 == webrtc.PeerConnectionStateConnected {
							callBack()
						}
					}); err != nil {
						log.Printf("WG WebRTC: Error setup connection: %v", err)
					}
				}
			}()
		})
	})
}

func (p *WireGuardPeerToPeer) setupWireGuardConn(peerConfig *PeerConfig, callBack func()) error {
	tunName := createTunNameFromPeerID(peerConfig.ID)
	tunDevice, err := createTUNInterface(tunName, peerConfig.EdgeIP)
	if err != nil {
		// If TUN creation fails due to "device busy", try to cleanup stale interface
		if strings.Contains(err.Error(), "device or resource busy") {
			log.Printf("WG Manager: TUN device %s busy, attempting cleanup", tunName)
			if cleanupErr := forceCleanupTUNInterface(tunName); cleanupErr != nil {
				log.Printf("WG Manager: Failed to cleanup stale TUN interface %s: %v", tunName, cleanupErr)
			} else {
				// Retry after cleanup
				time.Sleep(200 * time.Millisecond)
				tunDevice, err = createTUNInterface(tunName, peerConfig.EdgeIP)
			}
		}
		if err != nil {
			return fmt.Errorf("failed to create TUN interface %s: %w", tunName, err)
		}
	}

	p.tunDevice = tunDevice
	p.wgDevice = device.NewDevice(tunDevice, p.webrtcBind, p.logger)

	// Configure device with our private key
	privateKeyHex := hex.EncodeToString(p.manager.privateKey[:])
	wgConfig := fmt.Sprintf("private_key=%s\n", privateKeyHex)
	publicKey, err := wgtypes.ParseKey(peerConfig.PublicKey)
	if err != nil {
		log.Printf("WG Peer: failed to parse edge public key: %s", peerConfig.PublicKey)
		// Cleanup on configuration error
		p.cleanup()
		return err
	}

	// Add the peer configuration with specific allowed IP for this peer
	publicKeyHex := hex.EncodeToString(publicKey[:])
	allowedIP := fmt.Sprintf("%s/32", peerConfig.ClientIP)
	wgConfig += fmt.Sprintf("public_key=%s\nallowed_ip=%s\nendpoint=webrtc://peer\npersistent_keepalive_interval=25\n", publicKeyHex, allowedIP)
	log.Printf("WG Manager: IpcSet!\n%s", wgConfig)
	if err := p.wgDevice.IpcSet(wgConfig); err != nil {
		log.Printf("WG Manager: Failed to configure WireGuard peer %s: %v", p.targetID, err)
		// Cleanup on configuration error
		p.cleanup()
		return err
	}
	if err := p.wgDevice.Up(); err != nil {
		log.Printf("WG Manager: failed to bring up WG-device: %v", err)
		// Cleanup on device up error
		p.cleanup()
		return err
	}

	callBack()

	return nil
}

func (p *WireGuardPeerToPeer) cleanup() {
	// Cleanup resources in reverse order of creation
	if p.wgDevice != nil {
		p.wgDevice.Close()
		p.wgDevice = nil
	}
	if p.tunDevice != nil {
		if name, err := p.tunDevice.Name(); err == nil {
			p.tunDevice.Close()
			p.tunDevice = nil
			// Force cleanup the interface if it still exists
			go func() {
				time.Sleep(100 * time.Millisecond)
				forceCleanupTUNInterface(name)
			}()
		} else {
			p.tunDevice.Close()
			p.tunDevice = nil
		}
	}
}

func (p *WireGuardPeerToPeer) close() {
	var interfaceName string

	// Get interface name before closing for cleanup
	if p.tunDevice != nil {
		if name, err := p.tunDevice.Name(); err == nil {
			interfaceName = name
		}
	}

	if p.webrtcBind != nil {
		p.webrtcBind.Close()
	}
	if p.peerConnection != nil {
		p.peerConnection.Close()
	}
	if p.wgDevice != nil {
		p.wgDevice.Close()
	}
	if p.tunDevice != nil {
		p.tunDevice.Close()
	}

	// Ensure interface cleanup with retry logic
	if interfaceName != "" {
		go p.retryCleanupInterface(interfaceName, 3)
	}
}

func (p *WireGuardPeerToPeer) retryCleanupInterface(name string, maxRetries int) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Wait before each attempt to allow OS to release resources
		time.Sleep(time.Duration(attempt*100) * time.Millisecond)

		if err := forceCleanupTUNInterface(name); err != nil {
			log.Printf("WG Manager: Cleanup attempt %d/%d failed for interface %s: %v",
				attempt, maxRetries, name, err)
			if attempt == maxRetries {
				log.Printf("WG Manager: Failed to cleanup interface %s after %d attempts",
					name, maxRetries)
			}
		} else {
			log.Printf("WG Manager: Successfully cleaned up interface %s on attempt %d",
				name, attempt)
			break
		}
	}
}
