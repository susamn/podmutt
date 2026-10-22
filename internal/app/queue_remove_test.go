package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/ui"
)

func TestQueueCurrentRemovalRequiresConfirmationInEveryPlaybackState(t *testing.T) {
	for _, status := range []playback.Status{playback.Playing, playback.Paused, playback.Stopped} {
		for _, size := range [][2]int{{30, 12}, {70, 20}, {140, 40}} {
			for _, deleteKey := range []tea.KeyPressMsg{{Code: 'd', Text: "d"}, {Code: tea.KeyDelete}} {
				t.Run(fmt.Sprintf("%s/%dx%d/%s", status, size[0], size[1], deleteKey.String()), func(t *testing.T) {
					librarySession(t, func(s *session) {
						m, p := playerLibrary(t, s)
						// This harness executes finite commands, not live presentation timers.
						m.Model = m.Model.WithProgress(nil, nil, nil)
						finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
						entry, _ := p.EntryID(s.ctx)
						p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
						p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("42")}
						deadline := time.Now().Add(time.Second)
						for syncPlayer(t, &m).Current.PositionSeconds != 42 {
							if time.Now().After(deadline) {
								t.Fatal("no position")
							}
							time.Sleep(time.Millisecond)
						}
						finishLibrary(&m, libraryKey(&m, 'A'))
						switch status {
						case playback.Paused:
							if err := s.playback.Pause(s.ctx); err != nil {
								t.Fatal(err)
							}
						case playback.Stopped:
							if err := s.playback.Stop(s.ctx); err != nil {
								t.Fatal(err)
							}
						}
						state := syncPlayer(t, &m)
						id := state.Current.ID
						pending := state.Queue[0].ID
						m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(id))
						updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
						if cmd := updateLibrary(&m, deleteKey); cmd != nil || m.confirming == nil || m.queueEditing {
							t.Fatal("no guarded confirmation")
						}
						if got := syncPlayer(t, &m); got.Current.ID != id || got.Status != status || len(got.Queue) != 1 {
							t.Fatal("opening dialog mutated playback", got)
						}
						view := ansi.Strip(m.View().Content)
						for _, text := range []string{state.Current.Title, state.Current.FeedTitle, string(status), "y/n", "y remove", "n/Esc cancel"} {
							if !strings.Contains(view, text) {
								t.Fatalf("dialog missing %q at %v:\n%s", text, size, view)
							}
						}
						finishLibrary(&m, libraryKey(&m, 'y'))
						// The authoritative action reply removes the row without waiting for a streamed update.
						if m.state.Current != nil || len(m.Model.Visible(ui.Queue)) != 1 {
							t.Fatal("removal reply left current visible", m.state, m.notice, m.Model.Visible(ui.Queue))
						}
						got := syncPlayer(t, &m)
						if got.Current != nil || got.Status != playback.Stopped || len(got.Queue) != 1 || got.Queue[0].ID != pending {
							t.Fatal("removed wrong row or advanced", got)
						}
						if history, err := s.store.Playback(s.ctx, id); err != nil || history.PositionSeconds != 42 || history.Completed {
							t.Fatal("history lost", history, err)
						}
						if _, err := s.store.Current(s.ctx); !errors.Is(err, sql.ErrNoRows) {
							t.Fatal("current pointer remains", err)
						}
						finishLibrary(&m, libraryKey(&m, 'u'))
						if got := syncPlayer(t, &m); got.Current != nil || got.Status != playback.Stopped || len(got.Queue) != 2 {
							t.Fatal("undo autoplayed", got)
						}
						p.mu.Lock()
						loads := p.loads
						p.mu.Unlock()
						if loads != 1 {
							t.Fatal("removal/undo loaded audio", loads)
						}
					})
				})
			}
		}
	}
}

func TestRemoveOnlyCurrentLeavesEmptyQueueAndUndoDoesNotResume(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		id := syncPlayer(t, &m).Current.ID
		finishLibrary(&m, libraryKey(&m, 's'))
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(id))
		libraryKey(&m, 'd')
		finishLibrary(&m, libraryKey(&m, 'y'))
		if got := syncPlayer(t, &m); got.Current != nil || len(got.Queue) != 0 || len(m.Model.Visible(ui.Queue)) != 0 {
			t.Fatal("only-current row remained", got)
		}
		if libraryKey(&m, 'd') != nil || m.confirming != nil {
			t.Fatal("empty Queue opened removal dialog")
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		if got := syncPlayer(t, &m); got.Current != nil || got.Status != playback.Stopped || len(got.Queue) != 1 || got.Queue[0].ID != id {
			t.Fatal("undo resumed current", got)
		}
	})
}

func TestQueueRemovalCancellationAndOtherPanelsDoNotMutate(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		finishLibrary(&m, libraryKey(&m, 'A'))
		state := syncPlayer(t, &m)
		for _, panel := range []ui.Panel{ui.Podcasts, ui.Episodes, ui.Details, ui.PlayerPanel} {
			m.Model = m.Model.WithFocus(panel)
			for _, key := range []tea.KeyPressMsg{{Code: 'd', Text: "d"}, {Code: tea.KeyDelete}} {
				if cmd := updateLibrary(&m, key); cmd != nil || m.confirming != nil {
					t.Fatal("delete escaped queue", panel)
				}
			}
		}
		for _, id := range []int64{state.Current.ID, state.Queue[0].ID} {
			for _, cancel := range []tea.KeyPressMsg{{Code: 'n', Text: "n"}, {Code: tea.KeyEscape}} {
				m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(id))
				libraryKey(&m, 'd')
				libraryKey(&m, 'j')
				libraryKey(&m, 'd')
				libraryKey(&m, 'u')
				if m.confirming == nil {
					t.Fatal("ordinary keys dismissed confirmation")
				}
				updateLibrary(&m, cancel)
				if m.confirming != nil || m.Model.Busy() {
					t.Fatal("cancel failed")
				}
				if got := syncPlayer(t, &m); got.Current.ID != state.Current.ID || got.Status != state.Status || len(got.Queue) != 1 {
					t.Fatal("cancel changed playback", got)
				}
			}
		}
	})
}

func TestRemovalDialogCapturesIdentityAcrossSortSelectionAndStaleRows(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		target, other := m.state.Queue[0].ID, m.state.Queue[1].ID
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(target))
		libraryKey(&m, 'd')
		// Simulate an asynchronous selection/display change while the dialog is pinned.
		m.Model = m.Model.ToggleQueueOrder().Select(ui.Queue, fmt.Sprint(other))
		finishLibrary(&m, libraryKey(&m, 'y'))
		if got := syncPlayer(t, &m); len(got.Queue) != 1 || got.Queue[0].ID != other {
			t.Fatal("removed row under new cursor", got)
		}
		libraryKey(&m, 'd')
		if err := s.playback.RemoveQueued(s.ctx, other); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, libraryKey(&m, 'y'))
		if !strings.Contains(m.notice, "queue changed") {
			t.Fatal("stale identity silently accepted", m.notice)
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		if got := syncPlayer(t, &m); len(got.Queue) != 1 || got.Queue[0].ID != target {
			t.Fatal("failed removal replaced undo", got)
		}
	})
}
