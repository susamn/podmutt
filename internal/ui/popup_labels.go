package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"podmutt/internal/theme"
)

// Structural popup text is trusted UI copy. Never pass feed titles, URLs,
// search input, or remote error text through these styling helpers.
func popupSection(label string, pal theme.Palette) string {
	style := lipgloss.NewStyle().Bold(true).Underline(true).
		Foreground(lipgloss.Color(string(pal.Accent)))
	return stylePopupWords(label, style)
}

func popupLabel(label string, pal theme.Palette) string {
	style := lipgloss.NewStyle().Bold(true).Underline(true).
		Foreground(lipgloss.Color(string(pal.Cyan)))
	return stylePopupWords(label, style)
}

// Keep multiword labels underlined after the compositor splits styled spans
// at spaces (e.g. "NOW PLAYING" and "Active file:").
func stylePopupWords(text string, style lipgloss.Style) string {
	words := strings.Split(text, " ")
	for i, word := range words {
		if word != "" {
			words[i] = style.Render(word)
		}
	}
	return strings.Join(words, " ")
}

// Each hint segment starts with a key. Keep its explanation muted and style
// just that key; alignments and original text remain unchanged.
func popupHelpRow(row string, pal theme.Palette) string {
	if at := strings.Index(row, "  "); at > 0 {
		return popupLabel(row[:at], pal) + row[at:]
	}
	return row
}

func popupHint(hint string, pal theme.Palette) string {
	parts := strings.Split(clean(hint, false), " · ")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Muted)))
	for i, part := range parts {
		key, rest, found := strings.Cut(part, " ")
		if found {
			parts[i] = popupLabel(key, pal) + muted.Render(" "+rest)
		} else {
			parts[i] = popupLabel(key, pal)
		}
	}
	return strings.Join(parts, muted.Render(" · "))
}
