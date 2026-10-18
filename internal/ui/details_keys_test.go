package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDetailsVimNavigationIsFocusScoped(t *testing.T) {
	for _, size := range [][2]int{{140, 44}, {70, 20}, {30, 12}} {
		m := resize(New(fixture()), size[0], size[1]).WithFocus(Details).WithDetails(strings.Repeat("wrapped paragraph words\n", 100))
		before := m.selected
		m = press(m, "j")
		if m.detailScroll != 1 {
			t.Fatal("j did not scroll one line")
		}
		m = press(m, "k")
		if m.detailScroll != 0 {
			t.Fatal("k did not scroll one line")
		}
		_, height := m.detailsSize()
		half := max(1, max(1, height-3)/2)
		m = press(m, "ctrl+d")
		if m.detailScroll != half {
			t.Fatal("half-page down", m.detailScroll, half)
		}
		m = press(m, "ctrl+u")
		if m.detailScroll != 0 {
			t.Fatal("half-page up")
		}
		m = press(m, "G")
		end := m.detailScroll
		if end <= half {
			t.Fatal("G failed")
		}
		m = press(m, "j")
		if m.detailScroll != end {
			t.Fatal("past end")
		}
		m = m.WithPlayback(m.data.Queue, Player{Status: "playing", Title: "audio"})
		if m.detailScroll != end {
			t.Fatal("progress reset scroll")
		}
		m = press(m, "g")
		if m.detailScroll != 0 {
			t.Fatal("g failed")
		}
		m = press(m, "end")
		m = press(m, "up")
		if m.detailScroll != end-1 {
			t.Fatal("end/up failed")
		}
		m = press(m, "home")
		m = press(m, "down")
		if m.detailScroll != 1 {
			t.Fatal("home/down failed")
		}
		if m.selected != before {
			t.Fatal("Details keys moved list selection")
		}
		m = press(m, "?")
		m = press(m, "G")
		if m.detailScroll != 1 {
			t.Fatal("help did not guard scrolling")
		}
		m = press(m, "esc")
		m = m.WithDialog(&Dialog{Title: "Prompt"})
		m = press(m, "G")
		if m.detailScroll != 1 {
			t.Fatal("dialog did not guard scrolling")
		}
		m = m.WithDialog(nil)
		m = m.WithFocus(Episodes)
		m = press(m, "j")
		if m.detailScroll != 1 || m.selected == before {
			t.Fatal("list j no longer selects")
		}
		m = press(m, "/")
		m = press(m, "j")
		m = press(m, "g")
		if m.filters[Episodes] != "jg" {
			t.Fatal("input was not literal")
		}
		m = press(m, "ctrl+u")
		if m.filters[Episodes] != "" || m.detailScroll != 1 {
			t.Fatal("input Ctrl-u scrolled Details")
		}
		m = press(m, "esc").WithFocus(Details).WithDetails("short")
		m = press(m, "G")
		if m.detailScroll != 0 {
			t.Fatal("short details overflow")
		}
		m = resize(m, 200, 80)
		if m.detailScroll != 0 {
			t.Fatal("resize not clamped")
		}
	}
}

func TestDetailsKeysNeverDispatchPlayerActions(t *testing.T) {
	calls := 0
	m := resize(New(fixture()), 140, 44).WithFocus(Details).WithDetails(strings.Repeat("text\n", 100)).WithActions(func(string) tea.Cmd { calls++; return nil })
	for _, key := range []string{"j", "k", "g", "G", "ctrl+d", "ctrl+u"} {
		m = press(m, key)
	}
	if calls != 0 {
		t.Fatal("details dispatched playback")
	}
}
