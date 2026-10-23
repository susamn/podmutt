package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

var titles = [5]string{"Library", "Episodes", "Queue", "Details", "Favorites"}

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

func listCapacity(panel Panel, height int) int {
	rows := height - 3
	if panel == Episodes || panel == Queue || panel == Favorites {
		rows-- // pinned table header
	}
	return max(0, rows)
}

func (m Model) listPageSize() int {
	available := max(0, m.height-statusRows)
	height := available
	g := m.wideLayout()
	if m.focus == Episodes {
		height = g.episodes
	} else if m.focus == Queue {
		height = g.queue
	} else if m.focus == Favorites {
		height = g.favorites
	}
	if m.compact() {
		top, middle, _ := m.compactHeights()
		height = middle
		if m.focus == Podcasts && top > 0 {
			height = top
		}
	}
	return max(1, listCapacity(m.focus, height))
}

func (m Model) pane(panel Panel, width, height int) string {
	if panel == PlayerPanel {
		return m.playerPane(width, height)
	}
	var rows []string
	extraTag := ""
	if panel == Details {
		rows, extraTag = m.renderDetails(width, height)
		if columns, _ := m.ArtworkSize(); columns > 0 && m.artColumns > 0 {
			merged := make([]string, max(len(rows), len(m.artRows)))
			for i := range merged {
				art, text := "", ""
				if i < len(m.artRows) {
					art = m.artRows[i]
				}
				if i < len(rows) {
					text = strings.TrimPrefix(rows[i], "  ")
				}
				merged[i] = "  " + art + strings.Repeat(" ", max(0, m.artColumns-ansi.StringWidth(art))) + " " + text
			}
			rows = merged
		}
	} else {
		items := m.items(panel)
		if panel == Episodes {
			rows = append(rows, m.renderEpisodesHeader(width))
		} else if panel == Queue {
			rows = append(rows, m.renderQueueHeader(width))
		} else if panel == Favorites {
			rows = append(rows, m.renderFavoritesHeader(width))
		}
		rows = append(rows, m.renderList(panel, items, width, listCapacity(panel, height))...)
		if len(items) == 0 {
			empty := "No episodes for this selection"
			if panel == Podcasts {
				empty = "No shows · f find a podcast"
			}
			if m.HasFilter(panel) {
				if panel == Podcasts {
					empty = "No matching shows"
				} else if panel == Queue {
					empty = "No matching queue episodes"
				} else if panel == Favorites {
					empty = "No matching favorites"
				} else {
					empty = "No matching episodes"
				}
			} else if panel == Podcasts && m.data.PodcastEmpty != "" {
				empty = clean(m.data.PodcastEmpty, false)
			} else if panel == Episodes && m.data.EpisodeEmpty != "" {
				empty = clean(m.data.EpisodeEmpty, false)
			} else if panel == Queue {
				empty = "Queue empty · 2: Episodes"
			} else if panel == Favorites {
				empty = "No favorites · m in Queue to add"
			}
			rows = append(rows, "  "+strings.TrimLeft(empty, " "))
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
	if panel == Podcasts && (strings.HasPrefix(m.data.PodcastLabel, "Discover") || strings.HasPrefix(m.data.PodcastLabel, "RSS/")) {
		title = clean(m.data.PodcastLabel, false)
	}
	if panel == Details && m.data.EpisodeDetails != nil {
		context := "Selected"
		if m.data.EpisodeDetails.Inspecting {
			context = "Inspecting"
		}
		title += " · " + context
	}
	if panel == Episodes && m.data.EpisodeLabel != "" {
		title += " · " + clean(m.data.EpisodeLabel, false)
	}
	if panel == Queue {
		if m.HasFilter(Queue) {
			title += fmt.Sprintf(" · %d/%d", len(m.items(Queue)), len(m.data.Queue))
		} else {
			title += fmt.Sprintf(" · %d", len(m.data.Queue))
		}
	}
	if panel == Queue {
		title += " · " + m.queueOrderLabel()
	}
	if panel < Details && m.filters[panel] != "" {
		title += " / " + clean(m.filters[panel], false)
	}
	tag := fmt.Sprintf("[%d]", panel+1)
	if panel == Queue {
		extraTag = m.Version()
	}
	return m.box(title, rows, width, height, panel == m.focus, tag, extraTag)
}

func (m Model) renderEpisodesHeader(width int) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(m.Theme().LightForeground))).Render(episodeColumns(width).row("", "Title", "Ch", "Channel", "Date", "Duration"))
}

func padRight(s string, width int) string {
	s = ansi.Truncate(s, width, "…")
	pad := max(0, width-ansi.StringWidth(s))
	return s + strings.Repeat(" ", pad)
}

func padLeft(s string, width int) string {
	s = ansi.Truncate(s, width, "…")
	pad := max(0, width-ansi.StringWidth(s))
	return strings.Repeat(" ", pad) + s
}

func (m Model) renderRow(panel Panel, item Item, width int, selected bool) string {
	prefix := "  "
	switch panel {
	case Podcasts:
		if item.Destination {
			return prefix + clean(item.Title, false)
		}
		channel := item.Channel
		if channel == "" {
			channel = item.Title
		}
		chanColor := lipgloss.Color(string(m.Theme().ChannelColor(channel)))
		title := clean(item.Title, false)
		if offset := m.colOffset[Podcasts]; offset > 0 {
			runes := []rune(title)
			if offset < len(runes) {
				title = string(runes[offset:])
			} else {
				title = ""
			}
		}
		return prefix + m.Icon(ShowIcon, lipgloss.NewStyle().Foreground(chanColor).Render(title))
	case Episodes:
		return m.renderEpisodeRow(item, width, selected)
	case Queue:
		return m.renderQueueRow(item, width, selected)
	case Favorites:
		return m.renderFavoritesRow(item, width, selected)
	default:
		return prefix + m.Icon(panelIcon(panel), clean(item.Title, false))
	}
}

func (m Model) renderEpisodeRow(item Item, width int, selected bool) string {
	return m.renderEpisodeColumns(item, width)
}

func (m Model) renderQueueRow(item Item, width int, selected bool) string {
	remaining := FormatRemainingPercent(item.PositionSeconds, item.DurationSeconds)
	if item.Current {
		remaining = FormatRemainingPercent(m.data.Player.PositionSeconds, m.data.Player.DurationSeconds)
		if remaining != "--" {
			player := m.displayPlayer()
			remaining = FormatRemainingPercent(player.Progress*player.DurationSeconds, player.DurationSeconds)
		}
	}
	return m.renderQueueColumns(item, width, remaining)
}

func (m Model) box(title string, rows []string, width, height int, focused bool, tag, extraTag string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	pal := m.Theme()
	borderColor := string(pal.Muted)
	if focused {
		borderColor = string(pal.Accent)
	}
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))

	topLine := m.boxTop(title, tag, extraTag, width, focused)
	if height == 1 {
		return topLine
	}

	innerWidth := max(0, width-2)
	innerHeight := max(0, height-2)

	var lines []string
	lines = append(lines, topLine)

	leftBorder := borderStyle.Render("│")
	rightBorder := borderStyle.Render("│")

	for i := 0; i < innerHeight; i++ {
		if width == 1 {
			lines = append(lines, leftBorder)
			continue
		}
		rowText := ""
		if i < len(rows) {
			rowText = ansi.Truncate(rows[i], innerWidth, "…")
		}
		pad := max(0, innerWidth-ansi.StringWidth(rowText))
		lines = append(lines, leftBorder+rowText+strings.Repeat(" ", pad)+rightBorder)
	}

	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
	if width == 1 {
		bottomLine = borderStyle.Render("╰")
	}
	lines = append(lines, bottomLine)

	return strings.Join(lines, "\n")
}

func (m Model) boxTop(title, tag, extraTag string, width int, focused bool) string {
	if width <= 0 {
		return ""
	}
	pal := m.Theme()
	borderColor := string(pal.Muted)
	if focused {
		borderColor = string(pal.Accent)
	}
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))

	if width < 8 {
		if width == 1 {
			return borderStyle.Render("╭")
		}
		if width == 2 {
			return borderStyle.Render("╭╮")
		}
		return borderStyle.Render("╭" + strings.Repeat("─", width-2) + "╮")
	}

	leftCorner := "╭"
	rightCorner := "╮"
	prefix := "──"

	rightFurniture := ""
	if tag != "" {
		rightFurniture = tag + "─"
	}
	if extraTag != "" && width >= 30 {
		if rightFurniture != "" {
			rightFurniture = tag + "──" + extraTag + "─"
		} else {
			rightFurniture = extraTag + "─"
		}
	}

	innerWidth := width - 2
	fixedRightLen := ansi.StringWidth(rightFurniture)
	availForTitle := max(0, innerWidth-fixedRightLen-ansi.StringWidth(prefix))

	truncatedTitle := ansi.Truncate(title, availForTitle, "…")
	leftSideLen := ansi.StringWidth(prefix) + ansi.StringWidth(truncatedTitle)
	fillLen := max(0, innerWidth-leftSideLen-fixedRightLen)

	titleStyle := lipgloss.NewStyle().Bold(true)
	if focused {
		titleStyle = titleStyle.Foreground(lipgloss.Color(string(pal.Accent)))
	}

	tagStyle := lipgloss.NewStyle()
	if focused {
		tagStyle = tagStyle.Foreground(lipgloss.Color(string(pal.Accent)))
	} else {
		tagStyle = tagStyle.Foreground(lipgloss.Color(string(pal.Muted)))
	}

	var sb strings.Builder
	sb.WriteString(borderStyle.Render(leftCorner))
	sb.WriteString(borderStyle.Render(prefix))
	sb.WriteString(titleStyle.Render(truncatedTitle))
	if fillLen > 0 {
		sb.WriteString(borderStyle.Render(strings.Repeat("─", fillLen)))
	}
	if rightFurniture != "" {
		if tag != "" {
			sb.WriteString(tagStyle.Render(tag))
			if extraTag != "" && width >= 30 {
				sb.WriteString(borderStyle.Render("──"))
				sb.WriteString(borderStyle.Render(extraTag))
			}
			sb.WriteString(borderStyle.Render("─"))
		} else {
			sb.WriteString(borderStyle.Render(rightFurniture))
		}
	}
	sb.WriteString(borderStyle.Render(rightCorner))
	return sb.String()
}

type hint struct {
	key, action string
}

func (m Model) formatHints(hints []hint) string {
	pal := m.Theme()
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(pal.Cyan)))
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = keyStyle.Render(h.key) + ":" + h.action
	}
	return strings.Join(parts, "  ")
}

func (m Model) bottomHints() string {
	if m.editing {
		return m.formatHints([]hint{
			{"Enter", "apply"},
			{"Esc", "cancel"},
			{"Ctrl+U", "clear"},
		})
	}

	isDiscover := strings.HasPrefix(m.data.PodcastLabel, "Discover") || strings.HasPrefix(m.data.PodcastLabel, "RSS/")

	var hints []hint
	switch m.focus {
	case Details:
		hints = append(m.episodeHints(), []hint{
			{"Esc", "back"},
			{"j/k", "scroll"},
			{"Ctrl-d/u", "half-page"},
			{"g/G", "top/bottom"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}...)
	case PlayerPanel:
		hints = []hint{
			{"Left/Right", "select"},
			{"Enter", "activate"},
			{",/.", "seek"},
			{"Space", "play/pause"},
			{"s", "stop"},
			{"n", "next"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}
	case Favorites:
		hints = []hint{
			{"Enter", "play"},
			{"d/Del", "remove"},
			{"j/k", "select"},
			{"i", "details"},
			{"/", "filter"},
			{"Space", "pause/resume"},
			{",/.", "seek"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}
	case Queue:
		hints = []hint{
			{"d/Del", "remove"},
			{"u", "undo"},
			{"m", "actions"},
			{"j/k", "select"},
			{"Enter", "play"},
			{"Space", "pause/resume"},
			{"i", "details"},
			{"o", "sort view"},
			{"/", "filter"},
			{"D", "clear"},
			{",/.", "seek"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}
	default:
		if m.focus == Podcasts && !isDiscover {
			hints = []hint{{"j/k", "show"}, {"/", "filter"}, {"o", "sort episodes"}, {"Tab/2", "episodes"}, {"f", "find podcast"}, {"Space", "pause/resume"}, {"?", "help"}, {"q", "quit"}}
		} else if isDiscover {
			hints = []hint{
				{"/", "filter list"},
				{"f", "find podcast"},
				{"Tab", "episodes"},
				{"b", "library"},
				{"t", "top100"},
				{"Enter", "open/play"},
				{"Space", "pause/resume"},
				{"i", "details"},
				{"a", "enqueue"},
				{"u", "subscribe"},
				{"R", "retry"},
				{",/.", "seek"},
				{"e", "settings"},
				{"?", "help"},
				{"q", "quit"},
			}
		} else {
			hints = []hint{
				{"/", "filter"},
				{"Tab", "pane"},
				{"f", "find podcast"},
				{"b", "discover"},
				{"N", "add"},
				{"u", "unsub"},
				{"o", "sort"},
				{"c", "complete"},
				{"Enter", "open/play"},
				{"Space", "pause/resume"},
				{"i", "details"},
				{"s", "stop"},
				{"n", "next"},
				{",/.", "seek"},
				{"e", "settings"},
				{"?", "help"},
				{"q", "quit"},
			}
		}
	}

	if m.focus == Episodes && m.episodeOps.Available {
		// Prioritize one shared action set in the list and its reading surface.
		actions := m.episodeHints()
		var remaining []hint
		for _, existing := range hints {
			matched := false
			for _, action := range actions {
				if existing.key == action.key {
					matched = true
					break
				}
			}
			if !matched {
				remaining = append(remaining, existing)
			}
		}
		hints = append(actions, remaining...)
	}
	return m.formatHints(hints)
}

func renderInputBox(label, value string, width int, pal theme.Palette) string {
	if label == "" {
		label = "Input"
	}
	fieldWidth := max(1, width-6-ansi.StringWidth(label)-4)
	valTrunc := ansi.Truncate(value, fieldWidth-1, "…")
	inputArea := valTrunc + "▌"
	pad := max(0, fieldWidth-ansi.StringWidth(inputArea))

	inputBox := lipgloss.NewStyle().
		Background(lipgloss.Color(string(pal.Selection))).
		Foreground(lipgloss.Color(string(pal.BrightForeground))).
		Render(" " + inputArea + strings.Repeat(" ", pad) + " ")

	return "  " + label + ": " + inputBox
}

func (m Model) renderDialogModal(dialog *Dialog, width, height int) string {
	pal := m.Theme()
	var rows []string

	bodyLines := strings.Split(clean(dialog.Body, true), "\n")
	for _, l := range bodyLines {
		if strings.TrimSpace(l) == "" {
			rows = append(rows, "")
			continue
		}
		if dialog.TruncateBody {
			rows = append(rows, ansi.Truncate(l, max(1, width-2), "…"))
		} else {
			rows = append(rows, strings.Split(ansi.Wrap(l, max(1, width-4), ""), "\n")...)
		}
	}

	if dialog.InputLabel != "" || dialog.InputValue != "" {
		reserved := 2
		if dialog.Hint != "" {
			reserved += 2
		}
		rows = rows[:min(len(rows), max(0, height-2-reserved))]
		rows = append(rows, "", renderInputBox(dialog.InputLabel, dialog.InputValue, width, pal))
	}

	if dialog.Hint != "" {
		// Keep decision keys visible in compact row-based confirmations.
		if dialog.TruncateBody {
			rows = rows[:min(len(rows), max(0, height-4))]
		}
		rows = append(rows, "")
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Muted)))
		rows = append(rows, "  "+hintStyle.Render(clean(dialog.Hint, false)))
	}

	modalHeight := min(height, len(rows)+2)
	return m.box(clean(dialog.Title, false), rows, width, modalHeight, true, "", "")
}

func (m Model) renderPromptModal(prompt string, width, height int) string {
	val := prompt
	if strings.HasPrefix(val, "URL: ") {
		val = strings.TrimPrefix(val, "URL: ")
	}
	return m.renderDialogModal(&Dialog{
		Title:      "Add feed or podcast website",
		Body:       "Enter a podcast RSS/Atom feed or website URL:",
		InputLabel: "URL",
		InputValue: val,
		Hint:       "Enter add · Esc cancel · Ctrl+U clear",
	}, width, height)
}

func (m Model) View() tea.View {
	var content string
	switch {
	case m.width == 0 || m.height == 0:
		content = ""
	case m.width < 30 || m.height < 12:
		content = ansi.Truncate("Resize terminal (minimum 30×12); q quits", m.width, "…")
	default:
		available := m.height - statusRows
		if m.compact() {
			top, middle, player := m.compactHeights()
			panel := Episodes
			if top == 0 {
				panel = m.lastVisible
			}
			if m.focus == Queue || m.focus == Details || m.focus == Favorites {
				panel = m.focus
			}
			parts := []string{}
			if top > 0 {
				parts = append(parts, m.pane(Podcasts, m.width, top))
			}
			parts = append(parts, m.playerPane(m.width, player), m.pane(panel, m.width, middle))
			content = lipgloss.JoinVertical(lipgloss.Left, parts...)
		} else {
			g := m.wideLayout()
			middle := lipgloss.JoinVertical(lipgloss.Left, m.pane(Episodes, g.episodeWidth, g.episodes), m.pane(Details, g.episodeWidth, g.details))
			right := lipgloss.JoinVertical(lipgloss.Left, m.pane(Queue, g.queueWidth, g.queue), m.pane(Favorites, g.queueWidth, g.favorites))
			upper := lipgloss.JoinHorizontal(lipgloss.Top, middle, right)
			body := lipgloss.JoinVertical(lipgloss.Left, upper, m.playerPane(g.playerWidth, g.player))
			content = lipgloss.JoinHorizontal(lipgloss.Top, m.pane(Podcasts, g.libraryWidth, available), body)
		}
		hints := m.bottomHints()
		content += "\n" + m.statusLine() + "\n" + ansi.Truncate(hints, m.width, "…")
		if m.messageOpen || m.help || m.settings || m.data.Prompt != "" || m.data.Dialog != nil || m.chapters != nil {
			width, height := min(72, m.width-2), min(16, available-2)
			if m.messageOpen || m.help || m.settings || m.data.Dialog != nil || m.chapters != nil {
				height = min(22, available-2)
			}
			var modal string
			if m.chapters != nil {
				modal = m.renderChapters(width, height)
			} else if m.messageOpen {
				modal = m.feedbackModal(width, height)
			} else if m.data.Dialog != nil {
				modal = m.renderDialogModal(m.data.Dialog, width, height)
			} else if m.data.Prompt != "" {
				modal = m.renderPromptModal(clean(m.data.Prompt, false), width, height)
			} else if m.settings {
				height = min(20, available-2)
				width = min(64, m.width-4)
				var rows []string
				rows = append(rows, "Themes (j/k select, Enter apply):")
				for i, t := range m.settingsData.Themes {
					mark := " "
					if t == m.settingsData.ActiveTheme {
						mark = "●"
					}
					row := fmt.Sprintf("  [%s] %s", mark, t)
					if i == m.settingsData.Selected {
						row = m.highlightRow(row, width-2)
					}
					rows = append(rows, row)
				}
				rows = append(rows, "")
				rows = append(rows, "Configuration:")
				if m.settingsData.ThemePath != "" {
					rows = append(rows, "  Active file: "+clean(m.settingsData.ThemePath, false))
				}
				if m.settingsData.ConfigPath != "" {
					rows = append(rows, "  Config:      "+clean(m.settingsData.ConfigPath, false))
				}
				if m.settingsData.RefreshMin > 0 {
					rows = append(rows, fmt.Sprintf("  Refresh:     %d min", m.settingsData.RefreshMin))
				}
				if m.settingsData.Icons != "" {
					rows = append(rows, "  Icons:       "+clean(m.settingsData.Icons, false))
				}
				if m.settingsData.Country != "" {
					rows = append(rows, "  Country:     "+clean(strings.ToUpper(m.settingsData.Country), false)+" (Left/Right to change)")
				}
				if m.settingsData.MPVPath != "" {
					rows = append(rows, "  MPV:         "+clean(m.settingsData.MPVPath, false))
				}
				modal = m.box("Settings · Left/Right country · e/Esc to close", rows, width, height, true, "", "")
			} else if m.help {
				modal = m.box("Keys · Esc closes", []string{
					"j/k arrows · g/G first/last item",
					"Details: Enter play/resume · a queue · c complete",
					"Preview completion requires subscription",
					"h/Left back · l/Right open · Tab lists · 1-6 focus",
					"Player (6): ,/. seek ±15s · arrows select · Enter activate",
					"C Chapters for current playback · Ch: § advertised/cached",
					m.Icon(SearchIcon, "/             Filter current list"), "Enter/Esc     Apply/cancel filter",
					"Ctrl+U        Clear filter input", "q / Ctrl+C    Quit",
					"e             Settings · live theme switch",
					"!             Full error/warning · j/k scroll · Esc dismiss",
					"Space pause/resume current · s stop · n next",
					"Episodes: Enter play · i details · a enqueue · A enqueue visible",
					"Queue: m actions (favorite) · d/Del remove · u undo · D clear",
					"Favorites (5): Enter play · d/Del un-favorite",
					"b My subscriptions / Discover",
					"f Search · Tab library/Apple/Podcast Index",
					"U RSS/website preview · t Apple Top 100",
					"Podcasts: u subscribe/unsubscribe · N add",
					"Library views / a show · o sort",
					"c mark completion · R refresh", "Details: j/k · Ctrl-d/u half page · g/G ends · PgUp/PgDn",
				}, width, height, true, "", "")
			}
			content = lipgloss.NewCompositor(lipgloss.NewLayer(content),
				lipgloss.NewLayer(modal).X((m.width-width)/2).Y((available-height)/2).Z(1)).Render()
		}
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
