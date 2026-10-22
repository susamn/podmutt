package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/library"
	"podmutt/internal/playback"
	"podmutt/internal/profile"
	"podmutt/internal/store"
	"podmutt/internal/theme"
	"podmutt/internal/ui"
)

func librarySession(t *testing.T, run func(*session)) {
	t.Helper()
	paths := isolatedProfile(t)
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	_ = paths.SetOnboarded()
	if err := withSession(profile.Defaults(), paths, func(s *session) error { run(s); return nil }); err != nil {
		t.Fatal(err)
	}
}
func seededLibrary(t *testing.T, s *session) (int64, int64) {
	t.Helper()
	a, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: "https://example.test/a", Title: "Alpha show", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: "https://example.test/b", Title: "Beta show"})
	if err != nil {
		t.Fatal(err)
	}
	for id, items := range map[int64][]store.Episode{
		a: {{GUID: "a", Title: "Zulu", AudioURL: "https://example.test/z", Description: "<p>Hello</p><script>hidden</script>", PublishedAt: time.Unix(100, 0), DurationSeconds: 4500},
			{GUID: "b", Title: "Alpha", AudioURL: "https://example.test/x", PublishedAt: time.Unix(200, 0)}},
		b: {{GUID: "c", Title: "Cached beta", AudioURL: "https://example.test/y"}},
	} {
		if err := s.store.UpsertEpisodes(s.ctx, id, items); err != nil {
			t.Fatal(err)
		}
	}
	return a, b
}
func updateLibrary(m *libraryShell, msg tea.Msg) tea.Cmd {
	next, cmd := m.Update(msg)
	*m = next.(libraryShell)
	return cmd
}
func libraryKey(m *libraryShell, key rune) tea.Cmd {
	return updateLibrary(m, tea.KeyPressMsg{Code: key, Text: string(key)})
}

// Apply one user action and its optional cached-read continuation, not a live subscription.
func finishLibrary(m *libraryShell, cmd tea.Cmd) {
	if cmd != nil {
		if next := updateLibrary(m, cmd()); next != nil {
			updateLibrary(m, next())
		}
	}
}

func openLibraryDestination(m *libraryShell, id string) {
	m.Model = m.Model.WithFocus(ui.Podcasts).Select(ui.Podcasts, id)
	finishLibrary(m, updateLibrary(m, tea.KeyPressMsg{Code: tea.KeyEnter}))
}

func TestCachedDestinationsSortingSelectionAndCompletion(t *testing.T) {
	librarySession(t, func(s *session) {
		a, b := seededLibrary(t, s)
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 32})
		if len(m.rows) != 2 || m.libraryID != allDestination || !strings.Contains(m.View().Content, "All episodes") {
			t.Fatal("cached content missing")
		}
		selected, _ := m.Model.Selected(ui.Episodes)
		finishLibrary(&m, libraryKey(&m, 'o'))
		if current, _ := m.Model.Selected(ui.Episodes); current.ID != selected.ID {
			t.Fatal("sort lost selection identity")
		}
		finishLibrary(&m, libraryKey(&m, 'o'))
		openLibraryDestination(&m, strconv.FormatInt(a, 10))
		if m.libraryView() != library.ShowView || len(m.rows) != 2 {
			t.Fatal("show tab not loaded")
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		finishLibrary(&m, libraryKey(&m, 'c'))
		if selected, ok := m.selectedEpisode(); !ok || !selected.Completed {
			t.Fatal("completion not shown")
		}
		openLibraryDestination(&m, allDestination)
		if len(m.rows) != 2 {
			t.Fatal("All omitted completed episode")
		}
		openLibraryDestination(&m, strconv.FormatInt(a, 10))
		if len(m.Model.Visible(ui.Podcasts)) != len(libraryDestinations)+1 {
			t.Fatal("unsubscribed cache leaked into My subscriptions")
		}
		cached, err := s.store.Episodes(s.ctx, b)
		if err != nil || len(cached) != 1 {
			t.Fatal("unsubscribed history was deleted")
		}
		if err := m.service.Subscribe(s.ctx, b, true); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, m.reload())
		openLibraryDestination(&m, strconv.FormatInt(b, 10))
		if m.libraryFeedID() != b || len(m.rows) != 1 || m.rows[0].Title != "Cached beta" {
			t.Fatal("resubscribed show cache unavailable")
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		if m.snapshot.Feeds[1].Subscribed {
			t.Fatal("unsubscribe failed")
		}
		libraryKey(&m, 'R')
		if !strings.Contains(m.View().Content, "Refreshing") {
			t.Fatal("manual refresh invisible")
		}
	})
}

func TestRSSInputAddAndInvalidFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			fmt.Fprint(w, "<html/>")
			return
		}
		fmt.Fprint(w, `<rss><channel><title>Added show</title><item><title>Added episode</title><enclosure url="/audio.mp3"/></item></channel></rss>`)
	}))
	defer server.Close()
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, server.Client(), 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 120, Height: 24})
		libraryKey(&m, 'N')
		if !m.adding || !strings.Contains(m.View().Content, "Add feed or podcast website") {
			t.Fatal("RSS prompt missing")
		}
		updateLibrary(&m, tea.PasteMsg{Content: server.URL + "/rss"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.libraryView() != library.ShowView || len(m.rows) != 1 || m.rows[0].Title != "Added episode" || m.wantLibraryID != "" {
			t.Fatal("added feed not selected")
		}
		state, err := s.playback.Snapshot(s.ctx)
		if err != nil || state.Status != "stopped" {
			t.Fatal("add autoplayed")
		}
		libraryKey(&m, 'N')
		updateLibrary(&m, tea.PasteMsg{Content: server.URL + "/bad"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if len(m.snapshot.Feeds) != 1 || !strings.Contains(m.View().Content, "unreadable or unsupported") {
			t.Fatal("invalid add lost cache or hid error")
		}
		libraryKey(&m, 'N')
		updateLibrary(&m, tea.PasteMsg{Content: strings.Repeat("世", 5000)})
		if len([]rune(m.input)) != 4096 {
			t.Fatal("input unbounded")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyBackspace})
		if len([]rune(m.input)) != 4095 {
			t.Fatal("unicode backspace failed")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		if m.input != "" {
			t.Fatal("clear input failed")
		}
		libraryKey(&m, 'q')
		if m.input != "q" {
			t.Fatal("URL input interpreted quit")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.adding {
			t.Fatal("cancel input failed")
		}
		libraryKey(&m, 'N')
		if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
			t.Fatal("Ctrl+C did not quit prompt")
		}
	})
}

func TestRefreshErrorsStaleRepliesAndIdentity(t *testing.T) {
	librarySession(t, func(s *session) {
		a, _ := seededLibrary(t, s)
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 32})
		selected, _ := m.Model.Selected(ui.Episodes)
		if err := s.store.UpsertEpisodes(s.ctx, a, []store.Episode{{GUID: "new", Title: "Newest", AudioURL: "https://example.test/new", PublishedAt: time.Unix(300, 0)}}); err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: a, Error: errors.New("offline")}, ok: true})
		snapshot, err := m.service.Read(s.ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, libraryLoaded{version: m.version, snapshot: snapshot})
		if current, _ := m.Model.Selected(ui.Episodes); current.ID != selected.ID || len(m.rows) != 3 {
			t.Fatal("incremental refresh lost identity")
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: 99}, ok: true})
		if !strings.Contains(m.View().Content, "feeds could not be refreshed") || m.refreshErrors[a] == nil {
			t.Fatal("unrelated success cleared error")
		}
		updateLibrary(&m, libraryLoaded{version: m.version - 1, err: errors.New("stale")})
		if strings.Contains(m.View().Content, "stale") {
			t.Fatal("stale reply applied")
		}
		updateLibrary(&m, libraryLoaded{version: m.version, err: errors.New("read failed")})
		if !strings.Contains(m.View().Content, "Load library failed") {
			t.Fatal("read error hidden")
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: a}, ok: true})
		if len(m.refreshErrors) != 0 {
			t.Fatal("recovered feed retained error")
		}
		results := make(chan feed.Result, 1)
		results <- feed.Result{FeedID: a}
		close(results)
		m.results = results
		if msg := m.waitRefresh()(); !msg.(refreshUpdate).ok {
			t.Fatal("refresh subscription missing")
		}
		if cmd := updateLibrary(&m, m.waitRefresh()()); cmd != nil {
			t.Fatal("closed refresh resubscribed")
		}
		m.lastPlayingID = 1
		if cmd := updateLibrary(&m, playbackUpdate{state: playback.State{Current: &store.Episode{ID: 2}, Status: playback.Playing}, ok: true}); cmd == nil {
			t.Fatal("track transition did not update library")
		}
	})
}

func TestLibraryGuardsAndPlainDetails(t *testing.T) {
	librarySession(t, func(s *session) {
		_, b := seededLibrary(t, s)
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 32})
		m.Model = m.Model.WithFocus(ui.Episodes)
		m.Model = m.Model.Select(ui.Episodes, strconv.FormatInt(m.rows[1].ID, 10))
		m.renderLibrary()
		if view := m.View().Content; !strings.Contains(view, "Hello") || strings.Contains(view, "hidden") || !strings.Contains(view, "1:15:00") {
			t.Fatal("details not plain text or duration not formatted")
		}
		libraryKey(&m, '/')
		libraryKey(&m, 'c')
		if m.mutating {
			t.Fatal("filter dispatched completion")
		}
		updateLibrary(&m, tea.PasteMsg{Content: "literal"})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		m.mutating = true
		if libraryKey(&m, 'c') != nil || libraryKey(&m, 'u') != nil {
			t.Fatal("mutation reentered")
		}
		m.mutating = false
		updateLibrary(&m, libraryEdited{err: errors.New("edit failed")})
		if !strings.Contains(m.View().Content, "Library update failed") {
			t.Fatal("edit error hidden")
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.Model.Focus() != ui.Episodes {
			t.Fatal("Enter did not focus episodes")
		}
		m.wantLibraryID = strconv.FormatInt(b, 10)
		m.libraryID = m.wantLibraryID
		libraryKey(&m, 'j')
		if m.libraryFeedID() != b {
			t.Fatal("pending add selection was overwritten")
		}
	})
}

func TestThemeReloadAndSignal(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		newPal := theme.Catppuccin()
		updateLibrary(&m, themeReloadMsg{palette: newPal})
		if m.Model.Theme().Accent != newPal.Accent {
			t.Fatalf("theme not updated: got %v, want %v", m.Model.Theme().Accent, newPal.Accent)
		}
		if m.signals != nil {
			m.signals <- os.Interrupt
			cmd := m.waitSignal()
			if cmd != nil {
				msg := cmd()
				if _, ok := msg.(themeReloadMsg); !ok {
					t.Fatalf("expected themeReloadMsg from signal, got %T", msg)
				}
			}
		}
	})
}

func TestSettingsInteractiveThemeSwitch(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'e', Text: "e"})
		if !m.Model.SettingsOpen() {
			t.Fatal("settings not open")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'j', Text: "j"})
		cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil {
			msg := cmd()
			updateLibrary(&m, msg)
		}
		if s.config.Theme == "" {
			t.Fatal("theme not updated in session")
		}
	})
}
