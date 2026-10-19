package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/discovery"
	"podmutt/internal/ui"
)

func TestExplicitIndexScopePreviewAndSharedFeedIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rss" {
			t.Error("unexpected audio request")
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<rss><channel><title>Shared show</title><item><guid>shared</guid><title>Shared episode</title><enclosure url="/audio.mp3"/></item></channel></rss>`)
	}))
	defer server.Close()
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		shows := []discovery.Show{{Title: "Shared show", FeedURL: server.URL + "/rss", SourceURL: "https://podcastindex.org/podcast/123"}}
		apple := &fixtureDirectory{shows: shows}
		index := &fixtureDirectory{shows: shows}
		m.browse.directory = apple
		m.browse.indexDirectory = index
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
		if !strings.Contains(m.View().Content, "Podcast Index") {
			t.Fatal("scope not labelled")
		}
		updateLibrary(&m, tea.PasteMsg{Content: "shared"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if index.calls != 1 || apple.calls != 0 || !m.browse.usingIndex {
			t.Fatal("wrong provider or hidden fallback")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.preview == nil || m.Model.Focus() != ui.Episodes {
			t.Fatal("Index preview failed")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		first := syncPlayer(t, &m).Current.ID
		sources, _ := s.store.Feeds(s.ctx, true)
		if len(sources) != 1 {
			t.Fatal("Index ad-hoc play subscribed")
		}
		m.browse.usingIndex = false
		finishLibrary(&m, m.searchDirectory("shared"))
		m.Model = m.Model.WithFocus(ui.Podcasts)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if syncPlayer(t, &m).Current.ID != first {
			t.Fatal("provider changed episode identity")
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		sources, _ = s.store.Feeds(s.ctx, true)
		if len(sources) != 2 {
			t.Fatal("explicit subscription failed")
		}
	})
}
func TestIndexMissingCredentialsNoFallbackEmptyQueryAndStaleScope(t *testing.T) {
	t.Setenv(discovery.IndexKeyEnv, "")
	t.Setenv(discovery.IndexSecretEnv, "")
	librarySession(t, func(s *session) {
		apple := &fixtureDirectory{}
		m := browserLibrary(t, s, apple)
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
		if !m.browse.searchIndex {
			t.Fatal("Index not selectable")
		}
		if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || !m.browse.searching {
			t.Fatal("empty Index query fell back to chart")
		}
		updateLibrary(&m, tea.PasteMsg{Content: "science"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if apple.calls != 0 || !strings.Contains(m.notice, "PODMUTT_PODCAST_INDEX_KEY") {
			t.Fatal("missing credentials hidden", m.notice)
		}
		m.browse.usingIndex = false
		old := m.searchDirectory("old")
		m.browse.usingIndex = true
		m.browse.indexDirectory = &fixtureDirectory{shows: []discovery.Show{{Title: "New Index hit", FeedURL: "https://example.test/rss"}}}
		finishLibrary(&m, m.searchDirectory("new"))
		finishLibrary(&m, old)
		if !m.browse.usingIndex || m.browse.shows[0].Title != "New Index hit" {
			t.Fatal("stale provider overwrote current")
		}
		finishLibrary(&m, libraryKey(&m, 'b'))
		if len(m.browse.shows) != 0 || m.browse.loaded {
			t.Fatal("Index results cached after leaving provider")
		}
		finishLibrary(&m, libraryKey(&m, 'b'))
		if !m.browse.usingIndex || len(m.browse.shows) != 1 {
			t.Fatal("Index did not refetch on return")
		}
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
		if m.browse.searchApple {
			t.Fatal("Index did not cycle to offline library")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		finishLibrary(&m, libraryKey(&m, 't'))
		if m.browse.usingIndex {
			t.Fatal("chart silently used Index")
		}
		m.browse.usingIndex = true
		m.browse.indexDirectory = nil
		finishLibrary(&m, m.searchDirectory("missing"))
		if !strings.Contains(m.notice, "requires your own") {
			t.Fatal("nil provider not handled")
		}
	})
}
