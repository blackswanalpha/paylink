package p2p

// Config holds P2P networking configuration.
type Config struct {
	ListenAddrs    []string `json:"listenAddrs"`    // multiaddrs, default ["/ip4/0.0.0.0/tcp/9000"]
	BootstrapPeers []string `json:"bootstrapPeers"` // multiaddrs of bootstrap nodes
	MaxPeers       int      `json:"maxPeers"`       // default 50
	ProtocolID     string   `json:"protocolID"`     // default "/linkm/1.0.0"
	ChainID        string   `json:"chainId"`        // for rendezvous namespacing
	EnableDHT      bool     `json:"enableDHT"`      // default true
}

// DefaultConfig returns a default P2P configuration.
func DefaultConfig() Config {
	return Config{
		ListenAddrs: []string{"/ip4/0.0.0.0/tcp/9000"},
		MaxPeers:    50,
		ProtocolID:  "/linkm/1.0.0",
		ChainID:     "linkm-devnet",
		EnableDHT:   true,
	}
}
