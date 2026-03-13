package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	PanelAddr string `json:"panel_addr"`
	NodeID    string `json:"node_id"`
	CAPath    string `json:"ca_path"`
	CertPath  string `json:"cert_path"`
	KeyPath   string `json:"key_path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
