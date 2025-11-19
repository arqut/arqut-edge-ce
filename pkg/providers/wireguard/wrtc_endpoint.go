package wireguard

import (
	"net/netip"
)

// WebRTCEndpoint implements conn.Endpoint interface
type WebRTCEndpoint struct{}

func (e *WebRTCEndpoint) ClearSrc() {
	// Not applicable for WebRTC
}

func (e *WebRTCEndpoint) SrcToString() string {
	return "webrtc"
}

func (e *WebRTCEndpoint) DstToString() string {
	return "webrtc"
}

func (e *WebRTCEndpoint) DstToBytes() []byte {
	return []byte("webrtc")
}

func (e *WebRTCEndpoint) DstIP() netip.Addr {
	return netip.MustParseAddr("10.0.0.2") // Dummy IP
}

func (e *WebRTCEndpoint) SrcIP() netip.Addr {
	return netip.MustParseAddr("10.0.0.1") // Dummy IP
}
