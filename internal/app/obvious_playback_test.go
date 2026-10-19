package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/ui"
)

func TestManualTailSpaceVisualSortAndEOFAreIndependent(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		newer, older := m.rows[0], m.rows[1]
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(older.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(newer.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))
		rows := m.Model.Visible(ui.Queue)
		selected, _ := m.Model.Selected(ui.Queue)
		if len(rows) != 2 || rows[1].ID != fmt.Sprint(newer.ID) || selected.ID != fmt.Sprint(newer.ID) || m.Model.Focus() != ui.Queue {
			t.Fatal("manual addition did not append/select/focus", rows, selected)
		}
		if state := syncPlayer(t, &m); state.Current.ID != older.ID || state.Status != playback.Playing || p.loads != 1 {
			t.Fatal("enqueue autoplayed", state)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if state := syncPlayer(t, &m); state.Current.ID != older.ID || state.Status != playback.Paused {
			t.Fatal("Space followed selection", state)
		}
		libraryKey(&m, 'o')
		libraryKey(&m, 'o')
		if selected, _ = m.Model.Selected(ui.Queue); selected.ID != fmt.Sprint(newer.ID) || m.Model.Visible(ui.Queue)[0].ID != fmt.Sprint(newer.ID) {
			t.Fatal("sort lost identity or kept tail override")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &entry}
		deadline := time.Now().Add(time.Second)
		for syncPlayer(t, &m).Current.ID != newer.ID {
			if time.Now().After(deadline) {
				t.Fatal("EOF ignored publication priority")
			}
			time.Sleep(time.Millisecond)
		}
		if selected, _ = m.Model.Selected(ui.Queue); selected.ID != fmt.Sprint(newer.ID) {
			t.Fatal("activation did not select current", selected)
		}
		if history, err := s.store.Playback(s.ctx, older.ID); err != nil || !history.Completed {
			t.Fatal("EOF did not complete old selection", history, err)
		}
	})
}

func TestStartupResumesAndAlignsOnceIncludingUnsubscribedOfflineWork(t *testing.T) {
	for _, subscribed := range []bool{true, false} {
		t.Run(fmt.Sprint(subscribed), func(t *testing.T) {
			librarySession(t, func(s *session) {
				m, p := playerLibrary(t, s)
				target, other := m.rows[1], m.rows[0]
				if err := s.playback.PlayEpisode(s.ctx, target.ID); err != nil {
					t.Fatal(err)
				}
				p.mu.Lock()
				p.hasSeek, p.seekPosition = true, 73
				p.mu.Unlock()
				entry, _ := p.EntryID(s.ctx)
				p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
				p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("73")}
				deadline := time.Now().Add(time.Second)
				for syncPlayer(t, &m).Current.PositionSeconds != 73 {
					if time.Now().After(deadline) {
						t.Fatal("saved progress missing")
					}
					time.Sleep(time.Millisecond)
				}
				if err := s.playback.Pause(s.ctx); err != nil {
					t.Fatal(err)
				}
				if !subscribed {
					if err := s.store.SetSubscribed(s.ctx, target.FeedID, false); err != nil {
						t.Fatal(err)
					}
				}
				if err := s.playback.Close(s.ctx); err != nil {
					t.Fatal(err)
				}
				p = &tuiPlayer{events: make(chan player.Event, 16)}
				engine, err := playback.New(s.ctx, s.store, func(context.Context) (playback.Player, error) { return p, nil })
				if err != nil {
					t.Fatal(err)
				}
				defer engine.Close(context.Background())
				s.playback = engine
				m, err = newLibrary(s, feed.New(s.store, nil, 0))
				if err != nil {
					t.Fatal(err)
				}
				updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})
				wantScope := allDestination
				if subscribed {
					wantScope = fmt.Sprint(target.FeedID)
				}
				if m.libraryID != wantScope || m.browse.active || p.loads != 0 {
					t.Fatal("constructor side effect or wrong scope", m.libraryID, p.loads)
				}
				for _, panel := range []ui.Panel{ui.Episodes, ui.Queue} {
					if selected, _ := m.Model.Selected(panel); selected.ID != fmt.Sprint(target.ID) {
						t.Fatal("startup identity not aligned", panel, selected)
					}
				}
				if selected, _ := m.Model.Selected(ui.Podcasts); selected.ID != wantScope {
					t.Fatal("show not aligned", selected)
				}
				finishLibrary(&m, updateLibrary(&m, libraryStartup{}))
				if state := syncPlayer(t, &m); state.Current.ID != target.ID || state.Current.PositionSeconds != 73 || state.Status != playback.Playing || p.loads != 1 {
					t.Fatal("saved audio not resumed", state, p.loads)
				}
				if updateLibrary(&m, libraryStartup{}) != nil {
					t.Fatal("startup resumed twice")
				}
				// Only subscribed scopes also expose an unplayed other episode.
				if subscribed {
					m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(other.ID))
					entry, _ := p.EntryID(s.ctx)
					p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
					p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("50")}
					deadline := time.Now().Add(time.Second)
					for syncPlayer(t, &m).Current.PositionSeconds != 50 {
						if time.Now().After(deadline) {
							t.Fatal("progress missing")
						}
						time.Sleep(time.Millisecond)
					}
					if selected, _ := m.Model.Selected(ui.Episodes); selected.ID != fmt.Sprint(other.ID) {
						t.Fatal("progress snapped browsing back", selected)
					}
				}
				feeds, err := s.store.Feeds(s.ctx, true)
				if err != nil || len(feeds) != map[bool]int{true: 1, false: 0}[subscribed] {
					t.Fatal("startup subscribed saved work", feeds, err)
				}
			})
		})
	}
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		if updateLibrary(&m, libraryStartup{}) != nil || !m.startupDone {
			t.Fatal("empty startup scheduled playback")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		current := syncPlayer(t, &m).Current.ID
		if err := s.playback.MarkCompleted(s.ctx, current, true); err != nil {
			t.Fatal(err)
		}
		if err := s.playback.Stop(s.ctx); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		m.startupDone = false
		finishLibrary(&m, updateLibrary(&m, libraryStartup{}))
		if state := syncPlayer(t, &m); !state.Current.Completed || state.Status != playback.Stopped || p.loads != 1 {
			t.Fatal("startup replayed completed audio", state)
		}
	})
}

func TestLateActorPublicationCannotUndoEnqueueProjectionOrStealSearchFocus(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		before := syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Episodes)
		pending := libraryKey(&m, 'a')
		reply := pending().(playerEdited)
		if reply.state == nil || reply.state.Revision <= before.Revision {
			t.Fatal("actor revisions did not advance", before, reply.state)
		}
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		updateLibrary(&m, reply)
		if m.Model.Focus() != ui.Episodes || !m.Model.Busy() {
			t.Fatal("reply hijacked filter editor")
		}
		updateLibrary(&m, playbackUpdate{state: before, ok: true})
		if len(m.state.Queue) != 1 || len(m.Model.Visible(ui.Queue)) != 1 {
			t.Fatal("late publication replaced authoritative enqueue")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.Model.Focus() != ui.Queue || m.Model.HasFilter(ui.Queue) || m.queueSelectID != "" {
			t.Fatal("deferred Queue focus was lost")
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		finishLibrary(&m, libraryKey(&m, 'a'))
		now := syncPlayer(t, &m)
		projected := m.shell.withPlayback(before)
		if projected.state.Revision != now.Revision || len(projected.Model.Visible(ui.Queue)) != len(now.Queue) {
			t.Fatal("direct stale snapshot bypassed freshness guard")
		}
	})
}
