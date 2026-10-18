package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

type tuiPlayer struct {
	mu            sync.Mutex
	events        chan player.Event
	entry         int64
	loads, closed int
}

func (p *tuiPlayer) Load(context.Context, string, float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entry++
	p.loads++
	return nil
}
func (p *tuiPlayer) EntryID(context.Context) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.entry, nil
}
func (p *tuiPlayer) Play(context.Context) error                { return nil }
func (p *tuiPlayer) Pause(context.Context) error               { return nil }
func (p *tuiPlayer) Stop(context.Context) error                { return nil }
func (p *tuiPlayer) Position(context.Context) (float64, error) { return 42, nil }
func (p *tuiPlayer) Events() <-chan player.Event               { return p.events }
func (p *tuiPlayer) Err() error                                { return errors.New("disconnected") }
func (p *tuiPlayer) Close() error                              { p.mu.Lock(); defer p.mu.Unlock(); p.closed++; return nil }
func playerLibrary(t *testing.T, s *session) (libraryShell, *tuiPlayer) {
	t.Helper()
	seededLibrary(t, s)
	s.playback.Close(s.ctx)
	p := &tuiPlayer{events: make(chan player.Event, 16)}
	engine, err := playback.New(s.ctx, s.store, func(context.Context) (playback.Player, error) { return p, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close(context.Background()) })
	s.playback = engine
	m, err := newLibrary(s, feed.New(s.store, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 32})
	m.Model = m.Model.WithFocus(ui.Episodes)
	return m, p
}
func syncPlayer(t *testing.T, m *libraryShell) playback.State {
	t.Helper()
	state, err := m.s.playback.Snapshot(m.s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	updateLibrary(m, playbackUpdate{state, true})
	return state
}
func TestSelectedPlayToggleQueueEOFAndQuitCheckpoint(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		first, _ := m.selectedEpisode()
		if p.loads != 0 {
			t.Fatal("initial view autoplayed")
		}
		finishLibrary(&m, libraryKey(&m, 'A'))
		state := syncPlayer(t, &m)
		if len(state.Queue) != 2 {
			t.Fatal("batch enqueue", state)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		state = syncPlayer(t, &m)
		if state.Current.ID != first.ID || state.Status != playback.Playing || len(state.Queue) != 1 {
			t.Fatal(state)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if syncPlayer(t, &m).Status != playback.Paused {
			t.Fatal("current didn't pause")
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if syncPlayer(t, &m).Status != playback.Playing {
			t.Fatal("non-episode didn't toggle")
		}
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		data, _ := json.Marshal(42)
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: data}
		deadline := time.Now().Add(time.Second)
		for syncPlayer(t, &m).Current.PositionSeconds != 42 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !strings.Contains(m.View().Content, "0:42 / --:--") {
			t.Fatal("position hidden")
		}
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &entry}
		for syncPlayer(t, &m).Current.ID == first.ID && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		state = syncPlayer(t, &m)
		if state.Current.ID == first.ID || len(state.Queue) != 0 || state.Status != playback.Playing {
			t.Fatal("EOF didn't advance", state)
		}
		completed, err := s.store.Episode(s.ctx, first.ID)
		if err != nil || !completed.Completed {
			t.Fatal("EOF not completed", err)
		}
		finishLibrary(&m, libraryKey(&m, 's'))
		if syncPlayer(t, &m).Status != playback.Stopped {
			t.Fatal("stop failed")
		}
		if err := s.playback.Close(s.ctx); err != nil {
			t.Fatal(err)
		}
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if closed != 1 {
			t.Fatal("player not reaped")
		}
		restored, err := s.store.Current(s.ctx)
		if err != nil || restored.EpisodeID != state.Current.ID {
			t.Fatal("current not persisted", restored, err)
		}
	})
}
func TestQueueEditingConfirmationAndCompactDetails(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 70, Height: 20})
		m.Model = m.Model.WithFocus(ui.Queue)
		original, _ := m.Model.Selected(ui.Queue)
		finishLibrary(&m, libraryKey(&m, 'J'))
		syncPlayer(t, &m)
		if selected, _ := m.Model.Selected(ui.Queue); selected.ID != original.ID {
			t.Fatal("reorder lost identity")
		}
		finishLibrary(&m, libraryKey(&m, 'K'))
		syncPlayer(t, &m)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.Model.Focus() != ui.Details || m.detailTarget == nil || !strings.Contains(m.View().Content, m.detailTarget.Title) {
			t.Fatal("details not opened")
		}
		// Leaving details and navigating must discard the old pinned target.
		m.Model = m.Model.WithFocus(ui.Episodes)
		libraryKey(&m, 'j')
		if m.detailTarget != nil {
			t.Fatal("old details target survived navigation")
		}
		m.Model = m.Model.WithFocus(ui.Queue)
		libraryKey(&m, 'd')
		if m.confirming == nil || !strings.Contains(m.View().Content, "Confirm queue edit") {
			t.Fatal("no confirmation")
		}
		// Ordinary keys cannot mutate or dismiss a confirmation.
		libraryKey(&m, 'J')
		libraryKey(&m, 'n')
		if m.confirming != nil || len(syncPlayer(t, &m).Queue) != 2 {
			t.Fatal("cancel failed")
		}
		libraryKey(&m, 'd')
		finishLibrary(&m, libraryKey(&m, 'y'))
		if len(syncPlayer(t, &m).Queue) != 1 {
			t.Fatal("remove failed")
		}
		libraryKey(&m, 'D')
		expected := append([]int64(nil), m.confirming.ids...)
		// A concurrent change invalidates clear, rather than deleting unseen work.
		if err := s.playback.Enqueue(s.ctx, m.rows[0].ID, m.rows[1].ID); err != nil {
			t.Fatal(err)
		}
		if len(expected) != 1 {
			t.Fatal(expected)
		}
		finishLibrary(&m, libraryKey(&m, 'y'))
		if !strings.Contains(m.notice, "queue changed") {
			t.Fatal(m.notice)
		}
		syncPlayer(t, &m)
		libraryKey(&m, 'D')
		finishLibrary(&m, libraryKey(&m, 'y'))
		if len(syncPlayer(t, &m).Queue) != 0 {
			t.Fatal("clear failed")
		}
		if libraryKey(&m, 'd') != nil || m.confirming != nil {
			t.Fatal("empty queue confirmation")
		}
	})
}
func TestQueueSelectedSpaceAndQuitPersistence(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue)
		wanted := m.state.Queue[1]
		m.Model = m.Model.Select(ui.Queue, fmt.Sprint(wanted.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		state := syncPlayer(t, &m)
		if state.Current.ID != wanted.ID || len(state.Queue) != 1 {
			t.Fatal(state)
		}
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(`42`)}
		deadline := time.Now().Add(time.Second)
		for syncPlayer(t, &m).Current.PositionSeconds != 42 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		libraryKey(&m, 'D')
		cmd := libraryKey(&m, 'q')
		if cmd == nil {
			t.Fatal("quit swallowed")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("confirmation did not allow quit")
		}
		if err := s.playback.Close(s.ctx); err != nil {
			t.Fatal(err)
		}
		history, err := s.store.Playback(s.ctx, wanted.ID)
		if err != nil || history.PositionSeconds != 42 {
			t.Fatal(history, err)
		}
	})
}

func TestFocusedPlayerControlsTransportNotSelectedLibraryEpisode(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		var other store.Episode
		for _, episode := range m.rows {
			if episode.ID != first.ID {
				other = episode
			}
		}
		if err := s.playback.Enqueue(s.ctx, other.ID); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		libraryKey(&m, '5')
		if m.Model.Focus() != ui.PlayerPanel {
			t.Fatal("cannot focus player")
		}
		enter := func() {
			t.Helper()
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
			syncPlayer(t, &m)
		}
		enter()
		if m.state.Status != playback.Paused {
			t.Fatal("Play/Pause button didn't pause")
		}
		enter()
		if m.state.Status != playback.Playing {
			t.Fatal("Play/Pause button didn't resume")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyRight})
		enter()
		if m.state.Status != playback.Stopped {
			t.Fatal("Stop button didn't stop")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyRight})
		enter()
		if m.state.Status != playback.Playing || m.state.Current.ID != other.ID || len(m.state.Queue) != 0 {
			t.Fatal("Next button didn't advance", m.state)
		}
		enter()
		if m.state.Status != playback.Playing {
			t.Fatal("disabled Next changed playback")
		}
		// The selected library item is different, but Player Space controls current.
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(first.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		if m.state.Current.ID != other.ID || m.state.Status != playback.Paused {
			t.Fatal("Player Space started selected library item")
		}
	})
}

func TestPlayerInputGuardsBatchFilterAndFailures(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		libraryKey(&m, 'A')
		if len(syncPlayer(t, &m).Queue) != 0 {
			t.Fatal("filter input executed transport")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		finishLibrary(&m, libraryKey(&m, 'A'))
		if len(syncPlayer(t, &m).Queue) != 1 {
			t.Fatal("batch ignored filter")
		}
		m.Model = m.Model.WithFocus(ui.Queue)
		finishLibrary(&m, libraryKey(&m, 'a'))
		syncPlayer(t, &m)
		libraryKey(&m, 'D')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.confirming != nil {
			t.Fatal("Esc didn't cancel")
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		finishLibrary(&m, libraryKey(&m, 'a'))
		if !strings.Contains(m.notice, "Select episodes") {
			t.Fatal(m.notice)
		}
		if libraryKey(&m, 'J') != nil {
			t.Fatal("reordered outside queue")
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if m.Model.Focus() != ui.Details {
			t.Fatal("Enter didn't open details")
		}
		updateLibrary(&m, playbackUpdate{playback.State{Status: playback.Failed, Error: "persistent offline", Current: &store.Episode{DurationSeconds: 3700}}, true})
		if !strings.Contains(m.View().Content, "persistent offline") || clock(3700) != "1:01:40" {
			t.Fatal("failure/duration missing")
		}
		finishLibrary(&m, libraryKey(&m, 'R'))
		if !strings.Contains(m.View().Content, "persistent offline") {
			t.Fatal("refresh hid persistent player error")
		}
		s.playback.Close(s.ctx)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if !strings.Contains(m.notice, "closed") {
			t.Fatal(m.notice)
		}
		if _, err := newLibrary(s, feed.New(s.store, nil, 0)); !errors.Is(err, playback.ErrClosed) {
			t.Fatal(err)
		}
	})
}
