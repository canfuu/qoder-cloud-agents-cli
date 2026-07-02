package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultAPIBase = "https://api.qoder.com/api/v1/cloud"
)

type Config struct {
	Token   string `json:"token"`
	APIBase string `json:"api_base,omitempty"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "qca"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	// Check env var first
	if token := os.Getenv("QODER_PAT"); token != "" {
		apiBase := os.Getenv("QODER_API_BASE")
		if apiBase == "" {
			apiBase = DefaultAPIBase
		}
		return &Config{Token: token, APIBase: apiBase}, nil
	}

	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("not logged in. Run 'qca auth login' first")
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.APIBase == "" {
		cfg.APIBase = DefaultAPIBase
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func GetToken() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.Token, nil
}

func GetAPIBase() string {
	cfg, err := Load()
	if err != nil {
		return DefaultAPIBase
	}
	return cfg.APIBase
}
