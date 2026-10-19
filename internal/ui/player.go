package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Player contains presentation-only transport data, independent of the selected
// episode's description. Progress is display-only; ,/. dispatch explicit seeks.
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
func (m Model) playerPane(width, height int) string {
	player := m.displayPlayer()
	status := player.Status
	if status == "" {
		status = "stopped"
	}
	pal := m.Theme()
	innerWidth := max(0, width-2)
	innerHeight := max(0, height-2)

	topRow := m.renderPlayerTopLine(player, innerWidth)
	sliderRow := m.renderSliderLine(player, innerWidth)
	transportRow := m.renderTransportRow(player, innerWidth)

	var rows []string
	rows = append(rows, topRow)

	vizRows := min(2, max(0, innerHeight-3))
	if innerHeight >= 6 {
		if frame := m.animationFrame(max(0, innerWidth-4)); frame != "" {
			rows = append(rows, "  "+frame)
		}
	}

	if vizRows > 0 {
		t := player.PositionSeconds
		if t <= 0 && status == "playing" {
			t = float64(time.Now().UnixNano()) / 1e9
		}
		vizLines := globalVisualizer.Render(innerWidth, vizRows, t, status, pal)
		rows = append(rows, vizLines...)
	}

	rows = append(rows, sliderRow)
	rows = append(rows, transportRow)

	if len(rows) > innerHeight {
		if innerHeight <= 2 {
			rows = []string{sliderRow, transportRow}
		} else if innerHeight == 3 {
			rows = []string{topRow, sliderRow, transportRow}
		} else {
			rows = rows[len(rows)-innerHeight:]
		}
	}

	return m.box(m.Icon(stateIcon(status), "Now playing"), rows, width, height, m.focus == PlayerPanel, "[5]", "")
}

func (m Model) renderPlayerTopLine(player Player, width int) string {
	pal := m.Theme()
	status := player.Status
	if status == "" {
		status = "stopped"
	}
	glyph := "■"
	glyphColor := pal.BrightRed
	statusText := "STOPPED"
	switch status {
	case "playing":
		glyph = "●"
		glyphColor = pal.BrightGreen
		statusText = "PLAYING"
	case "paused":
		glyph = "⏸"
		glyphColor = pal.BrightYellow
		statusText = "PAUSED"
	case "failed":
		glyph = "!"
		glyphColor = pal.BrightRed
		statusText = "FAILED"
	}

	pill := lipgloss.NewStyle().Foreground(lipgloss.Color(string(glyphColor))).Bold(true).Render(glyph + " " + statusText)
	if player.Error != "" {
		pill = lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightRed))).Bold(true).Render(glyph + " " + clean(player.Error, false))
	}

	badge := ""
	if player.Channel != "" {
		chanColor := lipgloss.Color(string(pal.ChannelColor(player.Channel)))
		badge = "[ " + lipgloss.NewStyle().Foreground(chanColor).Render(ansi.Truncate(player.Channel, 24, "…")) + " ]"
	}

	title := clean(player.Title, false)
	if title == "" {
		title = "No episode selected"
	}

	fixedWidth := ansi.StringWidth(pill) + ansi.StringWidth(badge) + 6
	availTitle := max(8, width-fixedWidth)
	titleCol := ansi.Truncate(title, availTitle, "…")

	pad := max(1, width-ansi.StringWidth(pill)-ansi.StringWidth(titleCol)-ansi.StringWidth(badge)-4)
	if badge != "" {
		return "  " + pill + "   " + titleCol + strings.Repeat(" ", pad) + badge
	}
	return "  " + pill + "   " + titleCol
}

func (m Model) renderSliderLine(player Player, width int) string {
	pal := m.Theme()
	pos := player.Position
	if pos == "" {
		pos = "0:00"
	}
	dur := player.Duration
	if dur == "" {
		dur = "--:--"
	}

	countdown := ""
	if player.DurationSeconds > 0 && player.PositionSeconds >= 0 {
		remSec := max(0, player.DurationSeconds-player.PositionSeconds)
		countdown = " [-" + progressTime(remSec) + "]"
	}

	timeSummary := pos + " / " + dur
	rightText := timeSummary + countdown
	if width < 60 {
		rightText = timeSummary
	}

	fixed := 2 + ansi.StringWidth(pos) + 2 + 2 + ansi.StringWidth(rightText)
	sliderWidth := max(6, width-fixed)

	progress := player.Progress
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		progress = 0
	}
	progress = max(0, min(1, progress))

	knobPos := int(math.Round(progress * float64(sliderWidth-1)))
	leftLen := max(0, knobPos)
	rightLen := max(0, sliderWidth-1-leftLen)

	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Cyan)))
	knobStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(pal.BrightCyan)))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Muted)))

	leftTrack := cyanStyle.Render(strings.Repeat("─", leftLen))
	knob := knobStyle.Render("●")
	rightTrack := mutedStyle.Render(strings.Repeat("─", rightLen))

	return "  " + pos + "  " + leftTrack + knob + rightTrack + "  " + rightText
}

func (m Model) renderTransportRow(player Player, width int) string {
	pal := m.Theme()
	chipBack := lipgloss.NewStyle().Faint(!player.CanSeek).Render("[◀◀ ,] 15s")
	chipFwd := lipgloss.NewStyle().Faint(!player.CanSeek).Render("[.] 15s ▶▶")

	var buttons []string
	for i := 0; i < 3; i++ {
		label, _, enabled := m.playerControl(i)
		style := lipgloss.NewStyle().Faint(!enabled)
		if m.focus == PlayerPanel && m.playerButton == i {
			style = style.Reverse(true)
		}
		buttons = append(buttons, style.Render("["+label+"]"))
	}
	btnRow := strings.Join(buttons, " ")

	if player.Warning != "" {
		warnText := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightYellow))).Render("Media: " + clean(player.Warning, false))
		return "  " + btnRow + "   " + warnText
	}

	if width < 55 {
		return "  " + btnRow
	}

	gap := max(1, (width-ansi.StringWidth(chipBack)-ansi.StringWidth(btnRow)-ansi.StringWidth(chipFwd)-6)/2)
	gapStr := strings.Repeat(" ", min(4, gap))
	return "  " + chipBack + gapStr + btnRow + gapStr + chipFwd
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

func progressLine(player Player, width int) string {
	return Model{}.progressLine(player, width)
}

// compactHeights reserves a persistent player; at the minimum terminal size,
// show one library pane instead of squeezing three panes into unusable rows.
func (m Model) compactHeights() (top, middle, player int) {
	available := max(0, m.height-1)
	player = 6
	if m.height < 14 {
		return 0, max(0, available-5), 5
	}
	top = max(4, min(6, (available-player)/3))
	middle = available - player - top
	return
}
func (m Model) detailsSize() (width, height int) {
	if m.compact() {
		_, height, _ = m.compactHeights()
		return m.width, height
	}
	available := max(0, m.height-1)
	bottom := available - available/2
	return m.width - m.width*2/5, bottom / 2
}
