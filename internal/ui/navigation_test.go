package ui

import (
	"fmt"
	"testing"
)

func TestListPagingMatchesVisibleRowsAndClamps(t *testing.T) {
	var items []Item
	for i := range 100 {
		items = append(items, Item{ID: fmt.Sprint(i), Title: fmt.Sprintf("Episode %d", i)})
	}
	for _, layout := range []struct {
		width, height int
		pages         [3]int
	}{
		{140, 44, [3]int{40, 17, 34}},
		{70, 20, [3]int{11, 10, 10}},
		{30, 12, [3]int{3, 2, 2}},
	} {
		for panel := Podcasts; panel <= Queue; panel++ {
			m := resize(New(Data{Podcasts: items, Episodes: items, Queue: items}), layout.width, layout.height).WithFocus(panel)
			page := layout.pages[panel]
			m = press(m, "ctrl+d")
			if m.selected[panel] != max(1, page/2) {
				t.Fatalf("%dx%d panel=%v half-page=%d", layout.width, layout.height, panel, m.selected[panel])
			}
			m = press(m, "ctrl+u", "pgdown")
			if m.selected[panel] != page {
				t.Fatalf("panel=%v page=%d want=%d", panel, m.selected[panel], page)
			}
			m = press(m, "pgup")
			if m.selected[panel] != 0 {
				t.Fatal("page up did not restore selection")
			}
			m = press(m, "G", "ctrl+d", "pgdown")
			if m.selected[panel] != len(items)-1 {
				t.Fatal("paging exceeded end")
			}
			m = press(m, "g", "ctrl+u", "pgup")
			if m.selected[panel] != 0 {
				t.Fatal("paging exceeded start")
			}
			m = press(m, "?", "ctrl+d", "pgdown", "esc")
			if m.selected[panel] != 0 {
				t.Fatal("paging escaped help")
			}
		}
	}
	empty := press(New(Data{}), "ctrl+d", "pgdown", "ctrl+u", "pgup")
	if empty.selected[Podcasts] != 0 {
		t.Fatal("empty paging produced invalid selection")
	}
}

func TestTabCyclesListsAndBackLeavesSupportingPanes(t *testing.T) {
	for _, size := range [][2]int{{140, 44}, {70, 20}, {30, 12}} {
		m := resize(New(fixture()), size[0], size[1])
		for _, want := range []Panel{Episodes, Queue, Podcasts} {
			m = press(m, "tab")
			if m.Focus() != want {
				t.Fatal("Tab traversed supporting pane", m.Focus(), want)
			}
		}
		for _, want := range []Panel{Queue, Episodes, Podcasts} {
			m = press(m, "shift+tab")
			if m.Focus() != want {
				t.Fatal("reverse Tab traversed supporting pane", m.Focus(), want)
			}
		}
		for _, key := range []string{"h", "left", "esc"} {
			m = m.WithFocus(Episodes)
			for _, panel := range []Panel{Queue, Details, PlayerPanel} {
				m = m.WithFocus(panel)
				m = press(m, key)
				// Player Left retains explicit transport-button selection.
				if panel == PlayerPanel && key == "left" {
					if m.Focus() != PlayerPanel {
						t.Fatal("Player Left lost transport focus")
					}
					m = press(m, "esc")
				}
				if m.Focus() != Episodes {
					t.Fatal("back lost browsing origin", key, panel, m.Focus())
				}
			}
			m = press(m, key)
			if m.Focus() != Podcasts {
				t.Fatal("episode back did not reach shows", key)
			}
			m = press(m, key)
			if m.Focus() != Podcasts {
				t.Fatal("root back wrapped")
			}
		}
		m = m.WithFocus(Episodes)
		m = press(m, "/", "F", "enter", "esc")
		if m.Focus() != Episodes || m.filters[Episodes] != "" {
			t.Fatal("Esc did not clear applied filter before leaving")
		}
		m = press(m, "esc")
		if m.Focus() != Podcasts {
			t.Fatal("second Esc did not go back")
		}
		for _, panel := range []Panel{Details, PlayerPanel} {
			m = press(m.WithFocus(panel), "tab")
			if m.Focus() != Podcasts {
				t.Fatal("Tab failed to return to lists")
			}
			m = press(m.WithFocus(panel), "shift+tab")
			if m.Focus() != Queue {
				t.Fatal("reverse Tab failed to return to lists")
			}
		}
	}
}
