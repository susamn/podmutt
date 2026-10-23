package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderSettingsModal(width, height int) string {
	compact := height < 12
	var rows []string
	if !compact {
		rows = append(rows, m.modalLabel("Appearance", width))
	}
	for i, name := range m.settingsData.Themes {
		mark := " "
		if name == m.settingsData.ActiveTheme {
			mark = "●"
		}
		row := fmt.Sprintf("[%s] %s", mark, name)
		if i == m.settingsData.Selected {
			rows = append(rows, "  "+m.highlightRow(row, modalTextWidth(width)))
		} else {
			rows = append(rows, m.modalLine(row, width))
		}
	}
	if !compact {
		rows = append(rows, "", m.modalLabel("Configuration", width))
	}
	fields := []string{}
	if m.settingsData.ThemePath != "" {
		fields = append(fields, "Active file  "+m.settingsData.ThemePath)
	}
	if m.settingsData.ConfigPath != "" {
		fields = append(fields, "Config       "+m.settingsData.ConfigPath)
	}
	if m.settingsData.RefreshMin > 0 {
		fields = append(fields, fmt.Sprintf("Refresh      %d min", m.settingsData.RefreshMin))
	}
	if m.settingsData.Icons != "" {
		fields = append(fields, "Icons        "+m.settingsData.Icons)
	}
	if m.settingsData.Country != "" {
		fields = append(fields, "Country:     "+strings.ToUpper(m.settingsData.Country)+" · Left/Right to change")
	}
	if m.settingsData.MPVPath != "" {
		fields = append(fields, "MPV          "+m.settingsData.MPVPath)
	}
	footerRows := 1
	if !compact {
		footerRows = 2
	}
	available := max(0, modalBodyCapacity(height)-len(rows)-footerRows)
	for _, field := range fields[:min(len(fields), available)] {
		rows = append(rows, m.modalLine(field, width))
	}
	if !compact {
		rows = append(rows, "")
	}
	hint := "j/k select · Enter apply · Left/Right country · Esc close"
	if compact {
		hint = "Enter apply · Esc close"
	}
	rows = append(rows, m.modalFooter(hint, width))
	return m.box("Settings", rows, width, height, true, "", "")
}

func (m Model) helpPairLabel(left, right string, width int) string {
	half := modalTextWidth(width) / 2
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(m.Theme().Muted)))
	return m.modalLine(padRight(style.Render(strings.ToUpper(left)), half)+style.Render(strings.ToUpper(right)), width)
}

func (m Model) helpPair(leftKey, leftAction, rightKey, rightAction string, width int) string {
	half := modalTextWidth(width) / 2
	left := fmt.Sprintf("%-7s %s", leftKey, leftAction)
	right := fmt.Sprintf("%-7s %s", rightKey, rightAction)
	return m.modalLine(padRight(left, half)+right, width)
}

func (m Model) renderHelpModal(width, height int) string {
	compact := height < 21 || width < 56
	if compact {
		rows := []string{
			m.modalLine("Space   Pause / resume", width),
			m.modalLine("s       Stop", width),
			m.modalLine("n       Next episode", width),
			m.modalLine("C       Chapters", width),
			m.modalLine("j/k     Move selection", width),
			m.modalFooter("? or Esc close", width),
		}
		rows = rows[:min(len(rows), modalBodyCapacity(height))]
		return m.box("Keys", rows, width, height, true, "", "")
	}

	var rows []string
	rows = append(rows, m.helpPairLabel("Playback", "Episodes", width))
	for _, pair := range [][4]string{
		{"Space", "Pause / resume", "Enter", "Play"},
		{"s", "Stop", "i", "Details"},
		{"n", "Next episode", "a", "Enqueue"},
		{", / .", "Seek ±15s", "A", "Enqueue visible"},
		{"C", "Chapters", "c", "Complete"},
	} {
		rows = append(rows, m.helpPair(pair[0], pair[1], pair[2], pair[3], width))
	}

	rows = append(rows, "", m.helpPairLabel("Navigation", "Queue", width))
	for _, pair := range [][4]string{
		{"j / k", "Move", "m", "Actions"},
		{"g / G", "First / last", "d", "Remove"},
		{"Tab", "Next list", "u", "Undo"},
		{"1–6", "Focus panel", "D", "Clear pending"},
		{"/", "Filter list", "Enter", "Apply action"},
	} {
		rows = append(rows, m.helpPair(pair[0], pair[1], pair[2], pair[3], width))
	}

	rows = append(rows, "", m.helpPairLabel("General", "Favorites", width))
	for _, pair := range [][4]string{
		{"?", "Help", "Enter", "Play"},
		{"e", "Settings", "d", "Remove favorite"},
		{"!", "Message details", "Esc", "Close / back"},
	} {
		rows = append(rows, m.helpPair(pair[0], pair[1], pair[2], pair[3], width))
	}
	rows = append(rows, m.modalFooter("? or Esc close", width))
	rows = rows[:min(len(rows), modalBodyCapacity(height))]
	return m.box("Keys", rows, width, height, true, "", "")
}
