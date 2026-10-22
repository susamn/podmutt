package playback

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
)

func TestConfirmedCurrentRemovalStopsSavesAndNeverAdvances(t *testing.T) {
	for _, status := range []Status{Playing, Paused, Stopped, Failed} {
		for _, completed := range []bool{false, true} {
			t.Run(string(status)+map[bool]string{false: "/unfinished", true: "/completed"}[completed], func(t *testing.T) {
				db, p, items := setup(t)
				e, _ := engineFor(t, db, p)
				a, b := items[0].ID, items[1].ID
				if err := e.PlayEpisode(background, a); err != nil {
					t.Fatal(err)
				}
				entry := ready(t, e, p, 37)
				if err := e.Enqueue(background, b); err != nil {
					t.Fatal(err)
				}
				if completed {
					if err := e.MarkCompleted(background, a, true); err != nil {
						t.Fatal(err)
					}
				}
				switch status {
				case Paused:
					if err := e.Pause(background); err != nil {
						t.Fatal(err)
					}
				case Stopped:
					if err := e.Stop(background); err != nil {
						t.Fatal(err)
					}
				case Failed:
					p.event("end-file", "error", entry)
					eventually(t, func() bool { return state(t, e).Status == Failed })
				}
				if err := e.RemoveQueueItemUndoable(background, a); err != nil {
					t.Fatal(err)
				}
				got := state(t, e)
				if got.Current != nil || got.Status != Stopped || got.Error != "" || got.CanSeek || !got.ClockAt.IsZero() || !slices.Equal(queueIDs(got.Queue), []int64{b}) {
					t.Fatal(got)
				}
				if _, err := db.Current(background); !errors.Is(err, sql.ErrNoRows) {
					t.Fatal("current pointer persisted", err)
				}
				history, err := db.Playback(background, a)
				if err != nil || history.PositionSeconds != 37 || history.Completed != completed {
					t.Fatal("history changed", history, err)
				}
				// A delayed completion for removed audio cannot complete it or start pending work.
				p.event("end-file", "eof", entry)
				if got := state(t, e); got.Current != nil || got.Status != Stopped {
					t.Fatal("stale EOF advanced", got)
				}
				if err := e.UndoQueueRemoval(background); err != nil {
					t.Fatal(err)
				}
				got = state(t, e)
				if got.Current != nil || got.Status != Stopped || !slices.Equal(queueIDs(got.Queue), []int64{a, b}) {
					t.Fatal("undo autoplayed or lost queue", got)
				}
				if err := e.Close(background); err != nil {
					t.Fatal(err)
				}
				restored, err := New(background, db, func(context.Context) (Player, error) { t.Fatal("restart created player"); return nil, nil })
				if err != nil {
					t.Fatal(err)
				}
				defer restored.Close(background)
				if got := state(t, restored); got.Current != nil || got.Status != Stopped {
					t.Fatal("restart resurrected removed current", got)
				}
			})
		}
	}
}

func TestConfirmedRemovalUsesIdentityAgainstLiveStack(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	a, b, c := items[0].ID, items[1].ID, items[2].ID
	if err := e.PlayEpisode(background, a); err != nil {
		t.Fatal(err)
	}
	if err := e.Enqueue(background, b, c); err != nil {
		t.Fatal(err)
	}
	// A captured current becomes pending when another item is played.
	if err := e.PlayEpisode(background, b); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveQueueItemUndoable(background, a); err != nil {
		t.Fatal(err)
	}
	if got := state(t, e); got.Current.ID != b || got.Status != Playing || !slices.Equal(queueIDs(got.Queue), []int64{c}) {
		t.Fatal("removed different current", got)
	}
	if err := e.RemoveQueueItemUndoable(background, a); !errors.Is(err, ErrQueueChanged) {
		t.Fatal("stale identity accepted", err)
	}
	if err := e.RemoveQueueItemUndoable(background, 999); !errors.Is(err, ErrQueueChanged) {
		t.Fatal(err)
	}
	// A captured pending episode may have become current by confirmation time.
	if err := e.PlayEpisode(background, c); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveQueueItemUndoable(background, c); err != nil {
		t.Fatal(err)
	}
	if got := state(t, e); got.Current != nil || !slices.Equal(queueIDs(got.Queue), []int64{b}) {
		t.Fatal(got)
	}
}

func TestCurrentRemovalFailureKeepsPointerHistoryQueueAndPriorUndo(t *testing.T) {
	for _, fault := range []string{"checkpoint", "stop", "clear-current"} {
		t.Run(fault, func(t *testing.T) {
			db, p, items := setup(t)
			repo := &faultRepository{Repository: db}
			e, _ := engineFor(t, repo, p)
			a, b, c := items[0].ID, items[1].ID, items[2].ID
			if err := e.PlayEpisode(background, a); err != nil {
				t.Fatal(err)
			}
			ready(t, e, p, 29)
			if err := e.Enqueue(background, b, c); err != nil {
				t.Fatal(err)
			}
			if err := e.RemoveQueuedUndoable(background, b); err != nil {
				t.Fatal(err)
			}
			if fault == "stop" {
				p.setFailure("stop")
			} else {
				repo.set(fault)
			}
			if err := e.RemoveQueueItemUndoable(background, a); err == nil {
				t.Fatal("expected failure")
			}
			got := state(t, e)
			if got.Current == nil || got.Current.ID != a || !slices.Equal(queueIDs(got.Queue), []int64{c}) {
				t.Fatal("failure discarded work", got)
			}
			saved, err := db.Current(background)
			if err != nil || saved.EpisodeID != a || saved.Completed {
				t.Fatal(saved, err)
			}
			repo.set("")
			p.setFailure("")
			if err := e.UndoQueueRemoval(background); err != nil {
				t.Fatal(err)
			}
			if got := state(t, e); got.Current.ID != a || !slices.Equal(queueIDs(got.Queue), []int64{b, c}) {
				t.Fatal("failed removal replaced undo", got)
			}
		})
	}
}

func TestRemoveOnlyStoppedCurrentRestoredWithoutAPlayer(t *testing.T) {
	db, _, items := setup(t)
	id := items[0].ID
	if err := db.Checkpoint(background, id, 21); err != nil {
		t.Fatal(err)
	}
	e, err := New(background, db, func(context.Context) (Player, error) { t.Fatal("removal/undo started a player"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close(background)
	if err := e.RemoveQueueItemUndoable(background, id); err != nil {
		t.Fatal(err)
	}
	if got := state(t, e); got.Current != nil || len(got.Queue) != 0 || got.Status != Stopped {
		t.Fatal(got)
	}
	if saved, err := db.Playback(background, id); err != nil || saved.PositionSeconds != 21 || saved.Completed {
		t.Fatal(saved, err)
	}
	if err := e.UndoQueueRemoval(background); err != nil {
		t.Fatal(err)
	}
	if got := state(t, e); got.Current != nil || got.Status != Stopped || len(got.Queue) != 1 || got.Queue[0].ID != id {
		t.Fatal(got)
	}
}
