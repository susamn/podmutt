package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Modal content uses one shared visual rhythm: two-cell left padding, compact
// uppercase section labels, separated content groups, and a muted footer.
func modalTextWidth(width int) int { return max(1, width-5) }

func (m Model) modalLine(text string, width int) string {
	return "  " + ansi.Truncate(clean(text, false), modalTextWidth(width), "…")
}

func (m Model) modalWrapped(text string, width int) []string {
	var rows []string
	for _, paragraph := range strings.Split(clean(text, true), "\n") {
		if strings.TrimSpace(paragraph) == "" {
			rows = append(rows, "")
			continue
		}
		for _, row := range strings.Split(ansi.Wrap(paragraph, modalTextWidth(width), ""), "\n") {
			rows = append(rows, m.modalLine(row, width))
		}
	}
	return rows
}

func (m Model) modalLabel(label string, width int) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(m.Theme().Muted)))
	return m.modalLine(style.Render(strings.ToUpper(clean(label, false))), width)
}

func (m Model) modalFooter(hint string, width int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(string(m.Theme().Muted)))
	return m.modalLine(style.Render(clean(hint, false)), width)
}

func modalBodyCapacity(height int) int { return max(0, height-2) }
