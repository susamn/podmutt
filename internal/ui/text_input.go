package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// TextInput provides unicode-safe single-line text editing.
type TextInput struct {
	Value string
	Limit int
}

func (t *TextInput) limit() int {
	if t.Limit <= 0 {
		return 4096
	}
	return t.Limit
}

// Append sanitizes text (mapping control chars to spaces) and appends up to Limit runes.
func (t *TextInput) Append(text string) {
	text = strings.Map(func(r rune) rune {
		if r < ' ' || r == 127 {
			return ' '
		}
		return r
	}, text)
	runes := []rune(t.Value + text)
	t.Value = string(runes[:min(len(runes), t.limit())])
}

// Backspace removes the trailing unicode rune.
func (t *TextInput) Backspace() {
	runes := []rune(t.Value)
	if len(runes) > 0 {
		t.Value = string(runes[:len(runes)-1])
	}
}

// Clear empties the text buffer.
func (t *TextInput) Clear() {
	t.Value = ""
}

// EditKey handles backspace, ctrl+u, and printable text keys.
// Returns true if the key was consumed as an editing action.
func (t *TextInput) EditKey(key tea.KeyPressMsg) bool {
	switch key.String() {
	case "backspace":
		t.Backspace()
		return true
	case "ctrl+u":
		t.Clear()
		return true
	default:
		if key.Text != "" {
			t.Append(key.Text)
			return true
		}
		return false
	}
}
