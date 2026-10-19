package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/discovery"
	"podmutt/internal/ui"
)

func TestVisibleRSSWebsitePreviewIsReadOnlyUntilExplicitPlayOrSubscribe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/site":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><link rel="alternate" type="application/rss+xml" href="/rss"></html>`)
		case "/rss":
			fmt.Fprint(w, `<rss><channel><title>URL discovery</title><item><guid>a</guid><title>URL episode</title><enclosure url="/audio.mp3"/></item></channel></rss>`)
		default:
			t.Error("preview fetched audio", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "f:search") || !strings.Contains(view, "U:url") {
			t.Fatal("entry points hidden")
		}
		libraryKey(&m, 'U')
		if !m.browse.urlInput || !m.Model.Busy() {
			t.Fatal("URL editor did not open")
		}
		updateLibrary(&m, tea.PasteMsg{Content: server.URL + "/site"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.preview == nil || m.browse.preview.URL != server.URL+"/rss" || m.Model.Focus() != ui.Episodes {
			t.Fatal("website preview failed", m.notice)
		}
		sources, _ := s.store.Feeds(s.ctx, false)
		state := syncPlayer(t, &m)
		if len(sources) != 2 || state.Current != nil || len(state.Queue) != 0 {
			t.Fatal("preview mutated library or playback")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		state = syncPlayer(t, &m)
		sources, _ = s.store.Feeds(s.ctx, true)
		if state.Current == nil || state.Current.Title != "URL episode" || len(sources) != 1 {
			t.Fatal("URL ad-hoc playback subscribed or failed", state)
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		sources, _ = s.store.Feeds(s.ctx, true)
		if len(sources) != 2 {
			t.Fatal("explicit URL subscription failed", m.notice)
		}
	})
}

func TestURLInputBoundsValidationAndCancellation(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		libraryKey(&m, 'U')
		updateLibrary(&m, tea.PasteMsg{Content: strings.Repeat("a", 5000)})
		if len(m.browse.input) != 4096 {
			t.Fatal("URL input unbounded")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
		if !m.browse.urlInput {
			t.Fatal("URL editor silently changed scope")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		updateLibrary(&m, tea.PasteMsg{Content: "https://user:secret@example.test/rss"})
		if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || !m.browse.searching || strings.Contains(m.notice, "secret") {
			t.Fatal("invalid URL dispatched or leaked")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.Model.Busy() || m.browse.searching {
			t.Fatal("URL cancel stuck")
		}
		d := &fixtureDirectory{shows: []discovery.Show{{Title: "Canceled result", FeedURL: "https://example.test/rss"}}}
		m.browse.directory = d
		pending := m.searchDirectory("canceled")
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		finishLibrary(&m, pending)
		if m.browse.loading || len(m.browse.shows) != 0 {
			t.Fatal("canceled result applied")
		}
		m.browse.active = true
		m.browse.shows = d.shows
		m.Model = m.Model.Reset(ui.Podcasts)
		m.renderLibrary()
		preview := m.previewShow()
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		finishLibrary(&m, preview)
		if m.browse.preview != nil || m.browse.previewCancel != nil {
			t.Fatal("canceled preview applied")
		}
	})
}

func TestSearchAndChartHaveSeparateSourceQueryAndFailureStates(t *testing.T) {
	librarySession(t, func(s *session) {
		d := &fixtureDirectory{}
		m := browserLibrary(t, s, d)
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: "outside the chart"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if d.query != "outside the chart" || !strings.Contains(m.View().Content, "No matching shows") || !strings.Contains(m.View().Content, "Apple search") {
			t.Fatal("search source/query/empty state hidden")
		}
		d.err = errors.New("directory offline")
		pending := m.searchDirectory("retry")
		updateLibrary(&m, tea.WindowSizeMsg{Width: 30, Height: 12})
		m.Model = m.Model.WithFocus(ui.Podcasts)
		if !strings.Contains(m.View().Content, "Loading Apple") {
			t.Fatal("compact loading state hidden")
		}
		finishLibrary(&m, pending)
		if !strings.Contains(m.View().Content, "Search failed") {
			t.Fatal("compact error state hidden")
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 160, Height: 36})
		if !strings.Contains(m.View().Content, "Discovery failed") {
			t.Fatal("failed search looks like empty chart")
		}
		d.err = nil
		d.shows = []discovery.Show{{Title: "Chart result", FeedURL: "https://example.test/rss"}}
		finishLibrary(&m, libraryKey(&m, 't'))
		if m.browse.query != "" || d.query != "" || !strings.Contains(m.View().Content, "US top 25") {
			t.Fatal("chart not distinct from search")
		}
		for _, size := range [][2]int{{70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.Model = m.Model.WithFocus(ui.Episodes)
			if !strings.Contains(ansi.Strip(m.View().Content), "f:search") {
				t.Fatal("compact entry hidden", size, m.View().Content)
			}
		}
	})
}
