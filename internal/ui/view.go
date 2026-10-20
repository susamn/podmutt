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

var titles = [4]string{"Library", "Episodes", "Queue", "Details"}

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
	if panel == Episodes {
		rows-- // pinned table header
	}
	return max(0, rows)
}

func (m Model) listPageSize() int {
	available := max(0, m.height-1)
	height := available
	g := m.wideLayout()
	if m.focus == Episodes {
		height = g.episodes
	} else if m.focus == Queue {
		height = g.queue
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
					text = rows[i]
				}
				merged[i] = art + strings.Repeat(" ", max(0, m.artColumns-ansi.StringWidth(art))) + " " + text
			}
			rows = merged
		}
	} else {
		items := m.items(panel)
		if panel == Episodes {
			rows = append(rows, m.renderEpisodesHeader(width))
		}
		rows = append(rows, m.renderList(panel, items, width, listCapacity(panel, height))...)
		if len(items) == 0 {
			empty := "No items"
			if panel == Podcasts && m.data.PodcastEmpty != "" {
				empty = clean(m.data.PodcastEmpty, false)
			} else if panel == Episodes && m.data.EpisodeEmpty != "" {
				empty = clean(m.data.EpisodeEmpty, false)
			} else if panel == Queue {
				empty = "Queue is empty · a adds an episode"
				if m.HasFilter(Queue) {
					empty = "No matching queue episodes"
				}
			}
			rows = append(rows, empty)
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
		title += " · % left · " + m.queueOrderLabel()
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
	pal := m.Theme()
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(pal.LightForeground)))
	dateWidth := 10
	durWidth := 8
	gutterWidth := 6
	innerWidth := max(0, width-2)
	avail := max(0, innerWidth-gutterWidth)

	if width < 70 {
		availText := max(8, avail-durWidth-1)
		return headerStyle.Render("      " + padRight("Title", availText) + " " + padLeft("Duration", durWidth))
	}

	fixedWidths := dateWidth + durWidth + 4
	slack := max(16, avail-fixedWidths)
	titleBudget := max(8, (slack*60)/100)
	chanBudget := max(6, slack-titleBudget)

	hdrText := padRight("Title", titleBudget) + "  " + padRight("Channel", chanBudget) + "  " + padRight("Date", dateWidth) + " " + padLeft("Duration", durWidth)
	return headerStyle.Render("      " + hdrText)
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
		return prefix + m.Icon(ShowIcon, lipgloss.NewStyle().Foreground(chanColor).Render(title))
	case Episodes:
		return m.renderEpisodeRow(item, width, selected)
	case Queue:
		return m.renderQueueRow(item, width, selected)
	default:
		return prefix + m.Icon(panelIcon(panel), clean(item.Title, false))
	}
}

func formatTitleAndChannel(title, channel string, slack int, pal theme.Palette) string {
	titleBudget := max(8, (slack*60)/100)
	chanBudget := max(6, slack-titleBudget)

	titleCol := ansi.Truncate(title, titleBudget, "…")
	titlePad := max(0, titleBudget-ansi.StringWidth(titleCol))

	chanColor := lipgloss.Color(string(pal.ChannelColor(channel)))
	chanTrunc := ansi.Truncate(channel, chanBudget, "…")
	chanPad := max(0, chanBudget-ansi.StringWidth(chanTrunc))
	chanCol := lipgloss.NewStyle().Foreground(chanColor).Render(chanTrunc) + strings.Repeat(" ", chanPad)

	return titleCol + strings.Repeat(" ", titlePad) + "  " + chanCol
}

func (m Model) renderEpisodeRow(item Item, width int, selected bool) string {
	prefix := "  "
	pal := m.Theme()
	gutter := "[ ] "
	if item.Queued {
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Accent))).Render("[+] ")
	} else if item.Completed {
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Green))).Render("[✓] ")
	}

	titleText := item.PlainTitle
	if titleText == "" {
		titleText = clean(item.Title, false)
	}

	if item.Current {
		titleText = playerStatus(m.data.Player) + " · " + titleText
	}
	if item.Channel == "" && item.Date == "" && item.Duration == "" {
		return prefix + m.Icon(EpisodeIcon, clean(item.Title, false))
	}

	dateWidth := 10
	durWidth := 8
	gutterWidth := 4
	innerWidth := max(0, width-2-ansi.StringWidth(prefix))
	avail := max(0, innerWidth-gutterWidth)

	dateStr := item.Date
	durStr := item.Duration
	if durStr == "" {
		durStr = "--:--"
	}

	if width < 70 {
		availText := max(8, avail-durWidth-1)
		titleCol := ansi.Truncate(titleText, availText, "…")
		pad := max(0, availText-ansi.StringWidth(titleCol))
		return prefix + gutter + titleCol + strings.Repeat(" ", pad) + " " + durStr
	}

	fixedWidths := dateWidth + durWidth + 4
	slack := max(16, avail-fixedWidths)
	titleAndChan := formatTitleAndChannel(titleText, item.Channel, slack, pal)

	datePad := max(0, dateWidth-ansi.StringWidth(dateStr))
	dateCol := dateStr + strings.Repeat(" ", datePad)

	durPad := max(0, durWidth-ansi.StringWidth(durStr))
	durCol := strings.Repeat(" ", durPad) + durStr

	return prefix + gutter + titleAndChan + "  " + dateCol + " " + durCol
}

func (m Model) renderQueueRow(item Item, width int, selected bool) string {
	prefix := "  "
	if item.Index > 0 {
		prefix += fmt.Sprintf("%d. ", item.Index)
	}
	title := clean(item.PlainTitle, false)
	if title == "" {
		title = clean(item.Title, false)
	}
	if item.Current {
		title = playerStatus(m.data.Player) + " · " + title
	}
	remaining := FormatRemainingPercent(item.PositionSeconds, item.DurationSeconds)
	if item.Current {
		remaining = FormatRemainingPercent(m.data.Player.PositionSeconds, m.data.Player.DurationSeconds)
		if remaining != "--" {
			player := m.displayPlayer()
			remaining = FormatRemainingPercent(player.Progress*player.DurationSeconds, player.DurationSeconds)
		}
	}
	innerWidth := max(0, width-2)
	if innerWidth < 5 {
		return ansi.Truncate(remaining, innerWidth, "…")
	}
	// Reserve the percentage before truncating identity, so long titles and
	// narrow layouts cannot push it out. Elapsed clocks stay in Player only.
	textWidth := innerWidth - 5
	text := title
	if item.Channel != "" {
		text = formatTitleAndChannel(title, clean(item.Channel, false), max(0, textWidth-2-ansi.StringWidth(prefix)), m.Theme())
	}
	return padRight(prefix+text, textWidth) + " " + padLeft(remaining, 4)
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
	case Queue:
		hints = []hint{
			{"d/Del", "remove"},
			{"u", "undo"},
			{"j/k", "select"},
			{"Enter", "play"},
			{"Space", "pause/resume"},
			{"i", "details"},
			{"o", "sort view"},
			{"/", "search"},
			{"D", "clear"},
			{",/.", "seek"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}
	default:
		if m.focus == Podcasts && !isDiscover {
			hints = []hint{{"j/k", "show"}, {"/", "search"}, {"o", "sort episodes"}, {"Tab/2", "episodes"}, {"f", "find podcast"}, {"Space", "pause/resume"}, {"?", "help"}, {"q", "quit"}}
		} else if isDiscover {
			hints = []hint{
				{"/", "search list"},
				{"f", "find podcast"},
				{"Tab", "episodes"},
				{"b", "library"},
				{"t", "top25"},
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
				{"/", "search"},
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
	hintText := m.formatHints(hints)
	if m.data.Notice != "" && m.width >= 80 {
		pal := m.Theme()
		noticeText := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightYellow))).Render(clean(m.data.Notice, false))
		hintText = noticeText + " · " + hintText
	}
	return hintText
}

func renderInputBox(label, value string, width int, pal theme.Palette) string {
	if label == "" {
		label = "Input"
	}
	fieldWidth := max(16, width-6-ansi.StringWidth(label)-4)
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
		rows = append(rows, strings.Split(ansi.Wrap(l, max(1, width-4), ""), "\n")...)
	}

	if dialog.InputLabel != "" || dialog.InputValue != "" {
		rows = append(rows, "", renderInputBox(dialog.InputLabel, dialog.InputValue, width, pal))
	}

	if dialog.Hint != "" {
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
			parts = append(parts, m.playerPane(m.width, player), m.pane(panel, m.width, middle))
			content = lipgloss.JoinVertical(lipgloss.Left, parts...)
		} else {
			g := m.wideLayout()
			middle := lipgloss.JoinVertical(lipgloss.Left, m.pane(Episodes, g.episodeWidth, g.episodes), m.pane(Details, g.episodeWidth, g.details))
			upper := lipgloss.JoinHorizontal(lipgloss.Top, middle, m.pane(Queue, g.queueWidth, g.queue))
			body := lipgloss.JoinVertical(lipgloss.Left, upper, m.playerPane(g.playerWidth, g.player))
			content = lipgloss.JoinHorizontal(lipgloss.Top, m.pane(Podcasts, g.libraryWidth, available), body)
		}
		hints := m.bottomHints()
		content += "\n" + ansi.Truncate(hints, m.width, "…")
		if m.help || m.settings || m.data.Prompt != "" || m.data.Dialog != nil {
			width, height := min(72, m.width-2), min(16, m.height-2)
			if m.help || m.settings {
				height = min(20, m.height-2)
			}
			var modal string
			if m.data.Dialog != nil {
				modal = m.renderDialogModal(m.data.Dialog, width, height)
			} else if m.data.Prompt != "" {
				modal = m.renderPromptModal(clean(m.data.Prompt, false), width, height)
			} else if m.settings {
				height = min(20, m.height-2)
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
				if m.settingsData.MPVPath != "" {
					rows = append(rows, "  MPV:         "+clean(m.settingsData.MPVPath, false))
				}
				modal = m.box("Settings · e / Esc to close", rows, width, height, true, "", "")
			} else if m.help {
				modal = m.box("Keys · Esc closes", []string{
					"j/k arrows · g/G first/last item",
					"Details: Enter play/resume · a queue · c complete",
					"Preview completion requires subscription",
					"h/Left back · l/Right open · Tab lists · 1-5 focus",
					"Now playing (5): ,/. seek ±15s · arrows select · Enter activate",
					m.Icon(SearchIcon, "/             Filter current list"), "Enter/Esc     Apply/cancel filter",
					"Ctrl+U        Clear filter input", "q / Ctrl+C    Quit",
					"e             Settings · live theme switch",
					"Space pause/resume current · s stop · n next",
					"Episodes Enter play · i details · a enqueue · A enqueue visible",
					"Queue: o visual sort · d/Del remove pending · u undo",
					"Pending: Enter play · D clear (confirm)",
					"b My subscriptions / Discover",
					"f Search · Tab library/Apple/Podcast Index",
					"U RSS/website preview · t Apple US top 25",
					"Podcasts: u subscribe/unsubscribe · N add",
					"Library views / a show · o sort",
					"c mark completion · R refresh", "Details: j/k · Ctrl-d/u half page · g/G ends · PgUp/PgDn",
				}, width, height, true, "", "")
			}
			content = lipgloss.NewCompositor(lipgloss.NewLayer(content),
				lipgloss.NewLayer(modal).X((m.width-width)/2).Y((m.height-height)/2).Z(1)).Render()
		}
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
