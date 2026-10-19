package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/control"
	"podmutt/internal/feed"
	"podmutt/internal/library"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/profile"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

type workflowPlayer struct {
	*tuiPlayer
	resume float64
}

func (p *workflowPlayer) Load(ctx context.Context, url string, position float64) error {
	p.mu.Lock()
	p.resume = position
	p.mu.Unlock()
	return p.tuiPlayer.Load(ctx, url, position)
}

type workflowOwner struct {
	s       *session
	server  *control.Server
	lock    *profile.Lock
	p       *workflowPlayer
	created atomic.Int32
}

func openWorkflow(t *testing.T, paths profile.Paths) *workflowOwner {
	t.Helper()
	lock, err := paths.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), filepath.Join(paths.State, "podcasts.db"))
	if err != nil {
		lock.Close()
		t.Fatal(err)
	}
	owner := &workflowOwner{lock: lock, p: &workflowPlayer{tuiPlayer: &tuiPlayer{events: make(chan player.Event, 16)}}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	engine, err := playback.New(ctx, db, func(context.Context) (playback.Player, error) { owner.created.Add(1); return owner.p, nil })
	if err != nil {
		t.Fatal(err)
	}
	server, err := control.Listen(ctx, paths, engine)
	if err != nil {
		t.Fatal(err)
	}
	owner.s = &session{ctx: ctx, store: db, playback: engine, paths: paths, config: profile.Defaults()}
	owner.server = server
	t.Cleanup(owner.close)
	return owner
}
func (o *workflowOwner) close() {
	o.server.Close()
	o.s.playback.Close(context.Background())
	o.s.store.Close()
	o.lock.Close()
}
func readyWorkflow(t *testing.T, o *workflowOwner) {
	t.Helper()
	entry, _ := o.p.EntryID(o.s.ctx)
	o.p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
	o.p.events <- player.Event{Name: "file-loaded"}
	o.p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("42")}
	deadline := time.Now().Add(2 * time.Second)
	for {
		s, err := o.s.playback.Snapshot(o.s.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if s.Current.PositionSeconds == 42 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("player not ready")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestOfflineWorkflowRefreshSearchPlaybackQueueRestart(t *testing.T) {
	var offline atomic.Bool
	var audio atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if offline.Load() {
			http.Error(w, "offline", 503)
			return
		}
		switch r.URL.Path {
		case "/site":
			fmt.Fprint(w, `<html><head><link rel="alternate" type="application/rss+xml" href="/rss"></head></html>`)
		case "/rss":
			w.Header().Set("ETag", `"v1"`)
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(304)
				return
			}
			fmt.Fprint(w, `<rss><channel><title>Local workflow</title><item><guid>one</guid><title>First</title><description>Archaeology cached search</description><enclosure url="/one.mp3"/></item><item><guid>two</guid><title>Second</title><enclosure url="/two.mp3"/></item></channel></rss>`)
		default:
			audio.Add(1)
			http.Error(w, "must not fetch audio", 500)
		}
	}))
	defer server.Close()
	paths := isolatedProfile(t)
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	first := openWorkflow(t, paths)
	s := first.s
	refresher := feed.New(s.store, server.Client(), 0)
	service := library.New(s.store, refresher)
	preview, err := service.Preview(s.ctx, server.URL+"/site")
	if err != nil {
		t.Fatal(err)
	}
	if feeds, err := s.store.Feeds(s.ctx, false); err != nil || len(feeds) != 0 {
		t.Fatal("preview persisted", feeds, err)
	}
	id, err := service.SubscribePreview(s.ctx, preview)
	if err != nil {
		t.Fatal(err)
	}
	feeds, err := s.store.Feeds(s.ctx, false)
	if err != nil || len(feeds) != 1 || feeds[0].URL != server.URL+"/rss" {
		t.Fatal(feeds, err)
	}
	if result := refresher.Refresh(s.ctx, feeds[0]); result.Error != nil {
		t.Fatal(result.Error)
	}
	hits, err := service.Search(s.ctx, "archaeology")
	if err != nil || len(hits) != 1 {
		t.Fatal(hits, err)
	}
	episodes, err := s.store.Episodes(s.ctx, id)
	if err != nil || len(episodes) != 2 {
		t.Fatal(episodes, err)
	}
	if episodes[0].ID != hits[0].ID {
		episodes[0], episodes[1] = episodes[1], episodes[0]
	}
	model, err := newLibrary(s, refresher)
	if err != nil || model.browse.active {
		t.Fatal("not local-first", err)
	}
	updateLibrary(&model, tea.WindowSizeMsg{Width: 140, Height: 40})
	if first.created.Load() != 0 {
		t.Fatal("startup/preview/search autoplayed")
	}
	model.Model = model.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(episodes[0].ID))
	finishLibrary(&model, libraryKey(&model, 'A'))
	finishLibrary(&model, updateLibrary(&model, tea.KeyPressMsg{Code: tea.KeyEnter}))
	readyWorkflow(t, first)
	syncPlayer(t, &model)
	// Browsing another episode must not make Space replace current audio.
	model.Model = model.Model.Select(ui.Episodes, fmt.Sprint(episodes[1].ID))
	finishLibrary(&model, updateLibrary(&model, tea.KeyPressMsg{Code: tea.KeySpace}))
	if state := syncPlayer(t, &model); state.Current.ID != episodes[0].ID || state.Status != playback.Paused {
		t.Fatal("browse/pause changed episode", state)
	}
	if rows := model.Model.Visible(ui.Queue); len(rows) != 1 || rows[0].ID != fmt.Sprint(episodes[1].ID) || rows[0].Index != 1 {
		t.Fatal("paused current appeared in editable Up next", rows)
	}
	openLibraryDestination(&model, continueDestination)
	if len(model.rows) != 1 || model.rows[0].ID != episodes[0].ID || model.rows[0].PositionSeconds != 42 {
		t.Fatal("paused episode unavailable in Continue", model.rows)
	}
	finishLibrary(&model, updateLibrary(&model, tea.KeyPressMsg{Code: tea.KeySpace}))
	if _, err = control.Call(s.ctx, paths, "pause"); err != nil {
		t.Fatal(err)
	}
	current, err := s.store.Current(s.ctx)
	if err != nil || current.PositionSeconds != 42 {
		t.Fatal("pause checkpoint", current, err)
	}
	if _, err = control.Call(s.ctx, paths, "play"); err != nil {
		t.Fatal(err)
	}
	entry, _ := first.p.EntryID(s.ctx)
	first.p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &entry}
	deadline := time.Now().Add(2 * time.Second)
	for {
		state, err := s.playback.Snapshot(s.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if state.Current.ID == episodes[1].ID {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("EOF did not advance")
		}
		time.Sleep(time.Millisecond)
	}
	completed, err := s.store.Playback(s.ctx, episodes[0].ID)
	if err != nil || !completed.Completed {
		t.Fatal("EOF did not persist completion", err)
	}
	if err = s.playback.Enqueue(s.ctx, episodes[0].ID); err != nil {
		t.Fatal(err)
	}
	readyWorkflow(t, first)
	if _, err = control.Call(s.ctx, paths, "pause"); err != nil {
		t.Fatal(err)
	}
	first.close()
	if _, err = control.Call(context.Background(), paths, "status"); err == nil {
		t.Fatal("quit left endpoint")
	}
	offline.Store(true)
	second := openWorkflow(t, paths)
	s = second.s
	state, err := s.playback.Snapshot(s.ctx)
	if err != nil || state.Status != playback.Stopped || state.Current.ID != episodes[1].ID || state.Current.PositionSeconds != 42 || len(state.Queue) != 1 || second.created.Load() != 0 {
		t.Fatal("restart state/autoplay", state, err)
	}
	offlineRefresher := feed.New(s.store, server.Client(), 0)
	cached := library.New(s.store, offlineRefresher)
	if result := offlineRefresher.Refresh(s.ctx, feeds[0]); result.Error == nil {
		t.Fatal("offline refresh succeeded")
	}
	hits, err = cached.Search(s.ctx, "archaeology")
	if err != nil || len(hits) != 1 || !hits[0].Completed {
		t.Fatal("offline refresh lost cached history", hits, err)
	}
	restored, err := newLibrary(s, offlineRefresher)
	if err != nil {
		t.Fatal(err)
	}
	updateLibrary(&restored, tea.WindowSizeMsg{Width: 70, Height: 20})
	if rows := restored.Model.Visible(ui.Queue); len(rows) != 1 || rows[0].ID != fmt.Sprint(episodes[0].ID) || rows[0].Index != 1 {
		t.Fatal("offline pending/current separation lost", rows)
	}
	openLibraryDestination(&restored, continueDestination)
	if len(restored.rows) != 1 || restored.rows[0].ID != episodes[1].ID || restored.rows[0].PositionSeconds != 42 {
		t.Fatal("offline Continue lost saved history or includes completed episode", restored.rows)
	}
	finishLibrary(&restored, updateLibrary(&restored, tea.KeyPressMsg{Code: tea.KeySpace}))
	if state := syncPlayer(t, &restored); state.Current.ID != episodes[1].ID || state.Status != playback.Playing {
		t.Fatal("offline Space did not resume saved current", state)
	}
	second.p.mu.Lock()
	resume := second.p.resume
	second.p.mu.Unlock()
	if resume != 42 || second.created.Load() != 1 {
		t.Fatal("resume position", resume)
	}
	if stat, err := os.Stat(filepath.Join(paths.Runtime, "control.sock")); err != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("control endpoint permissions", err)
	}
	if audio.Load() != 0 {
		t.Fatal("unexpected audio download")
	}
}
