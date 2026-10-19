package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

type fixtureDirectory struct {
	shows []discovery.Show
	calls int
	query string
	err   error
}

func (d *fixtureDirectory) Top(ctx context.Context) ([]discovery.Show, error) {
	return d.Search(ctx, "")
}
func (d *fixtureDirectory) Search(ctx context.Context, q string) ([]discovery.Show, error) {
	d.calls++
	d.query = q
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return d.shows, d.err
}
func browserLibrary(t *testing.T, s *session, d showDirectory) libraryShell {
	t.Helper()
	m, err := newLibrary(s, feed.New(s.store, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	m.browse.directory = d
	updateLibrary(&m, tea.WindowSizeMsg{Width: 160, Height: 36})
	return m
}
func TestDiscoverAutomaticListReadOnlyPreviewAndSubscribe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/audio.mp3" {
			t.Error("preview fetched audio")
			return
		}
		fmt.Fprint(w, `<rss><channel><title>Science Hour</title><item><title>Episode preview</title><description>Inspect before subscribing</description><enclosure url="/audio.mp3" type="audio/mpeg"/></item></channel></rss>`)
	}))
	defer server.Close()
	librarySession(t, func(s *session) {
		d := &fixtureDirectory{shows: []discovery.Show{{Title: "Science Hour", FeedURL: server.URL + "/rss", SourceURL: "https://podcasts.apple.com/show"}, {Title: "Other", FeedURL: server.URL + "/other"}}}
		m := browserLibrary(t, s, d)
		if !m.browse.active || !strings.Contains(m.View().Content, "Discover") {
			t.Fatal("empty profile doesn't discover")
		}
		finishLibrary(&m, updateLibrary(&m, discoverStartup{}))
		if d.calls != 1 || d.query != "" || len(m.Model.Visible(ui.Podcasts)) != 2 {
			t.Fatal("initial chart not loaded")
		}
		if cmd := libraryKey(&m, 'u'); cmd != nil || m.mutating {
			t.Fatal("subscribed without preview")
		}
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "Science"})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if len(m.Model.Visible(ui.Podcasts)) != 1 {
			t.Fatal("local discovery filter failed")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.preview == nil || m.Model.Focus() != ui.Episodes || len(m.Model.Visible(ui.Episodes)) != 1 {
			t.Fatal("preview not shown")
		}
		feeds, err := s.store.Feeds(s.ctx, false)
		if err != nil || len(feeds) != 0 {
			t.Fatal("preview persisted a subscription", err)
		}
		if cmd := libraryKey(&m, 'c'); cmd != nil {
			t.Fatal("preview completion mutated history")
		}
		// Inspecting remains read-only. Explicit Enter/a/A are covered separately.
		finishLibrary(&m, libraryKey(&m, 'i'))
		if !strings.Contains(m.View().Content, "Inspect before subscribing") || !strings.Contains(m.View().Content, "https://podcasts.apple.com/show") {
			t.Fatal("preview details/source hidden")
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		if m.browse.active || m.mutating || len(m.snapshot.Feeds) != 1 || !m.snapshot.Feeds[0].Subscribed || len(m.rows) != 1 || m.rows[0].ID <= 0 || m.Model.Focus() != ui.Episodes {
			t.Fatal("subscription did not open stored episodes")
		}
		state, err := s.playback.Snapshot(s.ctx)
		if err != nil || state.Status != "stopped" || state.Current != nil || len(state.Queue) != 1 {
			t.Fatal("subscription did not enqueue latest while remaining stopped", state, err)
		}
		finishLibrary(&m, libraryKey(&m, 'b'))
		if !strings.Contains(m.View().Content, "[+] Science Hour") {
			t.Fatal("subscribed marker missing from directory")
		}
		// Reopening an existing preview/resubscription must not duplicate it.
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		finishLibrary(&m, libraryKey(&m, 'u'))
		feeds, err = s.store.Feeds(s.ctx, false)
		if err != nil || len(feeds) != 1 {
			t.Fatal(feeds, err)
		}
	})
}
func TestLibrarySearchShowsAndCompletedEpisodesWithExplicitScope(t *testing.T) {
	librarySession(t, func(s *session) {
		id, _ := seededLibrary(t, s)
		episodes, err := s.store.Episodes(s.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, episode := range episodes {
			if episode.Title == "Alpha" {
				if err := s.store.SetCompleted(s.ctx, episode.ID, true); err != nil {
					t.Fatal(err)
				}
			}
		}
		d := &fixtureDirectory{shows: []discovery.Show{{Title: "Directory hit", FeedURL: "https://example.test/directory"}}}
		m := browserLibrary(t, s, d)
		if m.browse.active || len(m.Model.Visible(ui.Podcasts)) != len(libraryDestinations)+1 {
			t.Fatal("library destinations/subscriptions missing")
		}
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if d.calls != 0 || !m.browse.localResults || len(m.Model.Visible(ui.Podcasts)) != 1 || len(m.rows) != 1 || !m.rows[0].Completed {
			t.Fatal("library search omitted completed history or used network")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.browse.localResults {
			t.Fatal("search not cleared")
		}
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: "Hello"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if len(m.Model.Visible(ui.Podcasts)) != 0 || len(m.rows) != 1 || m.rows[0].Title != "Zulu" {
			t.Fatal("episode description search failed")
		}
		// Clear restores all shows, and selecting a show leaves search results.
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.localResults || m.libraryFeedID() != id || len(m.rows) != 2 {
			t.Fatal("show selection didn't open its episodes")
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
		updateLibrary(&m, tea.PasteMsg{Content: "science"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if !m.browse.active || d.query != "science" || d.calls != 1 {
			t.Fatal("Apple search scope not explicit")
		}
	})
}
func TestSearchInputLiteralBoundedCancelAndEmptyReset(t *testing.T) {
	librarySession(t, func(s *session) {
		seededLibrary(t, s)
		d := &fixtureDirectory{}
		m := browserLibrary(t, s, d)
		libraryKey(&m, 'f')
		libraryKey(&m, 'q')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		libraryKey(&m, 's')
		if m.browse.input != "q s" || d.calls != 0 {
			t.Fatal("search interpreted transport/quit")
		}
		updateLibrary(&m, tea.PasteMsg{Content: strings.Repeat("界", 200)})
		if len([]rune(m.browse.input)) != 128 {
			t.Fatal("unbounded query")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyBackspace})
		if len([]rune(m.browse.input)) != 127 {
			t.Fatal("unicode backspace")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		if m.browse.input != "" {
			t.Fatal("clear failed")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.browse.searching || m.Model.Busy() {
			t.Fatal("search cancel stuck")
		}
		libraryKey(&m, 'f')
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.localResults {
			t.Fatal("empty local query did not reset")
		}
		libraryKey(&m, 'f')
		if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
			t.Fatal("Ctrl+C swallowed")
		}
	})
}
func TestDirectoryStaleRepliesSelectionAndPreviewCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<rss><channel><title>Preview</title></channel></rss>`)
	}))
	defer server.Close()
	librarySession(t, func(s *session) {
		d := &fixtureDirectory{shows: []discovery.Show{{Title: "One", FeedURL: server.URL + "/one"}, {Title: "Two", FeedURL: server.URL + "/two"}}}
		m := browserLibrary(t, s, d)
		old := m.searchDirectory("old")
		fresh := m.searchDirectory("new")
		finishLibrary(&m, fresh)
		before := m.notice
		finishLibrary(&m, old)
		if m.browse.query != "new" || m.notice != before {
			t.Fatal("stale directory result applied")
		}
		preview := m.previewShow()
		libraryKey(&m, 'j')
		finishLibrary(&m, preview)
		if m.browse.preview != nil {
			t.Fatal("stale preview applied")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.preview == nil {
			t.Fatal("current preview absent")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.browse.preview != nil {
			t.Fatal("preview cancel failed")
		}
		old = m.searchDirectory("late")
		libraryKey(&m, 'b')
		finishLibrary(&m, old)
		if m.browse.active {
			t.Fatal("late reply changed mode")
		}
		d.err = errors.New("offline")
		finishLibrary(&m, libraryKey(&m, 'b'))
		finishLibrary(&m, libraryKey(&m, 'R'))
		if !strings.Contains(m.notice, "offline") {
			t.Fatal("directory failure hidden", m.notice)
		}
		// A closed result or malformed scope cannot overwrite the current view.
		updateLibrary(&m, directoryLoaded{generation: m.browse.generation, local: true, err: errors.New("wrong scope")})
		if strings.Contains(m.notice, "wrong scope") {
			t.Fatal("scope guard absent")
		}
	})
}

type blockingDirectory struct {
	started chan struct{}
	stopped chan struct{}
}

func (d blockingDirectory) Top(ctx context.Context) ([]discovery.Show, error) {
	close(d.started)
	<-ctx.Done()
	close(d.stopped)
	return nil, ctx.Err()
}
func (d blockingDirectory) Search(ctx context.Context, _ string) ([]discovery.Show, error) {
	return d.Top(ctx)
}
func TestLeavingDiscoverCancelsWorkWithoutBlockingLibrary(t *testing.T) {
	librarySession(t, func(s *session) {
		d := blockingDirectory{started: make(chan struct{}), stopped: make(chan struct{})}
		m := browserLibrary(t, s, d)
		cmd := updateLibrary(&m, discoverStartup{})
		messages := make(chan tea.Msg, 1)
		go func() { messages <- cmd() }()
		select {
		case <-d.started:
		case <-time.After(time.Second):
			t.Fatal("directory not started")
		}
		libraryKey(&m, 'b')
		if m.browse.active || !strings.Contains(m.View().Content, "My library") {
			t.Fatal("online request blocked local view")
		}
		select {
		case <-d.stopped:
		case <-time.After(time.Second):
			t.Fatal("request not canceled")
		}
		updateLibrary(&m, <-messages)
		if m.browse.active {
			t.Fatal("canceled result replaced library")
		}
	})
}
func TestPreviewFailureDoesNotSubscribeAndLibraryReloadRefreshesSearch(t *testing.T) {
	librarySession(t, func(s *session) {
		id, _ := seededLibrary(t, s)
		d := &fixtureDirectory{shows: []discovery.Show{{Title: "Bad", FeedURL: "file:///secret"}}}
		m := browserLibrary(t, s, d)
		finishLibrary(&m, libraryKey(&m, 'b'))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.browse.preview != nil || m.mutating {
			t.Fatal("failed preview subscribed")
		}
		libraryKey(&m, 'b')
		finishLibrary(&m, m.searchLibrary("new match"))
		if err := s.store.UpsertEpisodes(s.ctx, id, []store.Episode{{GUID: "new", Title: "new match", AudioURL: "https://example.test/new"}}); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, m.reload())
		if len(m.rows) != 1 || m.rows[0].Title != "new match" {
			t.Fatal("refresh didn't update cached search")
		}
	})
}
