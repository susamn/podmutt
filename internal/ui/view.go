package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var titles = [4]string{"1 Podcasts", "2 Episodes", "3 Queue", "4 Details / Player"}

// clean removes terminal escapes and control characters from untrusted metadata.
func clean(text string, multiline bool) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' && multiline {
			return r
		}
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(text))
}

func (m Model) pane(panel Panel, width, height int) string {
	var rows []string
	if panel == Details {
		text := m.data.Details
		if text == "" {
			text = "Select an episode for details."
		}
		player := m.data.Player
		if player == "" {
			player = "Player not connected"
		}
		rows = strings.Split(clean(text+"\n\n"+player, true), "\n")
	} else {
		items := m.items(panel)
		visible := max(0, height-3)
		start := max(0, m.selected[panel]-visible+1)
		for i := start; i < min(len(items), start+visible); i++ {
			prefix := "  "
			if i == m.selected[panel] {
				prefix = "> "
			}
			rows = append(rows, prefix+clean(items[i].Title, false))
		}
		if len(items) == 0 {
			rows = []string{"No items"}
		}
	}
	title := titles[panel]
	if panel < Details && m.filters[panel] != "" {
		title += " / " + clean(m.filters[panel], false)
	}
	return box(title, rows, width, height, panel == m.focus)
}

func box(title string, rows []string, width, height int, focused bool) string {
	innerWidth, innerHeight := max(0, width-2), max(0, height-2)
	content := []string{lipgloss.NewStyle().Bold(true).Render(ansi.Truncate(title, innerWidth, "…"))}
	for _, row := range rows {
		if len(content) >= innerHeight {
			break
		}
		content = append(content, ansi.Truncate(row, innerWidth, "…"))
	}
	color := "8"
	if focused {
		color = "6"
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(color)).
		Width(innerWidth).Height(innerHeight).MaxWidth(width).MaxHeight(height).Render(strings.Join(content, "\n"))
}

func (m Model) View() tea.View {
	var content string
	switch {
	case m.width == 0 || m.height == 0:
		content = ""
	case m.width < 30 || m.height < 12:
		content = ansi.Truncate("Resize terminal (minimum 30×12); q quits", m.width, "…")
	default:
		available := m.height - 1
		if m.compact() {
			top := max(3, min(6, (available-4)/3))
			middle := available - top - 4
			player := clean(m.data.Player, false)
			if player == "" {
				player = "Player not connected"
			}
			content = lipgloss.JoinVertical(lipgloss.Left,
				m.pane(Podcasts, m.width, top), m.pane(Episodes, m.width, middle),
				box("Player", []string{player}, m.width, 4, false))
		} else {
			left, top := m.width*2/5, available/2
			content = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.JoinHorizontal(lipgloss.Top, m.pane(Podcasts, left, top), m.pane(Queue, m.width-left, top)),
				lipgloss.JoinHorizontal(lipgloss.Top, m.pane(Episodes, left, available-top), m.pane(Details, m.width-left, available-top)))
		}
		hints := "hjkl navigate · Tab/1-4 panes · / filter · ? help · q quit"
		if m.compact() && m.focus >= Queue {
			hints = "Esc return · j/k select · / filter · ? help · q quit"
		}
		if m.editing {
			hints = fmt.Sprintf("Filter: %s · Enter apply · Esc cancel", clean(m.filters[m.focus], false))
		}
		content += "\n" + ansi.Truncate(hints, m.width, "…")
		if m.help || (m.compact() && m.focus >= Queue) {
			width, height := min(72, m.width-2), min(16, m.height-2)
			var modal string
			if m.help {
				modal = box("Keys · Esc closes", []string{
					"j/k, arrows   Move selection", "g/G           First/last item",
					"h/l, Tab      Change pane", "1-4           Focus pane",
					"/             Filter current list", "Enter/Esc     Apply/cancel filter",
					"Ctrl+U        Clear filter input", "q / Ctrl+C    Quit",
					"Space toggle · s stop · n next (current episode)",
				}, width, height, true)
			} else {
				modal = m.pane(m.focus, width, height)
			}
			content = lipgloss.NewCompositor(lipgloss.NewLayer(content),
				lipgloss.NewLayer(modal).X((m.width-width)/2).Y((m.height-height)/2).Z(1)).Render()
		}
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
