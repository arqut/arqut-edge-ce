package wireguard

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
)

var (
	ErrBindClosed    = errors.New("bind is closed")
	ErrNoDataChannel = errors.New("no data channel available")
)

// WebRTCBind implements conn.Bind interface for WebRTC DataChannel transport
type WebRTCBind struct {
	logger      *device.Logger
	dataChannel *webrtc.DataChannel
	endpoint    *WebRTCEndpoint
	recvCh      chan []byte
	closed      chan struct{}
	closedFlag  bool
	mutex       sync.RWMutex
}

func NewWebRTCBind(logger *device.Logger) *WebRTCBind {
	return &WebRTCBind{
		logger:   logger,
		endpoint: &WebRTCEndpoint{},
		recvCh:   make(chan []byte, 100),
		closed:   make(chan struct{}),
	}
}

func (b *WebRTCBind) SetDataChannel(dc *webrtc.DataChannel) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.logger.Verbosef("WebRTCBind: Setting DataChannel, state: %s", dc.ReadyState())
	b.dataChannel = dc

	// Set up data channel message handler
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		select {
		case b.recvCh <- msg.Data:
			// b.logger.Verbosef("WebRTCBind: Received %d bytes from data channel", len(msg.Data))
			return
		case <-b.closed:
			return
		default:
			b.logger.Errorf("WebRTCBind: Receive buffer full, dropping packet")
		}
	})

	dc.OnError(func(err error) {
		b.logger.Errorf("WebRTCBind: data channel error: %v", err)
	})

	dc.OnClose(func() {
		b.logger.Verbosef("WebRTCBind: data channel closed")
		b.mutex.Lock()
		b.dataChannel = nil
		if !b.closedFlag {
			close(b.closed)
			b.closedFlag = true
		}
		b.mutex.Unlock()
	})
}

// Implement conn.Bind interface
func (b *WebRTCBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.logger.Verbosef("WebRTCBind: Open called with port %d, closed state: %v", port, b.closedFlag)

	// If bind was closed, reopen it
	if b.closedFlag {
		b.reopen()
	}

	// Return a single receive function since we're using a single channel
	receiveFunc := func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case data := <-b.recvCh:
			if len(data) > len(bufs[0]) {
				return 0, fmt.Errorf("WebRTCBind: packet too large! %d > %d", len(data), len(bufs[0]))
			}
			copy(bufs[0], data)
			sizes[0] = len(data)
			eps[0] = b.endpoint
			return 1, nil
		case <-time.After(100 * time.Millisecond):
			return 0, nil // Timeout, no packets available
		case <-b.closed:
			b.logger.Errorf("WebRTCBind: ReceiveFunc - bind closed (closed flag: %v)", b.closedFlag)
			return 0, net.ErrClosed
		}
	}

	return []conn.ReceiveFunc{receiveFunc}, port, nil
}

// BatchSize implements conn.Bind.BatchSize
func (b *WebRTCBind) BatchSize() int {
	return 1 // We process one packet at a time
}

// Close implements conn.Bind.Close
func (b *WebRTCBind) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.closedFlag {
		return nil
	}

	close(b.closed)
	b.closedFlag = true
	b.logger.Verbosef("WebRTCBind: closed")
	return nil
}

func (b *WebRTCBind) reopen() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Reset the bind to initial state
	b.closedFlag = false
	b.closed = make(chan struct{})
	b.recvCh = make(chan []byte, 100)
}

// SetMark implements conn.Bind.SetMark
func (b *WebRTCBind) SetMark(mark uint32) error {
	// Not applicable for WebRTC
	return nil
}

// Send implements conn.Bind.Send
func (b *WebRTCBind) Send(buff [][]byte, endpoint conn.Endpoint) error {
	b.mutex.RLock()
	closed := b.closedFlag
	dc := b.dataChannel
	b.mutex.RUnlock()

	if closed || dc == nil || dc.ReadyState() != webrtc.DataChannelStateOpen {
		return ErrBindClosed
	}

	if dc.ReadyState() != webrtc.DataChannelStateOpen {
		b.logger.Errorf("WebRTCBind: send while DC not open: %v", dc.ReadyState())
		return ErrBindClosed
	}

	for _, data := range buff {
		if len(data) == 0 {
			continue
		}
		cp := make([]byte, len(data))
		copy(cp, data)
		if err := dc.Send(cp); err != nil {
			b.logger.Errorf("WebRTCBind: Failed to send packet to client: %v", err)
			return err
		}
	}
	return nil
}

// ParseEndpoint implements conn.Bind.ParseEndpoint
func (b *WebRTCBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	return &WebRTCEndpoint{}, nil
}
