package signaling

import (
	"context"
	"encoding/json"
)

// SignallingMessage represents a WebRTC signaling message
type SignallingMessage struct {
	Type string          `json:"type"`
	From *string         `json:"from,omitempty"`
	To   *string         `json:"to,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// MessageHandler is a function that handles a signaling message
type MessageHandler func(ctx context.Context, msg *SignallingMessage) error

// OnConnectHandler is a function called when the signaling client connects
type OnConnectHandler func(ctx context.Context) error

// OutboundMessage represents a message to be sent via signaling
type OutboundMessage struct {
	Type string
	From *string
	To   *string
	Data interface{}
}
