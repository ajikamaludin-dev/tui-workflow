package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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

	cfg.ShowHidden = false
	cfg.AutoSave = false
	cfg.LastDir = "/custom/path"
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error saving config: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading saved config: %v", err)
	}
	if loaded.ShowHidden != false || loaded.AutoSave != false || loaded.LastDir != "/custom/path" {
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
