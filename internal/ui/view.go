package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var titles = [4]string{"Podcasts", "Episodes", "Queue", "Details"}

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
		if panel == Episodes {
			rows = append(rows, m.renderEpisodesHeader(width))
		} else if panel == Queue {
			rows = append(rows, m.renderQueueHeader(width))
		}
		visible := max(0, height-3-len(rows))
		start := max(0, m.selected[panel]-visible+1)
		for i := start; i < min(len(items), start+visible); i++ {
			rows = append(rows, m.renderRow(panel, items[i], width, i == m.selected[panel]))
		}
		if len(items) == 0 {
			empty := "No items"
			if panel == Podcasts && m.data.PodcastEmpty != "" {
				empty = clean(m.data.PodcastEmpty, false)
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
	if panel == Podcasts && m.data.PodcastLabel != "" {
		title += " · " + clean(m.data.PodcastLabel, false)
	}
	if panel == Episodes && m.data.EpisodeLabel != "" {
		title += " · " + clean(m.data.EpisodeLabel, false)
	}
	if panel < Details && m.filters[panel] != "" {
		title += " / " + clean(m.filters[panel], false)
	}
	tag := fmt.Sprintf("[%d]", panel+1)
	extraTag := ""
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
	if offset := m.colOffset[Episodes]; offset > 0 {
		runes := []rune(hdrText)
		if offset < len(runes) {
			hdrText = ansi.Truncate(string(runes[offset:]), avail, "…")
		}
	}
	return headerStyle.Render("      " + hdrText)
}

func (m Model) renderQueueHeader(width int) string {
	pal := m.Theme()
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(pal.LightForeground)))
	remWidth := 8
	timeWidth := 13
	idxWidth := 4
	gutterWidth := 4
	innerWidth := max(0, width-2)
	avail := max(0, innerWidth-gutterWidth-idxWidth-remWidth)

	if width < 70 {
		availText := max(8, avail-timeWidth-1)
		return headerStyle.Render("    #   Remains " + padRight("Title", availText) + " " + padLeft("Time", timeWidth))
	}

	slack := max(16, avail-timeWidth-3)
	titleBudget := max(8, (slack*60)/100)
	chanBudget := max(6, slack-titleBudget)

	hdrText := "Remains " + padRight("Title", titleBudget) + "  " + padRight("Channel", chanBudget) + "  " + padLeft("Time", timeWidth)
	if offset := m.colOffset[Queue]; offset > 0 {
		runes := []rune(hdrText)
		if offset < len(runes) {
			hdrText = ansi.Truncate(string(runes[offset:]), avail, "…")
		}
	}
	return headerStyle.Render("    #   " + hdrText)
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
	if selected {
		prefix = "> "
	}
	switch panel {
	case Podcasts:
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
			}
		}
		return prefix + m.Icon(ShowIcon, lipgloss.NewStyle().Foreground(chanColor).Render(title))
	case Episodes:
		return m.renderEpisodeRow(item, width, selected)
	case Queue:
		return m.renderQueueRow(item, width, selected)
	default:
		return prefix + m.Icon(panelIcon(panel), clean(item.Title, false))
	}
}

func (m Model) renderEpisodeRow(item Item, width int, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
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

	titleBudget := max(8, (slack*60)/100)
	chanBudget := max(6, slack-titleBudget)

	titleCol := ansi.Truncate(titleText, titleBudget, "…")
	titlePad := max(0, titleBudget-ansi.StringWidth(titleCol))

	chanColor := lipgloss.Color(string(pal.ChannelColor(item.Channel)))
	chanTrunc := ansi.Truncate(item.Channel, chanBudget, "…")
	chanPad := max(0, chanBudget-ansi.StringWidth(chanTrunc))
	chanCol := lipgloss.NewStyle().Foreground(chanColor).Render(chanTrunc) + strings.Repeat(" ", chanPad)

	datePad := max(0, dateWidth-ansi.StringWidth(dateStr))
	dateCol := dateStr + strings.Repeat(" ", datePad)

	durPad := max(0, durWidth-ansi.StringWidth(durStr))
	durCol := strings.Repeat(" ", durPad) + durStr

	row := prefix + gutter + titleCol + strings.Repeat(" ", titlePad) + "  " + chanCol + "  " + dateCol + " " + durCol
	if offset := m.colOffset[Episodes]; offset > 0 {
		runes := []rune(titleCol + strings.Repeat(" ", titlePad) + "  " + chanCol + "  " + dateCol + " " + durCol)
		if offset < len(runes) {
			row = prefix + gutter + ansi.Truncate(string(runes[offset:]), avail, "…")
		}
	}
	return row
}

func (m Model) renderQueueRow(item Item, width int, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	pal := m.Theme()
	gutter := "  "
	switch item.State {
	case "playing":
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightGreen))).Render("▶ ")
	case "paused":
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightYellow))).Render("⏸ ")
	case "stopped":
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightRed))).Render("■ ")
	}

	idxStr := "    "
	if item.Index > 0 {
		idxStr = fmt.Sprintf("%-3d ", item.Index)
	}

	remainsStr := item.Remains
	if remainsStr == "" {
		remainsStr = "100%"
	}
	remCol := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Cyan))).Render(padRight(remainsStr, 7)) + " "

	titleText := item.PlainTitle
	if titleText == "" {
		titleText = clean(item.Title, false)
	}

	if item.Channel == "" && item.Position == "" && item.Duration == "" {
		return prefix + m.Icon(QueueIcon, clean(item.Title, false))
	}

	timeStr := item.Position + "/" + item.Duration
	if item.Position == "" {
		timeStr = item.Duration
	}
	if timeStr == "" || timeStr == "/" {
		timeStr = "--:--"
	}

	timeWidth := 13
	gutterWidth := 2 + ansi.StringWidth(idxStr) + ansi.StringWidth(remCol)
	innerWidth := max(0, width-2-ansi.StringWidth(prefix))
	avail := max(0, innerWidth-gutterWidth)

	if width < 70 {
		availText := max(8, avail-timeWidth-1)
		titleCol := ansi.Truncate(titleText, availText, "…")
		pad := max(0, availText-ansi.StringWidth(titleCol))
		return prefix + gutter + idxStr + remCol + titleCol + strings.Repeat(" ", pad) + " " + timeStr
	}

	slack := max(16, avail-timeWidth-3)
	titleBudget := max(8, (slack*60)/100)
	chanBudget := max(6, slack-titleBudget)

	titleCol := ansi.Truncate(titleText, titleBudget, "…")
	titlePad := max(0, titleBudget-ansi.StringWidth(titleCol))

	chanColor := lipgloss.Color(string(pal.ChannelColor(item.Channel)))
	chanTrunc := ansi.Truncate(item.Channel, chanBudget, "…")
	chanPad := max(0, chanBudget-ansi.StringWidth(chanTrunc))
	chanCol := lipgloss.NewStyle().Foreground(chanColor).Render(chanTrunc) + strings.Repeat(" ", chanPad)

	timePad := max(0, timeWidth-ansi.StringWidth(timeStr))
	timeCol := strings.Repeat(" ", timePad) + timeStr

	row := prefix + gutter + idxStr + remCol + titleCol + strings.Repeat(" ", titlePad) + "  " + chanCol + "  " + timeCol
	if offset := m.colOffset[Queue]; offset > 0 {
		runes := []rune(remCol + titleCol + strings.Repeat(" ", titlePad) + "  " + chanCol + "  " + timeCol)
		if offset < len(runes) {
			row = prefix + gutter + idxStr + ansi.Truncate(string(runes[offset:]), avail, "…")
		}
	}
	return row
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
		hints = []hint{
			{"j/k", "scroll"},
			{"Ctrl-d/u", "half-page"},
			{"g/G", "top/bottom"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}
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
			{"j/k", "select"},
			{"Enter", "play"},
			{"J/K", "move"},
			{"d", "remove"},
			{"D", "clear"},
			{",/.", "seek"},
			{"Tab", "pane"},
			{"e", "settings"},
			{"?", "help"},
			{"q", "quit"},
		}
	default:
		if isDiscover {
			hints = []hint{
				{"f", "search"},
				{"U", "url"},
				{"Tab", "pane"},
				{"b", "library"},
				{"t", "top25"},
				{"Space", "play"},
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
				{"f", "search"},
				{"U", "url"},
				{"Tab", "pane"},
				{"b", "discover"},
				{"N", "add"},
				{"u", "unsub"},
				{"v", "view"},
				{"o", "sort"},
				{"c", "complete"},
				{"Space", "play"},
				{"s", "stop"},
				{"n", "next"},
				{",/.", "seek"},
				{"e", "settings"},
				{"?", "help"},
				{"q", "quit"},
			}
		}
	}

	hintText := m.formatHints(hints)
	if m.data.Notice != "" && m.width >= 80 {
		pal := m.Theme()
		noticeText := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightYellow))).Render(clean(m.data.Notice, false))
		hintText = noticeText + " · " + hintText
	}
	return hintText
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
		hints := m.bottomHints()
		content += "\n" + ansi.Truncate(hints, m.width, "…")
		if m.help || m.settings || m.data.Prompt != "" || m.data.Dialog != nil {
			width, height := min(72, m.width-2), min(16, m.height-2)
			if m.help || m.settings {
				height = min(20, m.height-2)
			}
			var modal string
			if m.data.Dialog != nil {
				dialog := m.data.Dialog
				rows := strings.Split(ansi.Wrap(clean(dialog.Body, true), max(1, width-2), ""), "\n")
				rows = append(rows, clean(dialog.Hint, false))
				modal = m.box(clean(dialog.Title, false), rows, width, height, true, "", "")
			} else if m.data.Prompt != "" {
				height = min(7, m.height-2)
				modal = m.box("Add feed or podcast website", []string{clean(m.data.Prompt, false), "Enter add · Esc cancel · Ctrl+U clear"}, width, height, true, "", "")
			} else if m.settings {
				height = min(20, m.height-2)
				width = min(64, m.width-4)
				var rows []string
				rows = append(rows, "Themes (j/k select, Enter apply):")
				for i, t := range m.settingsData.Themes {
					prefix := "  "
					if i == m.settingsData.Selected {
						prefix = "> "
					}
					mark := " "
					if t == m.settingsData.ActiveTheme {
						mark = "●"
					}
					rows = append(rows, fmt.Sprintf("%s[%s] %s", prefix, mark, t))
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
					"h/l, Tab · 1-5 focus pane",
					"Player (5): h/l seek ±15s · Left/Right select · Enter activate",
					m.Icon(SearchIcon, "/             Filter current list"), "Enter/Esc     Apply/cancel filter",
					"Ctrl+U        Clear filter input", "q / Ctrl+C    Quit",
					"e             Settings · live theme switch",
					"Space selected play/toggle · s stop · n next",
					"Episodes Enter details · a enqueue · A enqueue visible",
					"Queue: Enter play/promote · j/k select",
					"Pending: J/K move · d remove · D clear (confirm)",
					"b My subscriptions / Discover",
					"f Search · Tab library/Apple/Podcast Index",
					"U RSS/website preview · t Apple US top 25",
					"N add/subscribe · u subscribe/unsubscribe",
					"v New/Show · o newest/oldest/title",
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
