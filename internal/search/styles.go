package search

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// applyTextInputStyles applies the given Text/Prompt styles to both the
// focused and blurred states of a text input (the v1 TextStyle/PromptStyle
// fields were unified this way in bubbles v2).
func applyTextInputStyles(ti *textinput.Model, textStyle, promptStyle lipgloss.Style) {
	st := ti.Styles()
	st.Focused.Text = textStyle
	st.Blurred.Text = textStyle
	st.Focused.Prompt = promptStyle
	st.Blurred.Prompt = promptStyle
	ti.SetStyles(st)
}
