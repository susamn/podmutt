package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/ui"
)

func TestExplicitPreviewPlayAndEnqueueDoNotSubscribe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rss" {
			t.Error("cache/preview fetched audio", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<rss><channel><title>Adhoc show</title><item><guid>a</guid><title>First</title><enclosure url="/a.mp3"/></item><item><guid>b</guid><title>Second</title><enclosure url="/b.mp3"/></item><item><guid>c</guid><title>Not queued</title><enclosure url="/c.mp3"/></item></channel></rss>`)
	}))
	defer server.Close()
	librarySession(t, func(s *session) {
		if err := s.playback.Close(s.ctx); err != nil {
			t.Fatal(err)
		}
		p := &tuiPlayer{events: make(chan player.Event, 16)}
		engine, err := playback.New(s.ctx, s.store, func(context.Context) (playback.Player, error) { return p, nil })
		if err != nil {
			t.Fatal(err)
		}
		defer engine.Close(context.Background())
		s.playback = engine
		m := browserLibrary(t, s, &fixtureDirectory{shows: []discovery.Show{{Title: "Adhoc show", FeedURL: server.URL + "/rss"}}})
		finishLibrary(&m, updateLibrary(&m, discoverStartup{}))
		finishLibrary(&m, libraryKey(&m, 'l'))
		sources, err := s.store.Feeds(s.ctx, false)
		if err != nil || len(sources) != 0 {
			t.Fatal("browsing wrote cache", sources, err)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if state := syncPlayer(t, &m); state.Current != nil {
			t.Fatal("Space cached/played preview without current", state)
		}
		if sources, err := s.store.Feeds(s.ctx, false); err != nil || len(sources) != 0 {
			t.Fatal("Space wrote preview cache", sources, err)
		}
		libraryKey(&m, 'l')
		libraryKey(&m, 'h')
		if m.browse.preview == nil || m.Model.Focus() != ui.Episodes {
			t.Fatal("details return discarded preview")
		}
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "First"})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.browse.preview == nil || m.Model.HasFilter(ui.Episodes) || m.Model.Focus() != ui.Episodes {
			t.Fatal("Esc exited preview before clearing its filter")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		state := syncPlayer(t, &m)
		if state.Status != playback.Playing || state.Current == nil || state.Current.ID <= 0 || !m.browse.active {
			t.Fatal("ad-hoc playback blocked", state, m.notice)
		}
		id := state.Current.ID
		m.Model = m.Model.WithFocus(ui.Queue)
		libraryKey(&m, 'l')
		libraryKey(&m, 'h')
		if m.Model.Focus() != ui.Queue || m.browse.preview == nil {
			t.Fatal("queue details lost preview or origin")
		}
		libraryKey(&m, 'h')
		if m.Model.Focus() != ui.Episodes || m.browse.preview == nil {
			t.Fatal("leaving queue closed unrelated preview")
		}
		sources, err = s.store.Feeds(s.ctx, true)
		if err != nil || len(sources) != 0 {
			t.Fatal("ad-hoc subscribed", sources, err)
		}
		if err := engine.Stop(s.ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.store.Checkpoint(s.ctx, id, 72); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		state = syncPlayer(t, &m)
		if state.Current.ID != id || state.Current.PositionSeconds != 72 {
			t.Fatal("ad-hoc replay lost identity/resume", state)
		}
		finishLibrary(&m, libraryKey(&m, 'a'))
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "Second"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if len(m.Model.Visible(ui.Episodes)) != 1 {
			t.Fatal("preview filter failed")
		}
		finishLibrary(&m, libraryKey(&m, 'A'))
		allSources, err := s.store.Feeds(s.ctx, false)
		if err != nil || len(allSources) != 1 {
			t.Fatal(err)
		}
		cached, err := s.store.Episodes(s.ctx, allSources[0].ID)
		if err != nil || len(cached) != 2 {
			t.Fatal("filtered A cached unselected episodes", cached, err)
		}
		state = syncPlayer(t, &m)
		if len(state.Queue) == 0 {
			t.Fatal("explicit preview enqueue blocked")
		}
		sources, err = s.store.Feeds(s.ctx, true)
		if err != nil || len(sources) != 0 {
			t.Fatal("enqueue subscribed", sources, err)
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		sources, err = s.store.Feeds(s.ctx, true)
		if err != nil || len(sources) != 1 {
			t.Fatal("explicit subscription failed", sources, err)
		}
		episodes, err := s.store.Episodes(s.ctx, sources[0].ID)
		if err != nil || len(episodes) != 3 {
			t.Fatal("subscription missed cache", episodes, err)
		}
		history, err := s.store.Playback(s.ctx, id)
		if err != nil || history.PositionSeconds != 72 {
			t.Fatal("subscribe lost progress", history, err)
		}
	})
}

func TestPreviewActionSelectionGuards(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		if m.previewPlayback(false, false) != nil {
			t.Fatal("missing preview accepted")
		}
		m.browse.preview = &feed.Resolved{}
		if m.previewPlayback(false, false) != nil || m.previewPlayback(true, true) != nil {
			t.Fatal("empty selection accepted")
		}
		m.Model = ui.New(ui.Data{Episodes: []ui.Item{{ID: "bad", Title: "bad"}, {ID: "preview:x", Title: "bad index"}}}).WithFocus(ui.Episodes)
		if m.previewPlayback(true, true) != nil {
			t.Fatal("invalid preview identities accepted")
		}
		m.Model = ui.New(ui.Data{Episodes: []ui.Item{{ID: "preview:9", Title: "stale"}}}).WithFocus(ui.Episodes)
		cmd := m.previewPlayback(false, false)
		if cmd == nil {
			t.Fatal("no validation command")
		}
		if msg := cmd().(playerEdited); msg.err == nil {
			t.Fatal("stale preview played")
		}
	})
}
