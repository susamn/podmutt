package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestCurrentURLPopupUsesPlaybackNotSelectionAndKeepsKeys(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		m.snapshot.Feeds = []store.Feed{{ID: 1, URL: "https://example.test/rss", ArtworkURL: "https://example.test/feed-cover"}}
		m.state.Current = &store.Episode{ID: 10, FeedID: 1, Title: "Playing", AudioURL: "https://example.test/audio", ArtworkURL: "https://example.test/episode-cover", ChaptersURL: "https://example.test/chapters"}
		m.Model = m.Model.WithLibrary(ui.Data{Episodes: []ui.Item{{ID: "20", Title: "Selected elsewhere"}}}).WithFocus(ui.Episodes)
		for _, size := range [][2]int{{140, 40}, {48, 18}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			if cmd := libraryKey(&m, 'I'); cmd != nil || m.urlDialog == nil || !m.Model.Busy() {
				t.Fatal("URL popup not opened", cmd)
			}
			body := m.urlDialog.Body
			for _, want := range []string{"Feed", "RSS: https://example.test/rss", "Artwork: https://example.test/feed-cover", "Episode · Playing", "Audio: https://example.test/audio", "Artwork: https://example.test/episode-cover", "Chapters: https://example.test/chapters"} {
				if !strings.Contains(body, want) {
					t.Fatal("missing stored URL", want, body)
				}
			}
			if strings.Contains(body, "Selected elsewhere") {
				t.Fatal("selection leaked into playback popup", body)
			}
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "Current playback · URLs") || !strings.Contains(view, "Esc close") {
				t.Fatal("shared modal did not render", view)
			}
			focus := m.Model.Focus()
			libraryKey(&m, 'j')
			if m.urlDialog.Scroll != 1 || m.Model.Focus() != focus {
				t.Fatal("modal let navigation escape")
			}
			libraryKey(&m, 'k')
			if m.urlDialog.Scroll != 0 {
				t.Fatal("modal scroll did not reset")
			}
			libraryKey(&m, 'a')
			if m.urlDialog == nil || m.Model.Focus() != focus {
				t.Fatal("modal let enqueue escape")
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
			if m.urlDialog != nil || m.Model.Busy() {
				t.Fatal("Esc did not close popup")
			}
		}
	})
}

func TestCurrentURLPopupMissingPlaybackAndOptionalURLs(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		libraryKey(&m, 'I')
		if !strings.Contains(m.urlDialog.Body, "Nothing playing") {
			t.Fatal(m.urlDialog.Body)
		}
		libraryKey(&m, 'I')
		if m.urlDialog != nil {
			t.Fatal("I did not toggle popup")
		}
		m.Model = m.Model.WithDialog(&ui.Dialog{Title: "Other action"})
		libraryKey(&m, 'I')
		if m.urlDialog != nil {
			t.Fatal("URL shortcut stole another dialog's keys")
		}
		m.Model = m.Model.WithDialog(nil)
		m.browse.searching = true
		libraryKey(&m, 'I')
		if m.urlDialog != nil {
			t.Fatal("URL shortcut stole search input")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
		m.state.Current = &store.Episode{Title: "Sparse", AudioURL: "https://example.test/audio"}
		libraryKey(&m, 'I')
		if strings.Contains(m.urlDialog.Body, "RSS:") || strings.Contains(m.urlDialog.Body, "Chapters:") || !strings.Contains(m.urlDialog.Body, "Audio: https://example.test/audio") {
			t.Fatal("optional URLs were not omitted", m.urlDialog.Body)
		}
		libraryKey(&m, 'I')
		m.state.Current = &store.Episode{Title: "Empty"}
		libraryKey(&m, 'I')
		if !strings.Contains(m.urlDialog.Body, "No stored URLs") {
			t.Fatal(m.urlDialog.Body)
		}
	})
}
