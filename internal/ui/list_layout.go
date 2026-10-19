package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	listGap             = -1
	subscriptionHeading = -2
)

// Layout rows reference selectable items; negative entries are decoration only.
// Rendering and paging share this mapping so a heading never becomes a stop or
// pushes the selected item outside its viewport.
func (m Model) listLayout(panel Panel, items []Item, capacity int) []int {
	grouped := panel == Podcasts && slices.ContainsFunc(m.data.Podcasts, func(item Item) bool { return item.Destination })
	rows := make([]int, 0, len(items)+2)
	for i, item := range items {
		if grouped && !item.Destination {
			if i > 0 && capacity >= 6 {
				rows = append(rows, listGap)
			}
			rows = append(rows, subscriptionHeading)
			grouped = false
		}
		rows = append(rows, i)
	}
	return rows
}

func (m Model) renderList(panel Panel, items []Item, width, capacity int) []string {
	layout := m.listLayout(panel, items, capacity)
	selected := slices.Index(layout, m.selected[panel])
	start := max(0, selected-capacity+1)
	var rows []string
	for _, index := range layout[start:min(len(layout), start+capacity)] {
		switch index {
		case listGap:
			rows = append(rows, "")
		case subscriptionHeading:
			label := "  Subscriptions "
			line := label + strings.Repeat("─", max(0, width-2-ansi.StringWidth(label)))
			rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color(string(m.Theme().Muted))).Render(line))
		default:
			rows = append(rows, m.renderRow(panel, items[index], width, index == m.selected[panel]))
		}
	}
	return rows
}

func (m *Model) pageList(delta int) {
	items := m.items(m.focus)
	if len(items) == 0 {
		return
	}
	layout := m.listLayout(m.focus, items, m.listPageSize())
	position := slices.Index(layout, m.selected[m.focus])
	target := max(0, min(len(layout)-1, position+delta))
	step := 1
	if delta < 0 {
		step = -1
	}
	for layout[target] < 0 {
		if target == 0 {
			step = 1 // A filtered list may begin with its heading.
		}
		target += step
	}
	m.selected[m.focus] = layout[target]
}
