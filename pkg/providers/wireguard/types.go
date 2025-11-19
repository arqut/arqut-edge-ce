package wireguard

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
	EdgeIP    string `json:"edge_ip"`
	ClientIP  string `json:"client_ip"`
	Index     int    `json:"index"`
}

// TurnCredentials contains TURN server credentials
type TurnCredentials struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	TTL      int      `json:"ttl"`
	URLs     []string `json:"urls"`
}
