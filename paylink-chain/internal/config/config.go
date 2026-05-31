package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds the node configuration.
type Config struct {
	DataDir       string        `json:"dataDir"`
	GenesisFile   string        `json:"genesisFile"`
	RPCAddr       string        `json:"rpcAddr"`
	BlockInterval time.Duration `json:"blockInterval"`
	PrivateKey    string        `json:"privateKey"` // hex-encoded private key
	MempoolSize   int           `json:"mempoolSize"`

	// WebSocket / Event Bus configuration
	WSEnabled          bool `json:"wsEnabled"`
	WSMaxConnections   int  `json:"wsMaxConnections"`
	WSSubscriberBuffer int  `json:"wsSubscriberBuffer"`
	EventBusBuffer     int  `json:"eventBusBuffer"`
}

// DefaultConfig returns sensible defaults for development.
func DefaultConfig() *Config {
	return &Config{
		DataDir:            "./data",
		GenesisFile:        "",
		RPCAddr:            ":8545",
		BlockInterval:      1 * time.Second,
		PrivateKey:         "",
		MempoolSize:        10000,
		WSEnabled:          true,
		WSMaxConnections:   100,
		WSSubscriberBuffer: 256,
		EventBusBuffer:     4096,
	}
}

// LoadConfig loads configuration from a JSON file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig writes configuration to a JSON file.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
