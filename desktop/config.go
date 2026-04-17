package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type Config struct {
	DeviceID    string `json:"device_id"`
	HubURL      string `json:"hub_url"`
	Country     string `json:"country"`
	DeviceType  string `json:"device_type"`
	NodeSecret  string `json:"node_secret"`
}

var configPath = "config.json"

func LoadConfig() *Config {
	// Try to load existing config
	data, err := os.ReadFile(configPath)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			// Environment variable always overrides config file for security
			if envSecret := os.Getenv("NODE_SECRET"); envSecret != "" {
				cfg.NodeSecret = envSecret
			}
			return &cfg
		}
	}

	// Create new config if missing or corrupted
	cfg := &Config{
		DeviceID:   uuid.New().String(),
		HubURL:     "ws://api.exra.net/ws", // Default staging hub
		Country:    "US",                   // Default, will be updated by server/detect
		DeviceType: "pc",
		NodeSecret: os.Getenv("NODE_SECRET"),
	}
	if cfg.NodeSecret == "" {
		cfg.NodeSecret = "default_node_secret"
	}

	SaveConfig(cfg)
	log.Printf("[Config] Generated new device_id: %s", cfg.DeviceID)
	return cfg
}

func SaveConfig(cfg *Config) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func getExecutableDir() string {
	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(ex)
}
