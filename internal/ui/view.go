package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var titles = [4]string{"1 Podcasts", "2 Episodes", "3 Queue", "4 Details"}

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

func (m Model) detailRows(width int) []string {
	text := m.data.Details
	if text == "" {
		text = "Select an episode for details."
	}
	return strings.Split(ansi.Wrap(clean(text, true), max(1, m.descriptionWidth(width)-2), ""), "\n")
}
func (m *Model) scrollDetails(delta int) {
	width, height := m.detailsSize()
	m.detailScroll = max(0, min(m.detailScroll+delta, len(m.detailRows(width))-max(1, height-3)))
}

func (m Model) pane(panel Panel, width, height int) string {
	if panel == PlayerPanel {
		return m.playerPane(width, height)
	}
	var rows []string
	if panel == Details {
		rows = m.detailRows(width)
		start := min(m.detailScroll, max(0, len(rows)-max(1, height-3)))
		rows = rows[start:min(len(rows), start+max(0, height-3))]
		if columns, _ := m.ArtworkSize(); columns > 0 && m.artColumns > 0 {
			merged := make([]string, max(len(rows), len(m.artRows)))
			for i := range merged {
				art, text := "", ""
				if i < len(m.artRows) {
					art = m.artRows[i]
				}
				if i < len(rows) {
					text = rows[i]
				}
				merged[i] = art + strings.Repeat(" ", max(0, m.artColumns-ansi.StringWidth(art))) + " " + text
			}
			rows = merged
		}
	} else {
		items := m.items(panel)
		visible := max(0, height-3)
		start := max(0, m.selected[panel]-visible+1)
		for i := start; i < min(len(items), start+visible); i++ {
			prefix := "  "
			if i == m.selected[panel] {
				prefix = "> "
			}
			rows = append(rows, prefix+m.Icon(panelIcon(panel), clean(items[i].Title, false)))
		}
		if len(items) == 0 {
			rows = []string{"No items"}
		}
	}
	icon := panelIcon(panel)
	if panel == Podcasts {
		if strings.HasPrefix(m.data.PodcastLabel, "Discover") {
			icon = SearchIcon
		} else if strings.HasPrefix(m.data.PodcastLabel, "RSS/") {
			icon = FeedIcon
		}
	}
	title := m.Icon(icon, titles[panel])
	if panel == Podcasts && m.data.PodcastLabel != "" {
		title += " · " + clean(m.data.PodcastLabel, false)
	}
	if panel == Episodes && m.data.EpisodeLabel != "" {
		title += " · " + clean(m.data.EpisodeLabel, false)
	}
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
		Width(width).Height(height).MaxWidth(width).MaxHeight(height).Render(strings.Join(content, "\n"))
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
			top, middle, player := m.compactHeights()
			panel := Episodes
			if top == 0 {
				panel = m.lastVisible
			}
			if m.focus == Queue || m.focus == Details {
				panel = m.focus
			}
			parts := []string{}
			if top > 0 {
				parts = append(parts, m.pane(Podcasts, m.width, top))
			}
			parts = append(parts, m.pane(panel, m.width, middle), m.playerPane(m.width, player))
			content = lipgloss.JoinVertical(lipgloss.Left, parts...)
		} else {
			left, top := m.width*2/5, available/2
			right, bottom := m.width-left, available-top
			details := bottom / 2
			rightCell := lipgloss.JoinVertical(lipgloss.Left,
				m.pane(Details, right, details), m.playerPane(right, bottom-details))
			content = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.JoinHorizontal(lipgloss.Top, m.pane(Podcasts, left, top), m.pane(Queue, right, top)),
				lipgloss.JoinHorizontal(lipgloss.Top, m.pane(Episodes, left, bottom), rightCell))
		}
		hints := "hjkl navigate · Tab/1-5 panes · / filter · ? help · q quit"
		if m.compact() && m.focus >= Queue {
			hints = "Esc return · j/k select · / filter · ? help · q quit"
		}
		if m.data.EpisodeLabel != "" {
			hints = "N add · u subscribe · v New/Show · o sort · c complete · R refresh · ? help · q quit"
		}
		if m.data.ContextHint != "" {
			hints = clean(m.data.ContextHint, false)
		}
		if m.focus == Queue {
			hints = "j/k select · Enter play/promote · J/K move pending · d/D remove/clear pending · ? help · q quit"
		}
		if m.focus == PlayerPanel {
			hints = "h/l seek ±15s · Left/Right controls · Enter activate · Space play/pause · s stop · n next · Tab panes"
		}
		if m.focus == Details {
			hints = "j/k scroll · Ctrl-d/u half page · g/G top/bottom · PgUp/PgDn page · Tab panes · q quit"
		}
		if m.data.Notice != "" {
			hints = clean(m.data.Notice, false) + " · " + hints
		}
		if m.editing {
			hints = fmt.Sprintf("Filter: %s · Enter apply · Esc cancel", clean(m.filters[m.focus], false))
		}
		content += "\n" + ansi.Truncate(hints, m.width, "…")
		if m.help || m.data.Prompt != "" || m.data.Dialog != nil {
			width, height := min(72, m.width-2), min(16, m.height-2)
			if m.help {
				height = min(20, m.height-2)
			}
			var modal string
			if m.data.Dialog != nil {
				dialog := m.data.Dialog
				rows := strings.Split(ansi.Wrap(clean(dialog.Body, true), max(1, width-2), ""), "\n")
				rows = append(rows, clean(dialog.Hint, false))
				modal = box(clean(dialog.Title, false), rows, width, height, true)
			} else if m.data.Prompt != "" {
				height = min(7, m.height-2)
				modal = box("Add feed or podcast website", []string{clean(m.data.Prompt, false), "Enter add · Esc cancel · Ctrl+U clear"}, width, height, true)
			} else if m.help {
				modal = box("Keys · Esc closes", []string{
					"j/k arrows · g/G first/last item",
					"h/l, Tab · 1-5 focus pane",
					"Player (5): h/l seek ±15s · Left/Right select · Enter activate",
					m.Icon(SearchIcon, "/             Filter current list"), "Enter/Esc     Apply/cancel filter",
					"Ctrl+U        Clear filter input", "q / Ctrl+C    Quit",
					"Space selected play/toggle · s stop · n next",
					"Episodes Enter details · a enqueue · A enqueue visible",
					"Queue: Enter play/promote · j/k select",
					"Pending: J/K move · d remove · D clear (confirm)",
					"b My subscriptions / Apple Discover",
					"f search · Tab switches library/Apple scope",
					"N feed/website · u subscribe/unsubscribe",
					"v New/Show · o newest/oldest/title",
					"c mark completion · R refresh", "Details: j/k · Ctrl-d/u half page · g/G ends · PgUp/PgDn",
				}, width, height, true)
			}
			content = lipgloss.NewCompositor(lipgloss.NewLayer(content),
				lipgloss.NewLayer(modal).X((m.width-width)/2).Y((m.height-height)/2).Z(1)).Render()
		}
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
