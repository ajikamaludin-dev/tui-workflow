package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tui-workflow/internal/config"
	"tui-workflow/internal/ui"
)

type AISettingsField int

const (
	AIFieldProvider AISettingsField = iota
	AIFieldAPIKey
	AIFieldModels
	AIFieldDefaultModel
	AIFieldBtnSave
	AIFieldBtnCancel
)

type AISettingsDialog struct {
	Active            bool
	ProviderInput     textinput.Model
	APIKeyInput       textinput.Model
	ModelsInput       textinput.Model
	DefaultModelInput textinput.Model
	Focused           AISettingsField
	Width             int
	Height            int
	ScreenWidth       int
	ScreenHeight      int
}

func NewAISettingsDialog() *AISettingsDialog {
	createInput := func(placeholder string, isPassword bool) textinput.Model {
		ti := textinput.New()
		ti.Prompt = "❯ "
		ti.CharLimit = 256
		ti.Width = 50
		ti.Placeholder = placeholder
		if isPassword {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		ti.TextStyle = lipgloss.NewStyle().Foreground(ui.CurrentTheme.FgPrimary)
		ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.CurrentTheme.Accent)
		return ti
	}

	return &AISettingsDialog{
		Active:            false,
		ProviderInput:     createInput("https://api.openai.com/v1", false),
		APIKeyInput:       createInput("sk-...", true),
		ModelsInput:       createInput("gpt-4o-mini, gpt-4o, claude-3-5-sonnet", false),
		DefaultModelInput: createInput("gpt-4o-mini", false),
		Focused:           AIFieldProvider,
		Width:             62,
		Height:            19,
	}
}

func (d *AISettingsDialog) SetScreenSize(w, h int) {
	d.ScreenWidth = w
	d.ScreenHeight = h
}

func (d *AISettingsDialog) Open(cfg config.AIConfig) {
	d.Active = true
	provider := cfg.ProviderURL
	if provider == "" {
		provider = "https://api.openai.com/v1"
	}
	d.ProviderInput.SetValue(provider)
	d.APIKeyInput.SetValue(cfg.APIKey)

	modelsStr := strings.Join(cfg.Models, ", ")
	if strings.TrimSpace(modelsStr) == "" {
		modelsStr = "gpt-4o-mini, gpt-4o"
	}
	d.ModelsInput.SetValue(modelsStr)

	defModel := cfg.DefaultModel
	if defModel == "" {
		defModel = "gpt-4o-mini"
	}
	d.DefaultModelInput.SetValue(defModel)

	d.Focused = AIFieldProvider
	d.updateFocus()
}

func (d *AISettingsDialog) Close() {
	d.Active = false
	d.blurAll()
}

func (d *AISettingsDialog) blurAll() {
	d.ProviderInput.Blur()
	d.APIKeyInput.Blur()
	d.ModelsInput.Blur()
	d.DefaultModelInput.Blur()
}

func (d *AISettingsDialog) updateFocus() {
	d.blurAll()
	switch d.Focused {
	case AIFieldProvider:
		d.ProviderInput.Focus()
	case AIFieldAPIKey:
		d.APIKeyInput.Focus()
	case AIFieldModels:
		d.ModelsInput.Focus()
	case AIFieldDefaultModel:
		d.DefaultModelInput.Focus()
	}
}

func (d *AISettingsDialog) GetConfig() config.AIConfig {
	provider := strings.TrimSpace(d.ProviderInput.Value())
	if provider == "" {
		provider = "https://api.openai.com/v1"
	}
	apiKey := strings.TrimSpace(d.APIKeyInput.Value())

	var models []string
	rawModels := strings.Split(d.ModelsInput.Value(), ",")
	for _, m := range rawModels {
		trimmed := strings.TrimSpace(m)
		if trimmed != "" {
			models = append(models, trimmed)
		}
	}
	if len(models) == 0 {
		models = []string{"gpt-4o-mini", "gpt-4o"}
	}

	defModel := strings.TrimSpace(d.DefaultModelInput.Value())
	if defModel == "" {
		defModel = models[0]
	}

	return config.AIConfig{
		ProviderURL:  provider,
		APIKey:       apiKey,
		Models:       models,
		DefaultModel: defModel,
	}
}

func (d *AISettingsDialog) handleMouseClick(mouseX, mouseY int) (saved bool, canceled bool, newCfg config.AIConfig, cmd tea.Cmd) {
	sw := d.ScreenWidth
	sh := d.ScreenHeight
	if sw <= 0 {
		sw = 80
	}
	if sh <= 0 {
		sh = 24
	}

	atX := (sw - d.Width) / 2
	atY := (sh - d.Height) / 2
	if atX < 0 {
		atX = 0
	}
	if atY < 1 {
		atY = 1
	}

	relX := mouseX - atX
	relY := mouseY - atY

	// Click outside dialog box cancels dialog
	if relX < 0 || relX >= d.Width || relY < 0 || relY >= d.Height {
		d.Close()
		return false, true, config.AIConfig{}, nil
	}

	// Line mappings based on Render layout:
	// relY 4..5: Provider URL
	// relY 7..8: API Key
	// relY 10..11: Available Models
	// relY 13..14: Default Model
	// relY 16: Buttons
	switch {
	case relY == 4 || relY == 5:
		d.Focused = AIFieldProvider
		d.updateFocus()
	case relY == 7 || relY == 8:
		d.Focused = AIFieldAPIKey
		d.updateFocus()
	case relY == 10 || relY == 11:
		d.Focused = AIFieldModels
		d.updateFocus()
	case relY == 13 || relY == 14:
		d.Focused = AIFieldDefaultModel
		d.updateFocus()
	case relY == 16:
		// Buttons line: Save is roughly col 2..28, Cancel is roughly col 29..50
		if relX >= 2 && relX <= 28 {
			d.Close()
			return true, false, d.GetConfig(), nil
		} else if relX >= 29 && relX <= 52 {
			d.Close()
			return false, true, config.AIConfig{}, nil
		}
	}

	return false, false, config.AIConfig{}, nil
}

func (d *AISettingsDialog) Update(msg tea.Msg) (saved bool, canceled bool, newCfg config.AIConfig, cmd tea.Cmd) {
	if !d.Active {
		return false, false, config.AIConfig{}, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return d.handleMouseClick(msg.X, msg.Y)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			d.Close()
			return false, true, config.AIConfig{}, nil

		case "tab", "down":
			d.Focused = (d.Focused + 1) % 6
			d.updateFocus()
			return false, false, config.AIConfig{}, nil

		case "shift+tab", "up":
			d.Focused = (d.Focused + 5) % 6
			d.updateFocus()
			return false, false, config.AIConfig{}, nil

		case "enter":
			if d.Focused == AIFieldBtnCancel {
				d.Close()
				return false, true, config.AIConfig{}, nil
			}
			if d.Focused == AIFieldBtnSave {
				d.Close()
				return true, false, d.GetConfig(), nil
			}
			// If inside input, move to next field
			d.Focused = (d.Focused + 1) % 6
			d.updateFocus()
			return false, false, config.AIConfig{}, nil

		case "ctrl+s":
			d.Close()
			return true, false, d.GetConfig(), nil
		}
	}

	var curCmd tea.Cmd
	switch d.Focused {
	case AIFieldProvider:
		d.ProviderInput, curCmd = d.ProviderInput.Update(msg)
	case AIFieldAPIKey:
		d.APIKeyInput, curCmd = d.APIKeyInput.Update(msg)
	case AIFieldModels:
		d.ModelsInput, curCmd = d.ModelsInput.Update(msg)
	case AIFieldDefaultModel:
		d.DefaultModelInput, curCmd = d.DefaultModelInput.Update(msg)
	}

	return false, false, config.AIConfig{}, curCmd
}

func (d *AISettingsDialog) Render() string {
	if !d.Active {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgHighlight).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.CurrentTheme.FgSecondary)

	fieldWidth := d.Width - 6
	if fieldWidth < 20 {
		fieldWidth = 20
	}
	d.ProviderInput.Width = fieldWidth - 4
	d.APIKeyInput.Width = fieldWidth - 4
	d.ModelsInput.Width = fieldWidth - 4
	d.DefaultModelInput.Width = fieldWidth - 4

	inputBoxStyle := lipgloss.NewStyle().
		Width(fieldWidth).
		Background(ui.CurrentTheme.BgSidebar).
		Padding(0, 1)

	renderedTitle := titleStyle.Render("⚙ Konfigurasi Model AI (OpenAI API Format)")
	lblProvider := labelStyle.Render("Provider URL:")
	viewProvider := inputBoxStyle.Render(d.ProviderInput.View())

	lblKey := labelStyle.Render("API Key:")
	viewKey := inputBoxStyle.Render(d.APIKeyInput.View())

	lblModels := labelStyle.Render("Available Models (pisahkan koma):")
	viewModels := inputBoxStyle.Render(d.ModelsInput.View())

	lblDefault := labelStyle.Render("Default Model:")
	viewDefault := inputBoxStyle.Render(d.DefaultModelInput.View())

	btnSaveStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BorderColor)

	btnCancelStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ui.CurrentTheme.FgPrimary).
		Background(ui.CurrentTheme.BorderColor)

	if d.Focused == AIFieldBtnSave {
		btnSaveStyle = btnSaveStyle.Background(ui.CurrentTheme.Accent).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
	}
	if d.Focused == AIFieldBtnCancel {
		btnCancelStyle = btnCancelStyle.Background(ui.CurrentTheme.Red).Foreground(ui.CurrentTheme.FgHighlight).Bold(true)
	}

	btnSep := "  "
	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		btnSaveStyle.Render("Simpan (Enter / Ctrl+S)"),
		btnSep,
		btnCancelStyle.Render("Batal (Esc)"),
	)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		renderedTitle,
		lblProvider,
		viewProvider,
		"",
		lblKey,
		viewKey,
		"",
		lblModels,
		viewModels,
		"",
		lblDefault,
		viewDefault,
		"",
		buttons,
	)

	boxStyle := lipgloss.NewStyle().
		Background(ui.CurrentTheme.BgTopMenu).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(ui.CurrentTheme.BgTopMenu).
		BorderForeground(ui.CurrentTheme.Accent).
		Padding(1, 2).
		Width(d.Width)

	return NormalizeDialogBackground(boxStyle.Render(body), ui.CurrentTheme.BgTopMenu)
}
