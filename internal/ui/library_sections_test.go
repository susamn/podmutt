package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func sectionFixture() Data {
	data := Data{Podcasts: []Item{
		{ID: "new", Title: "Not started", Destination: true},
		{ID: "continue", Title: "Continue listening", Destination: true},
		{ID: "all", Title: "All episodes", Destination: true},
	}}
	for i := range 20 {
		data.Podcasts = append(data.Podcasts, Item{ID: fmt.Sprintf("show-%d", i), Title: fmt.Sprintf("Show %02d", i)})
	}
	return data
}

func TestLibrarySectionsAreQuietNonSelectableDecoration(t *testing.T) {
	m := resize(New(sectionFixture()), 140, 40).WithIcons("plain")
	rows := m.renderList(Podcasts, m.items(Podcasts), 56, 16)
	if rows[3] != "" || !strings.HasPrefix(ansi.Strip(rows[4]), "  Subscriptions ─") {
		t.Fatal("missing quiet spacing/heading", rows)
	}
	if len(m.Visible(Podcasts)) != 23 {
		t.Fatal("decoration became a selectable item")
	}
	m = press(m, "j", "j", "j")
	if item, _ := m.Selected(Podcasts); item.ID != "show-0" {
		t.Fatal("j stopped on section decoration", item)
	}
	m = press(m, "k")
	if item, _ := m.Selected(Podcasts); item.ID != "all" {
		t.Fatal("k stopped on section decoration", item)
	}
	for _, capacity := range []int{1, 3, 5} {
		m = m.Select(Podcasts, "show-0")
		rows := m.renderList(Podcasts, m.items(Podcasts), 30, capacity)
		for _, row := range rows {
			if row == "" {
				t.Fatal("short viewport wasted a row on spacing", capacity)
			}
		}
	}
	if rows := m.renderList(Podcasts, m.items(Podcasts), 30, 0); len(rows) != 0 {
		t.Fatal("zero capacity rendered rows")
	}
}

func TestSectionNavigationAndPagingKeepSelectionVisible(t *testing.T) {
	for _, size := range []struct{ width, height, paneWidth, paneHeight int }{
		{140, 40, 56, 19}, {100, 24, 40, 11}, {70, 20, 70, 4}, {30, 12, 30, 6},
	} {
		for _, icons := range []string{"unicode", "plain", "nerd"} {
			m := resize(New(sectionFixture()), size.width, size.height).WithIcons(icons)
			for _, key := range []string{"j", "j", "j", "k", "j", "G", "ctrl+u", "ctrl+d", "pgup", "pgdown", "g"} {
				m = press(m, key)
				selected, ok := m.Selected(Podcasts)
				if !ok {
					t.Fatal("navigation lost selection", key)
				}
				pane := ansi.Strip(m.pane(Podcasts, size.paneWidth, size.paneHeight))
				found := false
				for _, row := range strings.Split(pane, "\n") {
					if strings.Contains(row, selected.Title) {
						found = true
					}
					if ansi.StringWidth(row) > size.paneWidth {
						t.Fatal("sidebar overflow", size, icons)
					}
				}
				if !found {
					t.Fatal("decoration hid selected row", size, icons, key, selected, pane)
				}
			}
		}
	}
	// Half-page landing on the caption advances to a real item.
	m := resize(New(sectionFixture()), 30, 12).Select(Podcasts, "all")
	m = press(m, "ctrl+d")
	if item, _ := m.Selected(Podcasts); item.ID != "show-0" {
		t.Fatal("half-page stopped on heading", item)
	}
}

func TestSectionsFollowFiltersWithoutAppearingInSearchOrDiscovery(t *testing.T) {
	m := resize(New(sectionFixture()), 140, 40)
	m = press(m, "/")
	next, _ := m.Update(tea.PasteMsg{Content: "Show"})
	m = press(next.(Model), "enter", "G", "pgup", "pgup")
	if item, _ := m.Selected(Podcasts); item.ID != "show-0" {
		t.Fatal("page up could not pass a leading filtered caption", item)
	}
	rows := m.renderList(Podcasts, m.items(Podcasts), 56, 16)
	if !strings.Contains(ansi.Strip(rows[0]), "Subscriptions") {
		t.Fatal("filtered subscriptions lost section identity")
	}
	m = press(m, "esc", "/")
	next, _ = m.Update(tea.PasteMsg{Content: "Subscriptions"})
	m = press(next.(Model), "enter")
	if len(m.Visible(Podcasts)) != 0 || strings.Contains(ansi.Strip(m.pane(Podcasts, 56, 19)), "  Subscriptions ─") {
		t.Fatal("section heading is a filter result")
	}
	for _, data := range []Data{
		{PodcastLabel: "Discover", Podcasts: []Item{{ID: "remote", Title: "Remote show"}}},
		{PodcastLabel: "Library search", Podcasts: []Item{{ID: "local", Title: "Matching show"}}},
		{Podcasts: sectionFixture().Podcasts[:3]},
		{},
	} {
		m := resize(New(data), 140, 40)
		if strings.Contains(ansi.Strip(m.pane(Podcasts, 56, 19)), "  Subscriptions ─") {
			t.Fatal("empty subscription section or caption in results")
		}
	}
}

func TestSectionRefreshPreservesItemIdentity(t *testing.T) {
	m := resize(New(sectionFixture()), 140, 40).Select(Podcasts, "show-4")
	data := sectionFixture()
	data.Podcasts = slices.Insert(data.Podcasts, 3, Item{ID: "new-show", Title: "A new show"})
	m = m.WithLibrary(data)
	if item, _ := m.Selected(Podcasts); item.ID != "show-4" {
		t.Fatal("refresh selected a heading or another show", item)
	}
}
