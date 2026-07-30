package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is persisted user preferences for the manager process.
type Config struct {
	Listen     string `json:"listen"`
	Port       int    `json:"port"`
	ShowHidden bool   `json:"show_hidden"`
}

func Defaults() Config {
	return Config{Listen: "127.0.0.1", Port: 9876, ShowHidden: false}
}

// Path returns ~/.config/termux-manager/config.json
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "termux-manager", "config.json"), nil
}

func Load() Config {
	cfg := Defaults()
	p, err := Path()
	if err != nil {
		return cfg
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1"
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		cfg.Port = 9876
	}
	return cfg
}

func Save(cfg Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}
