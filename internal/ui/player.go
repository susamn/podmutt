package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/chapters"
)

// Player is transport presentation, independent of the inspected episode.
type Player struct {
	Status, Title, Channel, Position, Duration string
	Error, Warning                             string
	PositionSeconds, DurationSeconds           float64
	ClockAt                                    time.Time
	Remaining                                  string
	Progress                                   float64
	HasCurrent                                 bool
	CanNext                                    bool
	CanSeek                                    bool
	Chapters                                   []chapters.Chapter
	ChapterLabel                               string
}

func (m Model) WithPlayerDecoration(enabled bool) Model {
	m.decoration = enabled
	m.scrollDetails(0)
	return m
}

func (m Model) playerControl(index int) (label, command string, enabled bool) {
	switch index {
	case 0:
		label = "Play"
		if m.data.Player.Status == "playing" {
			label = "Pause"
		}
		return label, "toggle", m.data.Player.HasCurrent
	case 1:
		return "Stop", "stop", m.data.Player.HasCurrent || m.data.Player.Status == "failed"
	default:
		return "Next", "next", m.data.Player.CanNext
	}
}

func (m Model) activatePlayer() tea.Cmd {
	_, command, enabled := m.playerControl(m.playerButton)
	if !enabled || m.actions == nil {
		return nil
	}
	return m.actions(command)
}

func playerStatus(player Player) string {
	switch player.Status {
	case "playing":
		return "Playing"
	case "paused":
		return "Paused"
	case "failed":
		return "Failed"
	default:
		return "Stopped"
	}
}

// Put the first line of both failures ahead of continuations, so one long error
// cannot hide an independent warning in a constrained viewport.
func (m Model) playerNotices(width int) []string {
	pal := m.Theme()
	var first, continuation []string
	for _, notice := range []struct{ text, label, color string }{
		{m.data.Player.Error, "Error: ", string(pal.BrightRed)},
		{m.data.Player.Warning, "Media (!): ", string(pal.BrightYellow)},
	} {
		if notice.text == "" {
			continue
		}
		rows := strings.Split(ansi.Wrap(notice.label+clean(notice.text, false), max(1, width-2), ""), "\n")
		if len(rows) > 2 {
			rows[1] = ansi.Truncate(rows[1], max(1, width-3), "") + "…"
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(notice.color))
		first = append(first, "  "+style.Render(rows[0]))
		if len(rows) > 1 {
			continuation = append(continuation, "  "+style.Render(rows[1]))
		}
	}
	return append(first, continuation...)
}

func (m Model) playerPane(width, height int) string {
	player := m.displayPlayer()
	innerWidth, innerHeight := max(0, width-2), max(0, height-2)
	title := m.renderPlayerTopLine(player, innerWidth)
	slider, transport := m.renderSliderLine(player, innerWidth), m.renderTransportRow(player, innerWidth)
	notices := m.playerNotices(innerWidth)
	var rows []string
	if innerHeight <= 2 {
		rows = []string{slider, transport}
	} else {
		budget := max(0, innerHeight-3)
		rows = []string{title}
		rows = append(rows, notices[:min(len(notices), budget)]...)
		spare := innerHeight - len(rows) - 2
		padding := min(2, spare) // surrender whitespace before any core content
		if padding > 0 {
			rows = append([]string{""}, rows...)
		}
		if m.decoration && !m.Busy() && spare-padding >= 2 {
			if frame := m.animationFrame(max(0, innerWidth-15)); frame != "" {
				rows = append(rows, "  "+frame)
				rows = append(rows, globalVisualizer.Render(innerWidth, 1, float64(m.animation.frame), player.Status, m.Theme())...)
			}
		}
		rows = append(rows, slider, transport)
		if padding > 1 {
			rows = append(rows, "")
		}
	}
	return m.box(m.Icon(stateIcon(player.Status), "Now playing · "+playerStatus(player)), rows, width, height, m.focus == PlayerPanel, "[6]", "")
}

func (m Model) renderPlayerTopLine(player Player, width int) string {
	title := clean(player.Title, false)
	if title == "" {
		title = "No current episode"
	}
	show := clean(player.Channel, false)
	available := max(0, width-4) // two cells of padding at each side
	chapterBudget := 0
	if player.ChapterLabel != "" && available >= 55 {
		chapterBudget = min(28, available/3)
		available -= chapterBudget + 3
	}
	titleBudget, showBudget := available, 0
	if show != "" && available > 3 {
		// Prefer the full channel, but retain a small readable title when a
		// pathological channel cannot fit. The title gets all remaining width.
		minimumTitle := min(8, ansi.StringWidth(title), available-3)
		showBudget = min(ansi.StringWidth(show), max(0, available-3-minimumTitle))
		if showBudget > 0 {
			titleBudget = available - 3 - showBudget
		}
	}
	line := lipgloss.NewStyle().Bold(true).Render(ansi.Truncate(title, titleBudget, "…"))
	if showBudget > 0 {
		line += " · " + m.channelCell(ansi.Truncate(show, showBudget, "…"))
	}
	if chapterBudget > 0 {
		line += " · " + ansi.Truncate(clean(player.ChapterLabel, false), chapterBudget, "…")
	}
	return ansi.Truncate("  "+line+"  ", max(0, width), "…")
}

func (m Model) renderSliderLine(player Player, width int) string {
	pos, dur := player.Position, player.Duration
	if pos == "" {
		pos = "0:00"
	}
	if dur == "" {
		dur = "--:--"
	}
	right := dur
	if width >= 60 && player.DurationSeconds > 0 {
		right += " · -" + progressTime(max(0, player.DurationSeconds-player.PositionSeconds))
	}
	cells := width - ansi.StringWidth(pos) - ansi.StringWidth(right) - 6
	if cells < 4 {
		return ansi.Truncate("  "+pos+" / "+dur, max(0, width), "…")
	}
	progress := player.Progress
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		progress = 0
	}
	progress = max(0, min(1, progress))
	knob := int(math.Round(progress * float64(cells-1)))
	pal := m.Theme()
	markers := make(map[int]bool)
	if player.DurationSeconds > 0 {
		for _, chapter := range player.Chapters {
			if chapter.Start > 0 && chapter.Start < player.DurationSeconds {
				cell := int(math.Round(chapter.Start / player.DurationSeconds * float64(cells-1)))
				markers[cell] = true
			}
		}
	}
	var bar strings.Builder
	for i := range cells {
		glyph, color := "─", pal.Muted
		if i < knob {
			color = pal.Cyan
		}
		if markers[i] {
			glyph = "│"
		}
		if i == knob {
			glyph, color = "●", pal.BrightCyan
		}
		bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(string(color))).Render(glyph))
	}
	return "  " + pos + "  " + bar.String() + "  " + right
}

func (m Model) renderTransportRow(player Player, width int) string {
	var buttons []string
	for i := range 3 {
		label, _, enabled := m.playerControl(i)
		style := lipgloss.NewStyle().Faint(!enabled)
		if m.focus == PlayerPanel && m.playerButton == i {
			style = style.Reverse(true)
		}
		buttons = append(buttons, style.Render("["+label+"]"))
	}
	row := "  " + strings.Join(buttons, " ")
	if width >= ansi.StringWidth(row)+15 {
		row += "   " + lipgloss.NewStyle().Faint(!player.CanSeek).Render(",/. seek 15s")
	}
	return row
}

func (m Model) progressLine(player Player, width int) string {
	position, duration := player.Position, player.Duration
	if position == "" {
		position = "0:00"
	}
	if duration == "" {
		duration = "--:--"
	}
	stamp := clean(position+" / "+duration, false)
	if player.Remaining != "" {
		stamp += " · -" + clean(player.Remaining, false)
	}
	cells := min(40, width-ansi.StringWidth(stamp)-5)
	if cells < 4 {
		return ansi.Truncate(stamp, max(0, width), "…")
	}
	progress := player.Progress
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		progress = 0
	}
	progress = max(0, min(1, progress))
	filled := int(progress * float64(cells))
	partial := ""
	if filled < cells {
		fraction := int((progress*float64(cells) - float64(filled)) * 8)
		if fraction > 0 {
			partial = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}[fraction]
		}
	}
	empty := cells - filled
	if partial != "" {
		empty--
	}
	pal := m.Theme()
	barText := strings.Repeat("█", filled) + partial + strings.Repeat("░", empty)
	cyanBar := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Cyan))).Render(barText)
	return fmt.Sprintf("%s [[%s]]", stamp, cyanBar)
}

func progressLine(player Player, width int) string { return Model{}.progressLine(player, width) }
