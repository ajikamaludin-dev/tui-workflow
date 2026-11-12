package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origXdg := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("HOME", tmpDir)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("XDG_CONFIG_HOME", origXdg)
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if !cfg.ShowHidden {
		t.Errorf("expected default ShowHidden to be true, got false")
	}
	if !cfg.AutoSave {
		t.Errorf("expected default AutoSave to be true, got false")
	}
	if cfg.SidebarWidth != 30 {
		t.Errorf("expected default SidebarWidth to be 30, got %d", cfg.SidebarWidth)
	}

	cfg.ShowHidden = false
	cfg.AutoSave = false
	cfg.SidebarWidth = 35
	cfg.LastDir = "/custom/path"
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error saving config: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading saved config: %v", err)
	}
	if loaded.ShowHidden != false || loaded.AutoSave != false || loaded.SidebarWidth != 35 || loaded.LastDir != "/custom/path" {
		t.Errorf("saved config does not match loaded: %+v", loaded)
	}

	cfgPath, err := GetConfigPath()
	if err != nil {
		t.Fatalf("error getting config path: %v", err)
	}
	if !filepath.IsAbs(cfgPath) {
		t.Errorf("expected absolute path, got %s", cfgPath)
	}
}
