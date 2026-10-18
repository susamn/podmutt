package ui

import (
	tea "charm.land/bubbletea/v2"
	"strings"
	"testing"
)

func TestLibraryCompositionAndPaste(t *testing.T) {
	m := resize(New(fixture()), 120, 32)
	m = m.Select(Episodes, "2").WithFocus(Episodes)
	item, ok := m.Selected(Episodes)
	if !ok || item.ID != "2" || m.Focus() != Episodes {
		t.Fatal("selection accessor failed")
	}
	m = m.Select(Episodes, "missing").Reset(Episodes).Reset(Details)
	if item, _ := m.Selected(Episodes); item.ID != "1" {
		t.Fatal("reset failed")
	}
	if _, ok := m.Selected(Details); ok {
		t.Fatal("details exposes list selection")
	}
	m = m.WithLibrary(Data{Podcasts: fixture().Podcasts, Episodes: fixture().Episodes, EpisodeLabel: "New · title", Notice: "Offline", Prompt: "URL: https://example.test/feed"})
	if m.data.Player.Status != "stopped" || len(m.data.Queue) != 1 || !strings.Contains(m.View().Content, "Add feed or podcast website") {
		t.Fatal("library clobbered playback or prompt")
	}
	m = m.WithLibrary(Data{Podcasts: fixture().Podcasts, Episodes: fixture().Episodes})
	m = press(m.WithFocus(Podcasts), "/")
	if !m.Busy() {
		t.Fatal("filter not busy")
	}
	next, _ := m.Update(tea.PasteMsg{Content: "Alpha"})
	m = next.(Model)
	if len(m.items(Podcasts)) != 1 {
		t.Fatal("filter paste ignored")
	}
	m = press(m, "enter")
	if m.Busy() {
		t.Fatal("filter still busy")
	}
	next, _ = m.Update(tea.PasteMsg{Content: "not applied"})
	m = next.(Model)
	if len(m.items(Podcasts)) != 1 {
		t.Fatal("unfocused paste applied")
	}
	m = press(m, "?")
	if !m.Busy() {
		t.Fatal("help not busy")
	}
}

func TestDetailsWrapScrollAndReset(t *testing.T) {
	for _, width := range []int{80, 140} {
		m := resize(New(fixture()), width, 32).WithFocus(Details).WithDetails(strings.Repeat("paragraph words that wrap at the available width\n", 50))
		next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		m = next.(Model)
		if m.detailScroll == 0 {
			t.Fatal("details did not scroll")
		}
		for range 50 {
			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			m = next.(Model)
		}
		atEnd := m.detailScroll
		next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		m = next.(Model)
		if m.detailScroll != atEnd {
			t.Fatal("scroll overshot end")
		}
		next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		m = next.(Model)
		if m.detailScroll >= atEnd {
			t.Fatal("page up did not move immediately")
		}
		m = m.WithDetails("another episode")
		if m.detailScroll != 0 {
			t.Fatal("new details retained scroll")
		}
		if !strings.Contains(m.View().Content, "another episode") {
			t.Fatal("details missing")
		}
	}
}
