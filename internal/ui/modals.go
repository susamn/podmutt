package ui

import (
	"fmt"
	"strings"
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

func (m Model) renderHelpModal(width, height int) string {
	compact := height < 12
	sections := []struct {
		label string
		lines []string
	}{
		{"Navigation", []string{"j/k or arrows move · g/G first/last", "h/l or Left/Right open/pan · Tab lists · 1–6 focus", "/ filter · Enter/Esc apply/cancel"}},
		{"Playback", []string{"Space pause/resume · s stop · n next", ",/. seek ±15s · C current chapters", "Player (6): arrows select · Enter activate"}},
		{"Library", []string{"Episodes: Enter play · i details · a/A enqueue", "Queue: m actions · d remove · u undo · D clear", "Favorites: d remove · e settings · ! message"}},
	}
	var rows []string
	for _, section := range sections {
		if len(rows) > 0 && !compact {
			rows = append(rows, "")
		}
		if !compact {
			rows = append(rows, m.modalLabel(section.label, width))
		}
		for _, line := range section.lines {
			if len(rows) >= modalBodyCapacity(height)-1 {
				break
			}
			rows = append(rows, m.modalLine(line, width))
		}
	}
	if len(rows) < modalBodyCapacity(height)-1 && !compact {
		rows = append(rows, "")
	}
	if len(rows) < modalBodyCapacity(height) {
		rows = append(rows, m.modalFooter("? or Esc close", width))
	}
	return m.box("Keys", rows, width, height, true, "", "")
}
