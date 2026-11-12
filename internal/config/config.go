package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	ShowHidden   bool   `json:"show_hidden"`
	AutoSave     bool   `json:"auto_save"`
	SoftWrap     bool   `json:"soft_wrap"`
	SidebarWidth int    `json:"sidebar_width,omitempty"`
	LastDir      string `json:"last_dir,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		ShowHidden:   true,
		AutoSave:     true,
		SoftWrap:     true,
		SidebarWidth: 30,
		LastDir:      "",
	}
}

func GetConfigDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "tui-workflow"), nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() (*Config, error) {
	cfgPath, err := GetConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			_ = SaveConfig(cfg)
			return cfg, nil
		}
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}
	if cfg.SidebarWidth <= 0 {
		cfg.SidebarWidth = 30
	}
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	cfgPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cfgPath, data, 0644)
}
