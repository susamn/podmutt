package ui

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Player contains presentation-only transport data, independent of the selected
// episode's description. Progress is display-only; seeking is not supported.
type Player struct {
	Status, Title, Position, Duration string
	Error, Warning                    string
	Progress                          float64
	HasCurrent                        bool
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
		return "Next", "next", len(m.data.Queue) > 0
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
	player := m.data.Player
	status := player.Status
	if status == "" {
		status = "stopped"
	}
	title := clean(player.Title, false)
	if title == "" {
		title = "No episode selected"
	}
	summary := status + " · " + title
	if player.Error != "" {
		summary = status + " · " + clean(player.Error, false)
	}
	capacity := max(0, height-3)
	rows := []string{summary}
	// Reserve the last two rows for progress/controls. Extra metadata must never
	// push controls down, regardless of title/description/error length.
	extra := capacity - 3
	if extra > 0 && player.Warning != "" {
		rows = append(rows, "Media: "+clean(player.Warning, false))
		extra--
	}
	if extra > 0 {
		if player.Error == "" {
			rows[0] = status
		}
		rows = append(rows, title)
		extra--
	}
	for extra > 0 {
		rows = append(rows, "")
		extra--
	}
	if capacity >= 3 {
		rows = append(rows, progressLine(player, max(0, width-2)))
	}
	var buttons []string
	for i := 0; i < 3; i++ {
		label, _, enabled := m.playerControl(i)
		style := lipgloss.NewStyle().Faint(!enabled)
		if m.focus == PlayerPanel && m.playerButton == i {
			style = style.Reverse(true)
		}
		buttons = append(buttons, style.Render("["+label+"]"))
	}
	rows = append(rows, strings.Join(buttons, " "))
	// Extremely small layouts prioritize controls over metadata.
	if capacity < 2 {
		rows = rows[len(rows)-min(capacity, len(rows)):]
	}
	return box("5 Player", rows, width, height, m.focus == PlayerPanel)
}
func progressLine(player Player, width int) string {
	position, duration := player.Position, player.Duration
	if position == "" {
		position = "0:00"
	}
	if duration == "" {
		duration = "--:--"
	}
	stamp := clean(position+" / "+duration, false)
	cells := min(40, width-ansi.StringWidth(stamp)-3)
	if cells < 3 {
		return ansi.Truncate(stamp, max(0, width), "…")
	}
	progress := player.Progress
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		progress = 0
	}
	progress = max(0, min(1, progress))
	filled := int(progress * float64(cells))
	return fmt.Sprintf("%s [%s%s]", stamp, strings.Repeat("=", filled), strings.Repeat("-", cells-filled))
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
