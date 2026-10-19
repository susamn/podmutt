package playback

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestQueueUndoMergesWithLiveOrderAndEnqueues(t *testing.T) {
	for _, scenario := range []string{"first", "middle", "last", "reorder", "enqueue", "no-neighbors", "two-removals"} {
		t.Run(scenario, func(t *testing.T) {
			db, p, items := setup(t)
			e, _ := engineFor(t, db, p)
			a, b, c := items[0].ID, items[1].ID, items[2].ID
			initial, removed, want := []int64{a, b, c}, b, []int64{a, b, c}
			if scenario == "first" {
				removed = a
			} else if scenario == "last" {
				removed = c
			} else if scenario == "enqueue" {
				initial = []int64{a, b}
				want = []int64{c, a, b}
			}
			if err := e.SetQueue(background, initial); err != nil {
				t.Fatal(err)
			}
			if err := e.RemoveQueuedUndoable(background, removed); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "reorder":
				if err := e.MoveQueued(background, c, -1); err != nil {
					t.Fatal(err)
				}
				want = []int64{b, c, a}
			case "enqueue":
				if err := e.Enqueue(background, c); err != nil {
					t.Fatal(err)
				}
				if err := e.MoveQueued(background, c, -1); err != nil {
					t.Fatal(err)
				}
			case "two-removals":
				if err := e.RemoveQueuedUndoable(background, c); err != nil {
					t.Fatal(err)
				}
				want = []int64{a, c}
			case "no-neighbors":
				if err := e.RemoveQueued(background, a, c); err != nil {
					t.Fatal(err)
				}
				want = []int64{b}
			}
			if err := e.UndoQueueRemoval(background); err != nil {
				t.Fatal(err)
			}
			if got := queueIDs(state(t, e).Queue); !slices.Equal(got, want) {
				t.Fatal(got, want)
			}
			if err := e.UndoQueueRemoval(background); !errors.Is(err, ErrNoQueueUndo) {
				t.Fatal("undo was not single-use", err)
			}
		})
	}
}

func TestQueueUndoFailuresKeepLastSuccessfulRemoval(t *testing.T) {
	db, p, items := setup(t)
	repo := &faultRepository{Repository: db}
	e, _ := engineFor(t, repo, p)
	a, b, c := items[0].ID, items[1].ID, items[2].ID
	if err := e.Enqueue(background, a, b, c); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveQueuedUndoable(background, b); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveQueuedUndoable(background, b); !errors.Is(err, ErrQueueChanged) {
		t.Fatal(err)
	}
	if err := e.ClearQueue(background, []int64{a, b, c}); !errors.Is(err, ErrQueueChanged) {
		t.Fatal(err)
	}
	repo.set("replace")
	if err := e.RemoveQueuedUndoable(background, c); !errors.Is(err, testFailure) {
		t.Fatal(err)
	}
	if err := e.UndoQueueRemoval(background); !errors.Is(err, testFailure) {
		t.Fatal(err)
	}
	repo.set("")
	canceled, cancel := context.WithCancel(background)
	cancel()
	if err := e.UndoQueueRemoval(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := e.UndoQueueRemoval(background); err != nil {
		t.Fatal(err)
	}
	if got := queueIDs(state(t, e).Queue); !slices.Equal(got, []int64{a, b, c}) {
		t.Fatal(got)
	}
}

func TestQueueUndoCannotResurrectPlayedOrExplicitlyChangedWork(t *testing.T) {
	for _, action := range []string{"play-and-advance", "enqueue", "clear", "replace", "complete"} {
		t.Run(action, func(t *testing.T) {
			db, p, items := setup(t)
			e, _ := engineFor(t, db, p)
			a, b, c := items[0].ID, items[1].ID, items[2].ID
			if err := e.Enqueue(background, a, b, c); err != nil {
				t.Fatal(err)
			}
			if err := e.RemoveQueuedUndoable(background, b); err != nil {
				t.Fatal(err)
			}
			var err error
			switch action {
			case "play-and-advance":
				if err = e.PlayEpisode(background, b); err == nil {
					err = e.Next(background)
				}
			case "enqueue":
				if err = e.Enqueue(background, b); err == nil {
					err = e.RemoveQueued(background, b)
				}
			case "clear":
				err = e.ClearQueue(background, []int64{a, c})
			case "replace":
				err = e.SetQueue(background, []int64{a, c})
			case "complete":
				err = e.MarkCompleted(background, b, true)
			}
			if err != nil {
				t.Fatal(err)
			}
			before := queueIDs(state(t, e).Queue)
			if err := e.UndoQueueRemoval(background); !errors.Is(err, ErrNoQueueUndo) {
				t.Fatal(err)
			}
			if got := queueIDs(state(t, e).Queue); !slices.Equal(got, before) {
				t.Fatal("stale undo changed queue", got, before)
			}
		})
	}
}

func TestAutoqueueReadditionInvalidatesEarlierManualRemoval(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	var latest int64
	for _, item := range items {
		if item.GUID == "a" {
			latest = item.ID
		}
	}
	if err := e.Enqueue(background, latest); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveQueuedUndoable(background, latest); err != nil {
		t.Fatal(err)
	}
	if err := e.QueueLatest(background, 0); err != nil {
		t.Fatal(err)
	}
	if got := queueIDs(state(t, e).Queue); !slices.Equal(got, []int64{latest}) {
		t.Fatal("fixture not readded by policy", got)
	}
	if err := e.RemoveQueued(background, latest); err != nil {
		t.Fatal(err)
	}
	if err := e.UndoQueueRemoval(background); !errors.Is(err, ErrNoQueueUndo) {
		t.Fatal("old removal resurrected after autoqueue", err)
	}
}

func TestQueueUndoAfterEOFAndStoppedRestoreKeepsHistory(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	a, b, c := items[0].ID, items[1].ID, items[2].ID
	if err := db.Checkpoint(background, b, 22); err != nil {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, a); err != nil {
		t.Fatal(err)
	}
	if err := e.Enqueue(background, b, c); err != nil {
		t.Fatal(err)
	}
	entry := ready(t, e, p, 12)
	if err := e.RemoveQueuedUndoable(background, a); !errors.Is(err, ErrQueueChanged) {
		t.Fatal("current was removable", err)
	}
	if err := e.RemoveQueuedUndoable(background, b); err != nil {
		t.Fatal(err)
	}
	p.event("end-file", "eof", entry)
	eventually(t, func() bool { return state(t, e).Current.ID == c })
	if err := e.UndoQueueRemoval(background); err != nil {
		t.Fatal(err)
	}
	got := state(t, e)
	if got.Status != Playing || got.Current.ID != c || len(got.Queue) != 1 || got.Queue[0].ID != b || got.Queue[0].PositionSeconds != 22 {
		t.Fatal("undo overwrote advancement/progress", got)
	}
	if history, err := db.Playback(background, a); err != nil || !history.Completed || history.PositionSeconds != 12 {
		t.Fatal(history, err)
	}
	if err := e.Close(background); err != nil {
		t.Fatal(err)
	}
	restored, _ := engineFor(t, db, p)
	if got := state(t, restored); got.Status != Stopped || got.Current.ID != c || !slices.Equal(queueIDs(got.Queue), []int64{b}) {
		t.Fatal(got)
	}
	if err := restored.UndoQueueRemoval(background); !errors.Is(err, ErrNoQueueUndo) {
		t.Fatal("undo survived restart", err)
	}
}
