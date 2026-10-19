package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

func queueIDs(items []Item) []string {
	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestQueueDisplaySortTailAndChannelSearchNeverMutateSource(t *testing.T) {
	source := []Item{
		{ID: "old", Title: "Older", Channel: "Zulu", PublishedAt: time.Unix(10, 0)},
		{ID: "new", Title: "Newer", Channel: "Zulu", PublishedAt: time.Unix(30, 0)},
		{ID: "alpha", Title: "Alpha release", Channel: "alpha", PublishedAt: time.Unix(20, 0)},
		{ID: "tie", Title: "Same date", Channel: "Alpha", PublishedAt: time.Unix(20, 0)},
		{ID: "unknown", Title: "Undated", Channel: "Zulu"},
	}
	original := append([]Item(nil), source...)
	m := New(Data{Queue: source}).WithFocus(Queue).Select(Queue, "old")
	check := func(want ...string) {
		t.Helper()
		if got := queueIDs(m.Visible(Queue)); !reflect.DeepEqual(got, want) {
			t.Fatal(got, want)
		}
		if !reflect.DeepEqual(source, original) {
			t.Fatal("presentation mutated source", source)
		}
	}
	check("new", "alpha", "tie", "old", "unknown")
	m = m.ToggleQueueOrder()
	check("alpha", "tie", "new", "old", "unknown")
	if selected, _ := m.Selected(Queue); selected.ID != "old" {
		t.Fatal("sort lost selection", selected)
	}
	if m.queueOrderLabel() != "channel / newest" {
		t.Fatal(m.queueOrderLabel())
	}
	m = m.WithQueueTail([]string{"new", "alpha", "new"})
	check("tie", "old", "unknown", "new", "alpha")
	if len(m.queueTail) != 2 {
		t.Fatal("duplicate manual tail", m.queueTail)
	}
	m = press(m, "/", "z", "u", "l", "u", "enter")
	check("old", "unknown", "new")
	for _, item := range m.Visible(Queue) {
		if item.Index < 1 {
			t.Fatal("missing display number")
		}
	}
	m = press(m, "esc").ToggleQueueOrder()
	check("new", "alpha", "tie", "old", "unknown")
	if m.queueOrderLabel() != "newest" {
		t.Fatal(m.queueOrderLabel())
	}
	m = m.WithQueueTail([]string{"unknown"}).WithPlayback(source[:4], Player{})
	if len(m.queueTail) != 0 {
		t.Fatal("removed tail survived projection")
	}
	m = m.WithPlayback(source, Player{})
	check("new", "alpha", "tie", "old", "unknown")
}

func TestSelectedRowsUseBackgroundNotCursorsAndQueueHasNoSecondClock(t *testing.T) {
	data := Data{
		Podcasts: []Item{{ID: "all", Title: "All episodes", Destination: true}, {ID: "show", Title: "Subscribed show"}},
		Episodes: []Item{{ID: "episode", PlainTitle: "Inspected title", Channel: "RSS show", Current: true, Duration: "48:00"}},
		Queue:    []Item{{ID: "episode", PlainTitle: "Audio identity", Channel: "RSS show", Current: true, Position: "12:34", Duration: "48:00", Remains: "74%"}},
		Player:   Player{Status: "paused", HasCurrent: true},
	}
	for _, palette := range []theme.Palette{theme.Default(), theme.SolarizedDark(), theme.Catppuccin()} {
		for _, icons := range []string{"plain", "unicode", "nerd"} {
			m := resize(New(data).WithIcons(icons).WithTheme(palette), 140, 40)
			for panel := Podcasts; panel <= Queue; panel++ {
				rows := m.renderList(panel, m.Visible(panel), 60, 10)
				if len(rows) == 0 || !strings.Contains(rows[0], "48;") || ansi.StringWidth(rows[0]) != 58 {
					t.Fatal("selection is not a full highlighted row", icons, panel, rows)
				}
				if strings.HasPrefix(strings.TrimSpace(ansi.Strip(rows[0])), "> ") {
					t.Fatal("selection cursor returned", rows)
				}
			}
			queue := ansi.Strip(m.pane(Queue, 60, 8))
			if !strings.Contains(queue, "Paused") || !strings.Contains(queue, "Audio identity") || strings.Contains(queue, "12:34") || strings.Contains(queue, "74%") || strings.Contains(queue, "Remains") {
				t.Fatal("queue identity/status or simplified rows", queue)
			}
			for _, size := range [][2]int{{30, 12}, {70, 20}, {100, 24}, {160, 44}} {
				for _, panel := range []Panel{Podcasts, Episodes, Queue, Details} {
					view := resize(m, size[0], size[1]).WithFocus(panel).View().Content
					lines := strings.Split(view, "\n")
					if len(lines) != size[1] {
						t.Fatal("layout height", size, len(lines))
					}
					for _, line := range lines {
						if ansi.StringWidth(line) > size[0] {
							t.Fatal("layout width", size, line)
						}
					}
					if !strings.Contains(ansi.Strip(view), "Now playing") {
						t.Fatal("transport hidden", size)
					}
				}
			}
		}
	}
}
