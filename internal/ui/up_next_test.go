package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestUpNextHasPendingCountsAndNoTransportRows(t *testing.T) {
	pending := []Item{{ID: "one", Title: "First pending"}, {ID: "two", Title: "Second pending"}}
	for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
		for _, status := range []string{"playing", "paused", "stopped", "failed"} {
			for _, icons := range []string{"unicode", "plain", "nerd"} {
				m := resize(New(Data{Queue: pending, Player: Player{Title: "Current audio", HasCurrent: true, Status: status}}), size[0], size[1]).WithIcons(icons).WithFocus(Queue)
				pane := ansi.Strip(m.pane(Queue, size[0], 8))
				if !strings.Contains(pane, "Queue · 2") || strings.Contains(pane, "Current audio") {
					t.Fatal("pending list includes current or wrong count", pane)
				}
				m = press(m, "/", "F", "i", "r", "s", "t", "enter")
				pane = ansi.Strip(m.pane(Queue, size[0], 8))
				if !strings.Contains(pane, "Queue · 1/2") || strings.Contains(pane, "Second pending") {
					t.Fatal("filtered count or list wrong", pane)
				}
				m = press(m, "esc", "/", "z", "enter")
				pane = ansi.Strip(m.pane(Queue, size[0], 8))
				if !strings.Contains(pane, "Queue · 0/2") || !strings.Contains(pane, "No matching queue") {
					t.Fatal("filter hides real pending work without explanation", pane)
				}
				for _, line := range strings.Split(m.View().Content, "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("layout overflow", size, status, icons)
					}
				}
				if !strings.Contains(ansi.Strip(m.View().Content), "Now playing") {
					t.Fatal("current transport pane disappeared")
				}
			}
		}
	}
	m := resize(New(Data{Player: Player{Title: "Only current", HasCurrent: true}}), 140, 40).WithFocus(Queue)
	pane := ansi.Strip(m.pane(Queue, 80, 8))
	if !strings.Contains(pane, "Queue · 0") || !strings.Contains(pane, "Queue is empty") {
		t.Fatal("current-only playback is not an empty pending list", pane)
	}
}
