package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestFavoritesLayoutColumnsAndDetailsHeart(t *testing.T) {
	favItem := Item{
		ID:          "101",
		Title:       "Favorite Title",
		PlainTitle:  "Favorite Title",
		Channel:     "Favorite Show",
		Date:        "Today",
		PublishedAt: time.Now(),
	}
	data := Data{
		Favorites: []Item{favItem},
		Details:   "Select an episode",
	}

	for _, size := range [][2]int{{140, 40}, {100, 24}} {
		for _, icons := range []string{"unicode", "nerd", "plain"} {
			m := resize(New(data).WithIcons(icons), size[0], size[1])
			view := ansi.Strip(m.View().Content)

			// Wide layout: both Queue [3] and Favorites [5] and Player [6] are rendered
			if !strings.Contains(view, "[3]") || !strings.Contains(view, "[5]") || !strings.Contains(view, "[6]") {
				t.Fatalf("missing panel tags at %v: %s", size, view)
			}
			if !strings.Contains(view, "Favorite Title") {
				t.Fatalf("missing favorite row content at %v:\n%s", size, view)
			}
			if size[0] >= 140 && !strings.Contains(view, "Favorite Show") {
				t.Fatalf("missing favorite channel at %v:\n%s", size, view)
			}
			// Verify Favorites header contains Title, Channel, Date, but NOT Remains
			favPane := ansi.Strip(m.pane(Favorites, 50, 10))
			if !strings.Contains(favPane, "Title") || !strings.Contains(favPane, "Channel") || !strings.Contains(favPane, "Date") {
				t.Fatalf("Favorites header missing standard columns: %s", favPane)
			}
			if strings.Contains(favPane, "Remains") {
				t.Fatalf("Favorites header must not include Remains column: %s", favPane)
			}
		}
	}

	// Details Heart Badge
	for _, icons := range []string{"unicode", "plain", "nerd"} {
		cardFav := EpisodeDetails{
			Title:     "Episode A",
			Show:      "Show A",
			Favorited: true,
		}
		cardNotFav := EpisodeDetails{
			Title:     "Episode B",
			Show:      "Show B",
			Favorited: false,
		}

		mFav := resize(New(Data{}).WithIcons(icons).WithEpisodeDetails(cardFav), 100, 24).WithFocus(Details)
		viewFav := ansi.Strip(mFav.View().Content)
		wantBadge := "♥ Favorited"
		if icons == "plain" {
			wantBadge = "[Fav] Favorited"
		}
		if !strings.Contains(viewFav, wantBadge) {
			t.Fatalf("Details missing favorited badge for mode %q:\n%s", icons, viewFav)
		}
		// Verify Details rows have padding from left border wall
		if !strings.Contains(viewFav, "│  Episode A") || !strings.Contains(viewFav, "│  "+wantBadge) {
			t.Fatalf("Details missing left margin padding:\n%s", viewFav)
		}

		mNotFav := resize(New(Data{}).WithIcons(icons).WithEpisodeDetails(cardNotFav), 100, 24).WithFocus(Details)
		viewNotFav := ansi.Strip(mNotFav.View().Content)
		if strings.Contains(viewNotFav, "Favorited") {
			t.Fatalf("Details showed favorited badge when not favorited:\n%s", viewNotFav)
		}
	}
}

func TestFavoritesRowFallbackTitleAndHorizontalScroll(t *testing.T) {
	m := New(Data{}).WithIcons("plain")
	item := Item{Title: "Fallback title", Channel: "My channel", Date: "Today"}
	row := ansi.Strip(m.renderFavoritesRow(item, 60, false))
	if !strings.Contains(row, "* ") || !strings.Contains(row, "Fallback title") || !strings.Contains(row, "My channel") || !strings.Contains(row, "Today") {
		t.Fatal("favorite row lost its fallback content", row)
	}
	m.colOffset[Favorites] = 9
	row = ansi.Strip(m.renderFavoritesRow(item, 60, false))
	if strings.Contains(row, "Fallback") || !strings.Contains(row, "itle") {
		t.Fatal("horizontal scroll did not advance title", row)
	}
	m.colOffset[Favorites] = 100
	row = ansi.Strip(m.renderFavoritesRow(item, 60, false))
	if strings.Contains(row, "Fallback") || strings.Contains(row, "title") || !strings.Contains(row, "My channel") {
		t.Fatal("overscroll did not retain channel", row)
	}
}

func TestFavoritesEmptyStateAndFilter(t *testing.T) {
	m := resize(New(Data{}), 140, 40).WithFocus(Favorites)
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "No favorites · m in Queue to add") || !strings.Contains(view, "│  No favorites") {
		t.Fatalf("unexpected empty state for favorites:\n%s", view)
	}

	// Filtering with no matches
	m = m.WithLibrary(Data{Favorites: []Item{{ID: "1", Title: "Unique Title"}}})
	m = press(m, "/", "z", "z", "z", "enter")
	view = ansi.Strip(m.View().Content)
	if !strings.Contains(view, "No matching favorites") {
		t.Fatalf("expected 'No matching favorites' when filtered:\n%s", view)
	}
}

func TestFavoritesCompactLayoutSelectionAndPaging(t *testing.T) {
	var items []Item
	for i := range 30 {
		items = append(items, Item{
			ID:         string(rune('A' + i)),
			Title:      "Fav " + string(rune('A'+i)),
			PlainTitle: "Fav " + string(rune('A'+i)),
		})
	}
	m := resize(New(Data{Favorites: items}), 70, 20).WithFocus(Favorites)

	// In compact layout, focusing Favorites renders Favorites as the middle pane
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "[5]") || !strings.Contains(view, "Favorites") {
		t.Fatalf("compact layout did not show Favorites pane:\n%s", view)
	}
	if !strings.Contains(view, "[6]") || !strings.Contains(view, "Now playing") {
		t.Fatalf("compact layout missing Player pane below Favorites:\n%s", view)
	}

	// Paging and navigation
	m = press(m, "j", "j")
	if m.selected[Favorites] != 2 {
		t.Fatalf("selection failed: got %d, want 2", m.selected[Favorites])
	}
	m = press(m, "ctrl+d")
	if m.selected[Favorites] <= 2 {
		t.Fatalf("ctrl+d failed to advance: %d", m.selected[Favorites])
	}
}
