package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type AIConfig struct {
	ProviderURL  string   `json:"provider_url"`
	APIKey       string   `json:"api_key"`
	Models       []string `json:"models"`
	DefaultModel string   `json:"default_model"`
}

type Config struct {
	ShowHidden   bool                `json:"show_hidden"`
	AutoSave     bool                `json:"auto_save"`
	SoftWrap     bool                `json:"soft_wrap"`
	FormatOnSave bool                `json:"format_on_save"`
	SidebarWidth int                 `json:"sidebar_width,omitempty"`
	LastDir      string              `json:"last_dir,omitempty"`
	AI           AIConfig            `json:"ai"`
	Formatters   map[string][]string `json:"formatters,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		ShowHidden:   true,
		AutoSave:     true,
		SoftWrap:     true,
		FormatOnSave: false,
		SidebarWidth: 30,
		LastDir:      "",
		AI: AIConfig{
			ProviderURL:  "https://api.openai.com/v1",
			APIKey:       "",
			Models:       []string{"gpt-4o-mini", "gpt-4o"},
			DefaultModel: "gpt-4o-mini",
		},
		Formatters:   make(map[string][]string),
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
	if cfg.AI.ProviderURL == "" {
		cfg.AI.ProviderURL = "https://api.openai.com/v1"
	}
	if len(cfg.AI.Models) == 0 {
		cfg.AI.Models = []string{"gpt-4o-mini", "gpt-4o"}
	}
	if cfg.AI.DefaultModel == "" {
		cfg.AI.DefaultModel = "gpt-4o-mini"
	}
	if cfg.Formatters == nil {
		cfg.Formatters = make(map[string][]string)
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

	return atomicWriteFile(cfgPath, data, 0o600)
}
