package playback

import (
	"errors"
	"reflect"
	"testing"
)

func TestAtomicSelectedToggle(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.Toggle(background, 0); !errors.Is(err, ErrNoEpisode) {
		t.Fatal(err)
	}
	for _, id := range []int64{items[0].ID, items[1].ID} {
		if err := e.Toggle(background, id); err != nil {
			t.Fatal(err)
		}
		if got := state(t, e); got.Current.ID != id || got.Status != Playing {
			t.Fatal(got)
		}
		if err := e.Toggle(background, id); err != nil {
			t.Fatal(err)
		}
		if state(t, e).Status != Paused {
			t.Fatal("did not pause current")
		}
		if err := e.Toggle(background, 0); err != nil {
			t.Fatal(err)
		}
		if state(t, e).Status != Playing {
			t.Fatal("did not resume current")
		}
	}
}

func TestQueueIdentityEditsAndStaleClear(t *testing.T) {
	db, p, items := setup(t)
	repo := &faultRepository{Repository: db}
	e, _ := engineFor(t, repo, p)
	ids := []int64{items[0].ID, items[1].ID, items[2].ID}
	check := func(want []int64) {
		t.Helper()
		if got := queueIDs(state(t, e).Queue); !reflect.DeepEqual(got, want) {
			t.Fatalf("queue %v want %v", got, want)
		}
	}
	if err := e.Enqueue(background, ids...); err != nil {
		t.Fatal(err)
	}
	if err := e.MoveQueued(background, ids[1], -1); err != nil {
		t.Fatal(err)
	}
	check([]int64{ids[1], ids[0], ids[2]})
	if err := e.ClearQueue(background, ids); !errors.Is(err, ErrQueueChanged) {
		t.Fatal(err)
	}
	if err := e.MoveQueued(background, ids[2], 1); err != nil {
		t.Fatal(err)
	}
	if err := e.MoveQueued(background, -1, 1); !errors.Is(err, ErrQueueChanged) {
		t.Fatal(err)
	}
	if err := e.MoveQueued(background, ids[0], 3); err == nil {
		t.Fatal("invalid step")
	}
	repo.set("replace")
	if err := e.RemoveQueued(background, ids[0]); !errors.Is(err, testFailure) {
		t.Fatal(err)
	}
	check([]int64{ids[1], ids[0], ids[2]})
	repo.set("")
	if err := e.PlayEpisode(background, ids[1]); err != nil {
		t.Fatal(err)
	}
	// A stale identity edit cannot reintroduce the consumed queue head.
	if err := e.MoveQueued(background, ids[1], 1); !errors.Is(err, ErrQueueChanged) {
		t.Fatal(err)
	}
	if err := e.RemoveQueued(background, ids[1], ids[0]); err != nil {
		t.Fatal(err)
	}
	check([]int64{ids[2]})
	if err := e.ClearQueue(background, []int64{ids[2]}); err != nil {
		t.Fatal(err)
	}
	check([]int64{})
	if state(t, e).Status != Playing {
		t.Fatal("queue edit interrupted playback")
	}
	persisted, err := db.Queue(background)
	if err != nil || len(persisted) != 0 {
		t.Fatal(persisted, err)
	}
}
