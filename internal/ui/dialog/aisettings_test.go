package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"tui-workflow/internal/config"
)

func TestAISettingsDialog_Lifecycle(t *testing.T) {
	d := NewAISettingsDialog()
	if d.Active {
		t.Errorf("expected dialog to start inactive")
	}

	initCfg := config.AIConfig{
		ProviderURL:  "https://custom.ai/v1",
		APIKey:       "secret-key",
		Models:       []string{"model-a", "model-b"},
		DefaultModel: "model-a",
	}

	d.Open(initCfg)
	if !d.Active {
		t.Errorf("expected dialog to be active after Open()")
	}

	rendered := d.Render()
	if len(rendered) == 0 {
		t.Errorf("expected non-empty rendered dialog")
	}

	// Test Tab focus cycle
	d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if d.Focused != AIFieldAPIKey {
		t.Errorf("expected focused to be AIFieldAPIKey, got %v", d.Focused)
	}

	// Test Ctrl+S save
	saved, canceled, cfg, _ := d.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if !saved || canceled {
		t.Errorf("expected saved=true canceled=false, got saved=%v canceled=%v", saved, canceled)
	}
	if cfg.ProviderURL != "https://custom.ai/v1" || cfg.APIKey != "secret-key" || cfg.DefaultModel != "model-a" {
		t.Errorf("unexpected config returned: %+v", cfg)
	}
	if d.Active {
		t.Errorf("expected dialog to be inactive after save")
	}

	// Test Esc cancel
	d.Open(initCfg)
	saved, canceled, _, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if saved || !canceled {
		t.Errorf("expected saved=false canceled=true, got saved=%v canceled=%v", saved, canceled)
	}
	if d.Active {
		t.Errorf("expected dialog to be inactive after cancel")
	}
}

func TestAISettingsDialog_MouseInteraction(t *testing.T) {
	d := NewAISettingsDialog()
	d.SetScreenSize(100, 30)
	initCfg := config.AIConfig{
		ProviderURL:  "https://test.ai/v1",
		APIKey:       "key",
		Models:       []string{"m1"},
		DefaultModel: "m1",
	}
	d.Open(initCfg)

	// atX = (100 - 62)/2 = 19
	// atY = (30 - 19)/2 = 5
	// Click on Provider URL input (relY=5 -> msg.Y=10)
	d.Update(tea.MouseClickMsg{X: 25, Y: 10, Button: tea.MouseLeft})
	if d.Focused != AIFieldProvider {
		t.Errorf("expected focus AIFieldProvider, got %v", d.Focused)
	}

	// Click on API Key input (relY=8 -> msg.Y=13)
	d.Update(tea.MouseClickMsg{X: 25, Y: 13, Button: tea.MouseLeft})
	if d.Focused != AIFieldAPIKey {
		t.Errorf("expected focus AIFieldAPIKey, got %v", d.Focused)
	}

	// Click on Models input (relY=11 -> msg.Y=16)
	d.Update(tea.MouseClickMsg{X: 25, Y: 16, Button: tea.MouseLeft})
	if d.Focused != AIFieldModels {
		t.Errorf("expected focus AIFieldModels, got %v", d.Focused)
	}

	// Click on Default Model input (relY=14 -> msg.Y=19)
	d.Update(tea.MouseClickMsg{X: 25, Y: 19, Button: tea.MouseLeft})
	if d.Focused != AIFieldDefaultModel {
		t.Errorf("expected focus AIFieldDefaultModel, got %v", d.Focused)
	}

	// Click on Save button (relY=16 -> msg.Y=21, relX=10 -> msg.X=29)
	saved, canceled, _, _ := d.Update(tea.MouseClickMsg{X: 29, Y: 21, Button: tea.MouseLeft})
	if !saved || canceled {
		t.Errorf("expected saved=true on click Save button, got saved=%v canceled=%v", saved, canceled)
	}

	// Click outside dialog closes/cancels dialog
	d.Open(initCfg)
	saved, canceled, _, _ = d.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	if saved || !canceled {
		t.Errorf("expected canceled=true on click outside dialog, got saved=%v canceled=%v", saved, canceled)
	}
}
