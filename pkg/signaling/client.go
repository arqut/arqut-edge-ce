package signaling

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/utils"
	"github.com/gorilla/websocket"
)

// Client handles WebSocket communication with the cloud server
type Client struct {
	cloudURL   string
	apiPath    string // API path prefix (e.g., "/api/v1" or "/v1")
	apiKey     string
	edgeID     string
	serverAddr string
	conn       *websocket.Conn
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc

	messageHandlers   map[string]MessageHandler
	onConnectHandlers []OnConnectHandler
	handlerMutex      sync.RWMutex

	outboundChan chan *OutboundMessage

	logger *logger.Logger

	reconnecting   bool
	reconnectMutex sync.Mutex
}

// NewClient creates a new signaling client with default API path "/api/v1"
func NewClient(cloudURL string, log *logger.Logger) (*Client, error) {
	return NewClientWithPath(cloudURL, "/api/v1", log)
}

// NewClientWithPath creates a new signaling client with custom API path
func NewClientWithPath(cloudURL string, apiPath string, log *logger.Logger) (*Client, error) {
	return &Client{
		cloudURL:        cloudURL,
		apiPath:         apiPath,
		messageHandlers: make(map[string]MessageHandler),
		outboundChan:    make(chan *OutboundMessage, 100), // Buffered channel for non-blocking sends
		logger:          log,
	}, nil
}

// Connect establishes WebSocket connection to the cloud server
// This function will retry indefinitely in the background if initial connection fails
func (c *Client) Connect(ctx context.Context, apiKey string, edgeID string, serverAddr string) {
	// Store edge ID and API key for reconnection
	c.apiKey = apiKey
	c.edgeID = edgeID
	c.serverAddr = serverAddr

	// Create a new context for the client operations, derived from parent context
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Attempt initial connection
	if err := c.connectOnce(ctx); err != nil {
		// If initial connection fails, start retry loop in background
		c.logger.Printf("[Signaling] Connection failed: %v", err)
		c.logger.Printf("[Signaling] Will retry in background...")
		go c.reconnect()
		// Don't return error - service should continue running
		return
	}
}

// connectOnce performs a single connection attempt
func (c *Client) connectOnce(ctx context.Context) error {
	// Convert http:// to ws:// and https:// to wss://
	cloudURL := c.cloudURL
	if after, ok := strings.CutPrefix(cloudURL, "http://"); ok {
		cloudURL = "ws://" + after
	} else if after, ok := strings.CutPrefix(cloudURL, "https://"); ok {
		cloudURL = "wss://" + after
	}

	// Append edgeServerAddr as a query parameter
	host, portStr, err := net.SplitHostPort(c.serverAddr)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %w", err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return fmt.Errorf("failed to lookup server port: %w", err)
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
		// server listens on all interfaces, try to get actual local IPs
		localIPs, err := utils.GetLocalIPs(true)
		if err == nil && len(localIPs) > 0 {
			host = strings.Join(localIPs, ",")
		}
	}

	wsURL := fmt.Sprintf("%s%s/signaling/ws/edge?id=%s&host=%s&port=%d&os=%s", cloudURL, c.apiPath, c.edgeID, host, port, runtime.GOOS)

	c.logger.Printf("[Signaling] Connecting to %s", wsURL)

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	// Add Authorization header with API key for authentication
	headers := make(map[string][]string)
	headers["Authorization"] = []string{"Bearer " + c.apiKey}

	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to signaling server: %w", err)
	}

	c.mutex.Lock()
	c.conn = conn
	c.mutex.Unlock()

	c.logger.Printf("[Signaling] Connected to cloud server")

	// Call onConnect handlers
	c.handlerMutex.RLock()
	handlers := make([]OnConnectHandler, len(c.onConnectHandlers))
	copy(handlers, c.onConnectHandlers)
	c.handlerMutex.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx); err != nil {
			c.logger.Printf("[Signaling] OnConnect handler error: %v", err)
		}
	}

	// Start message reader
	go c.readMessages()

	// Start keepalive
	go c.keepalive()

	// Start outbound message processor
	go c.processOutboundMessages()

	return nil
}

// readMessages reads incoming messages from WebSocket
func (c *Client) readMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.mutex.RLock()
		conn := c.conn
		c.mutex.RUnlock()

		if conn == nil {
			// Connection is nil, exit this reader goroutine
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			c.logger.Printf("[Signaling] Read error: %v", err)
			// Trigger reconnection and exit this goroutine
			go c.reconnect()
			return
		}

		var msg SignallingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Printf("[Signaling] Failed to unmarshal message: %v", err)
			continue
		}

		// Handle message
		c.handlerMutex.RLock()
		handler, exists := c.messageHandlers[msg.Type]
		c.handlerMutex.RUnlock()

		if exists {
			if err := handler(c.ctx, &msg); err != nil {
				c.logger.Printf("[Signaling] Handler error for %s: %v", msg.Type, err)
			}
		} else {
			c.logger.Printf("[Signaling] No handler for message type: %s", msg.Type)
		}
	}
}

// SendMessage sends a signaling message
func (c *Client) SendMessage(msgType string, from *string, to *string, data any) error {
	c.mutex.RLock()
	conn := c.conn
	c.mutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected to signaling server")
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	msg := SignallingMessage{
		Type: msgType,
		From: from,
		To:   to,
		Data: dataBytes,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// RegisterMessageHandlers registers message handlers using the provided callback
func (c *Client) RegisterMessageHandlers(callback func(msgType string, handler MessageHandler)) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	// The callback will be called to register each handler
	// We store the callback for use when registering handlers
	callback("", nil) // This is a placeholder - the actual registration happens via SetMessageHandler
}

// SetMessageHandler adds a message handler for a specific type
func (c *Client) SetMessageHandler(msgType string, handler MessageHandler) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	c.messageHandlers[msgType] = handler
}

// AddOnConnectHandler adds a handler to be called on connection
func (c *Client) AddOnConnectHandler(handler OnConnectHandler) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	c.onConnectHandlers = append(c.onConnectHandlers, handler)
}

// keepalive sends periodic ping messages
func (c *Client) keepalive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mutex.Lock()
			if c.conn != nil {
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					c.logger.Printf("[Signaling] Ping failed: %v", err)
				}
			}
			c.mutex.Unlock()
		}
	}
}

// processOutboundMessages processes messages from the outbound channel
func (c *Client) processOutboundMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.outboundChan:
			// Check if connected before attempting to send
			c.mutex.RLock()
			connected := c.conn != nil
			c.mutex.RUnlock()

			if !connected {
				c.logger.Printf("[Signaling] Skipping outbound message (disconnected): %s", msg.Type)
				continue
			}

			// Send the message
			if err := c.SendMessage(msg.Type, msg.From, msg.To, msg.Data); err != nil {
				c.logger.Printf("[Signaling] Failed to send outbound message %s: %v", msg.Type, err)
			}
		}
	}
}

// reconnect attempts to reconnect to the signaling server with exponential backoff
func (c *Client) reconnect() {
	// Check if already reconnecting
	c.reconnectMutex.Lock()
	if c.reconnecting {
		c.reconnectMutex.Unlock()
		return
	}
	c.reconnecting = true
	c.reconnectMutex.Unlock()

	defer func() {
		c.reconnectMutex.Lock()
		c.reconnecting = false
		c.reconnectMutex.Unlock()
	}()

	c.mutex.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mutex.Unlock()

	c.logger.Printf("[Signaling] Attempting to reconnect...")

	// Exponential backoff parameters
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	attempt := 1

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Println("[Signaling] Reconnection stopped - context cancelled")
			return
		default:
		}

		c.logger.Printf("[Signaling] Reconnection attempt #%d...", attempt)

		if err := c.connectOnce(c.ctx); err != nil {
			c.logger.Printf("[Signaling] Reconnect failed: %v (retrying in %v)", err, backoff)

			// Wait before retry with exponential backoff
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(backoff):
			}

			// Increase backoff exponentially, up to maxBackoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			attempt++
			continue
		}

		c.logger.Printf("[Signaling] Reconnected successfully on attempt #%d", attempt)
		return
	}
}

// Close closes the signaling client connection
func (c *Client) Close() {
	c.cancel()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.logger.Printf("[Signaling] Connection closed")
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.conn != nil
}

// OutboundChannel returns the send-only channel for outbound messages
func (c *Client) OutboundChannel() chan<- *OutboundMessage {
	return c.outboundChan
}
