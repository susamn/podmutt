package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func rowContaining(t *testing.T, text, needle string) int {
	t.Helper()
	for i, row := range strings.Split(ansi.Strip(text), "\n") {
		if strings.Contains(row, needle) {
			return i
		}
	}
	t.Fatalf("missing %q", needle)
	return -1
}
func TestDescriptionsNeverMovePlayerOrControls(t *testing.T) {
	for _, size := range [][2]int{{100, 24}, {140, 32}, {160, 50}} {
		data := fixture()
		data.Player = Player{Status: "playing", HasCurrent: true, Title: "Current audio", Position: "0:42", Duration: "2:00", Progress: .35}
		m := resize(New(data), size[0], size[1]).WithFocus(Details)
		available := size[1] - statusRows
		want := available - min(7, available-available/2-5)
		for _, description := range []string{"Short description", strings.Repeat("very long paragraph words that must wrap 界\n", 2000)} {
			m = m.WithDetails(description)
			if got := rowContaining(t, m.View().Content, "Now playing"); got != want {
				t.Fatalf("%v player row=%d want %d", size, got, want)
			}
			if got := rowContaining(t, m.View().Content, "Details"); got != available/2 {
				t.Fatal("details escaped cell")
			}
			if controls := rowContaining(t, m.View().Content, "[Pause] [Stop] [Next]"); controls <= want || controls >= size[1]-statusRows {
				t.Fatal("controls hidden or outside viewport")
			}
			if !strings.Contains(ansi.Strip(m.View().Content), "0:42") || !strings.Contains(ansi.Strip(m.View().Content), "2:00") {
				t.Fatal("timeline missing")
			}
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("overflow")
				}
			}
			if lines := strings.Count(m.View().Content, "\n") + 1; lines != size[1] {
				t.Fatal("height changed", lines)
			}
		}
		for range 10 {
			next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			m = next.(Model)
		}
		if m.detailScroll == 0 {
			t.Fatal("details cannot scroll")
		}
		if rowContaining(t, m.View().Content, "Now playing") != want {
			t.Fatal("scroll moved player")
		}
		if strings.Contains(strings.Join(m.detailRows(60), "\n"), "Current audio") {
			t.Fatal("transport still embedded in details")
		}
		offset := m.detailScroll
		player := m.data.Player
		player.Position = "0:43"
		m = m.WithPlayback(m.data.Queue, player)
		if m.detailScroll != offset {
			t.Fatal("progress update rewound details")
		}
		m = resize(m, 140, 60)
		_, height := m.detailsSize()
		if m.detailScroll > max(0, len(m.detailRows(84))-max(1, height-3)) {
			t.Fatal("resize left stale scroll")
		}
	}
}
func TestCompactDetailsAndQueueLeavePlayerVisible(t *testing.T) {
	for _, size := range [][2]int{{30, 12}, {80, 24}, {99, 40}} {
		for _, panel := range []Panel{Podcasts, Episodes, Queue, Details, PlayerPanel} {
			m := resize(New(fixture()), size[0], size[1]).WithDetails(strings.Repeat("description\n", 500)).WithFocus(panel)
			if rowContaining(t, m.View().Content, "Now playing") != 0 {
				t.Fatal("compact player moved", size, panel)
			}
			if !strings.Contains(ansi.Strip(m.View().Content), "[Play] [Stop] [Next]") {
				t.Fatal("compact controls missing")
			}
			if lines := strings.Count(m.View().Content, "\n") + 1; lines != size[1] {
				t.Fatal("compact height expanded", size, lines)
			}
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("compact overflow")
				}
			}
		}
	}
}
func TestPlayerDisplaysVolumeAtCompactAndWideWidths(t *testing.T) {
	for _, tc := range []struct {
		width int
		want  string
	}{{30, "V105%"}, {80, "Vol 105%"}} {
		m := New(Data{Player: Player{Volume: 105, HasVolume: true}})
		pane := ansi.Strip(m.playerPane(tc.width, 8))
		if !strings.Contains(pane, tc.want) {
			t.Fatalf("width %d player volume missing %q: %s", tc.width, tc.want, pane)
		}
	}
}

func TestPlayerButtonsUseSharedActionsAndDisabledCapabilities(t *testing.T) {
	var commands []string
	m := resize(New(fixture()), 140, 32).WithFocus(PlayerPanel).WithActions(func(command string) tea.Cmd {
		return func() tea.Msg { commands = append(commands, command); return nil }
	})
	update := func(key tea.KeyPressMsg) {
		t.Helper()
		next, cmd := m.Update(key)
		m = next.(Model)
		if cmd != nil {
			cmd()
		}
	}
	// No current episode: Play is visibly unavailable and cannot dispatch.
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(commands) != 0 {
		t.Fatal("empty play dispatched")
	}
	m = m.WithPlayback(m.data.Queue, Player{Status: "playing", HasCurrent: true, CanNext: true})
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	update(tea.KeyPressMsg{Code: tea.KeyRight})
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	update(tea.KeyPressMsg{Code: tea.KeyRight})
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if fmt.Sprint(commands) != "[toggle stop next]" {
		t.Fatal(commands)
	}
	update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.playerButton != 0 {
		t.Fatal("right did not wrap")
	}
	update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.playerButton != 2 {
		t.Fatal("left did not wrap")
	}
	m = m.WithPlayback([]Item{{ID: "current", Title: "Current episode"}}, Player{Status: "paused", HasCurrent: true})
	before := len(commands)
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(commands) != before {
		t.Fatal("current-only next dispatched")
	}
	m = press(m, "?")
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(commands) != before {
		t.Fatal("help dispatched player command")
	}
	m = press(m, "esc")
	m = m.WithDialog(&Dialog{Title: "Modal"})
	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(commands) != before {
		t.Fatal("dialog dispatched command")
	}
	m = m.WithDialog(nil).WithFocus(Episodes)
	m = press(m, "/")
	update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if len(commands) != before || m.filters[Episodes] != " " {
		t.Fatal("filter Space was not literal")
	}
	noActions := New(Data{Player: Player{HasCurrent: true}}).WithFocus(PlayerPanel)
	if _, cmd := noActions.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("missing action handler")
	}
	if noActions.WithFocus(Panel(99)).Focus() != PlayerPanel {
		t.Fatal("invalid focus accepted")
	}
}
func TestPlayerFormattingBoundsAndErrorPriority(t *testing.T) {
	for _, progress := range []float64{-1, 0, .5, 1, 100, math.NaN(), math.Inf(1)} {
		for _, width := range []int{0, 4, 15, 40, 100} {
			line := progressLine(Player{Progress: progress}, width)
			if ansi.StringWidth(line) > width {
				t.Fatal("progress width overflow", width, line)
			}
		}
	}
	player := Player{Status: "failed", Title: strings.Repeat("title", 500), Error: "persistent failure\x1b[2J", Warning: "media offline", HasCurrent: true, Position: "0:42", Duration: "2:00"}
	m := New(Data{Player: player}).WithFocus(PlayerPanel)
	output := ansi.Strip(m.playerPane(84, 8))
	for _, text := range []string{"persistent failure", "media offline", "0:42", "2:00", "[Play]", "[Stop]", "[Next]"} {
		if !strings.Contains(output, text) {
			t.Fatal("player lost " + text)
		}
	}
	m = m.WithPlayerError("action error")
	if m.data.Player.Title != player.Title || m.data.Player.Error != "action error" {
		t.Fatal("action error replaced transport")
	}
	// Very short callers still keep controls instead of overflowing the box.
	if !strings.Contains(ansi.Strip(m.pane(PlayerPanel, 30, 4)), "[Play] [Stop] [Next]") {
		t.Fatal("short player hid controls")
	}
}

func TestCompactPlayerHierarchy(t *testing.T) {
	for _, tc := range []struct {
		status    string
		wantGlyph string
	}{
		{"playing", "Playing"},
		{"paused", "Paused"},
		{"stopped", "Stopped"},
		{"failed", "Failed"},
	} {
		m := New(Data{Player: Player{Status: tc.status, HasCurrent: true, Title: "Test Song", Channel: "Podcast Channel", Position: "01:23", Duration: "04:56", Progress: 0.5}})
		pane := ansi.Strip(m.playerPane(80, 8))
		if !strings.Contains(pane, tc.wantGlyph) {
			t.Fatalf("expected glyph %q in status %q, got: %s", tc.wantGlyph, tc.status, pane)
		}
		if !strings.Contains(pane, "Test Song") || !strings.Contains(pane, "Podcast Channel") {
			t.Fatalf("expected track and channel in: %s", pane)
		}
		if !strings.Contains(pane, "●") || !strings.Contains(pane, "01:23") {
			t.Fatalf("expected slider knob in: %s", pane)
		}
	}
}
