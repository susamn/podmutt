package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

func TestListRowsUsePlainAccentPlusWithoutTransportPrefix(t *testing.T) {
	item := Item{ID: "1", PlainTitle: "Literal episode", Channel: "RSS show", Queued: true, Current: true, Duration: "1:00", PositionSeconds: 25, DurationSeconds: 100}
	for _, palette := range []theme.Palette{theme.Default(), theme.SolarizedDark(), theme.Catppuccin()} {
		for _, status := range []string{"playing", "paused", "stopped", "failed"} {
			m := New(Data{Player: Player{Status: status, PositionSeconds: 25, DurationSeconds: 100}, Queue: []Item{item}}).WithTheme(palette)
			episode := ansi.Strip(m.renderEpisodeRow(item, 70, false))
			queue := ansi.Strip(m.renderQueueRow(item, 70, false))
			if !strings.HasPrefix(episode, "  + Literal episode") || strings.Contains(episode, "[+]") || strings.Contains(episode, " · Literal episode") || strings.Contains(queue, " · Literal episode") || !strings.Contains(queue, "75%") {
				t.Fatal("marker or transport prefix", episode, queue)
			}
		}
	}
}

func TestPlayerPaddingYieldsBeforeNoticesAndMinimumDetails(t *testing.T) {
	player := Player{HasCurrent: true, Title: "Current audio", Channel: "RSS show", Status: "paused", Position: "0:42", Duration: "2:00", Error: "Action failed", Warning: "Media offline"}
	for _, size := range [][2]int{{30, 12}, {30, 14}, {70, 20}, {100, 24}, {140, 44}, {180, 56}} {
		m := resize(New(Data{Player: player}), size[0], size[1]).WithEpisodeDetails(EpisodeDetails{Key: "pin", Title: "Inspected episode", Show: "RSS show", Inspecting: true}).WithFocus(Details)
		view := ansi.Strip(m.View().Content)
		for _, needle := range []string{"Current audio", "Inspected episode", "Action failed", "Media offline", "0:42", "[Play]", "[Stop]", "[Next]"} {
			if !strings.Contains(view, needle) {
				t.Fatal("padding displaced required content", size, needle, view)
			}
		}
		if len(strings.Split(view, "\n")) != size[1] || !utf8.ValidString(view) {
			t.Fatal("bad padded frame", size)
		}
		for _, row := range strings.Split(view, "\n") {
			if ansi.StringWidth(row) > size[0] {
				t.Fatal("padding overflow", size, row)
			}
		}
	}
	m := New(Data{})
	for _, width := range []int{0, 4, 10, 28, 78, 150} {
		line := m.renderPlayerTopLine(Player{Title: strings.Repeat("界", 100), Channel: strings.Repeat("語", 100)}, width)
		if ansi.StringWidth(line) > width || !utf8.ValidString(line) {
			t.Fatal("pathological channel overflow", width, line)
		}
	}
	line := ansi.Strip(m.renderPlayerTopLine(Player{Title: "Short episode", Channel: "A complete channel longer than twenty cells"}, 100))
	if line != "  Short episode · A complete channel longer than twenty cells  " {
		t.Fatal("short title or full channel lost", line)
	}
	longChannel := strings.Repeat("C", 92)
	if line := ansi.Strip(m.renderPlayerTopLine(Player{Title: "E", Channel: longChannel}, 100)); !strings.Contains(line, longChannel) {
		t.Fatal("unnecessarily truncated a channel that fits", line)
	}
}
