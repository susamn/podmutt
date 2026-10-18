package playback

import (
	"context"
	"fmt"
	"testing"
)

func TestManualPlaybackStackPromotesAndResumes(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	play := func(index int, pos float64) {
		t.Helper()
		if err := e.PlayEpisode(background, items[index].ID); err != nil {
			t.Fatal(err)
		}
		ready(t, e, p, pos)
	}
	expect := func(current int, pending ...int) {
		t.Helper()
		s := state(t, e)
		if s.Current.ID != items[current].ID {
			t.Fatal("current", s)
		}
		ids := []int64{}
		for _, i := range pending {
			ids = append(ids, items[i].ID)
		}
		if fmt.Sprint(queueIDs(s.Queue)) != fmt.Sprint(ids) {
			t.Fatal("pending", queueIDs(s.Queue), ids)
		}
	}
	play(0, 11)
	expect(0)
	play(1, 22)
	expect(1, 0)
	play(2, 33)
	expect(2, 1, 0)
	if got := state(t, e).Queue; got[0].PositionSeconds != 22 || got[1].PositionSeconds != 11 {
		t.Fatal("interrupted positions", got)
	}
	play(0, 11)
	expect(0, 2, 1)
	p.mu.Lock()
	loads := p.loads
	p.mu.Unlock()
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	after := p.loads
	p.mu.Unlock()
	if after != loads {
		t.Fatal("reloaded already-current row")
	}
	if err := e.Next(background); err != nil {
		t.Fatal(err)
	}
	expect(2, 1)
	if state(t, e).Current.PositionSeconds != 33 {
		t.Fatal("Next lost resume position")
	}
	id := ready(t, e, p, 35)
	p.event("end-file", "eof", id)
	eventually(t, func() bool { return state(t, e).Current.ID == items[1].ID })
	expect(1)
	if state(t, e).Current.PositionSeconds != 22 {
		t.Fatal("EOF lost resume position")
	}
	id = ready(t, e, p, 40)
	p.event("end-file", "eof", id)
	eventually(t, func() bool { return state(t, e).Current.Completed })
	for index, want := range map[int]bool{0: false, 1: true, 2: true} {
		history, err := db.Playback(background, items[index].ID)
		if err != nil || history.Completed != want {
			t.Fatal("history", history, err)
		}
	}
}

func TestStackStoppedRestoreAndLegacyNormalization(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	for i := range 3 {
		if err := e.PlayEpisode(background, items[i].ID); err != nil {
			t.Fatal(err)
		}
		ready(t, e, p, float64((i+1)*10))
	}
	if err := e.Enqueue(background, items[2].ID); err != nil {
		t.Fatal(err)
	}
	if len(state(t, e).Queue) != 2 {
		t.Fatal("current duplicated")
	}
	if err := e.Close(background); err != nil {
		t.Fatal(err)
	}
	// A v1 profile may have explicitly enqueued the current entry too.
	if err := db.ReplaceQueue(background, []int64{items[2].ID, items[1].ID, items[0].ID}); err != nil {
		t.Fatal(err)
	}
	restored, err := New(background, db, func(context.Context) (Player, error) { t.Error("restore started player"); return p, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(background)
	s := state(t, restored)
	if s.Status != Stopped || s.Current.ID != items[2].ID || s.Current.PositionSeconds != 30 || fmt.Sprint(queueIDs(s.Queue)) != fmt.Sprint([]int64{items[1].ID, items[0].ID}) {
		t.Fatal(s)
	}
	persisted, err := db.Queue(background)
	if err != nil || fmt.Sprint(queueIDs(persisted)) != fmt.Sprint(queueIDs(s.Queue)) {
		t.Fatal("normalization not persisted", persisted, err)
	}
}

func TestLegacyNormalizationFailure(t *testing.T) {
	db, p, items := setup(t)
	if err := db.Checkpoint(background, items[0].ID, 10); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceQueue(background, []int64{items[0].ID}); err != nil {
		t.Fatal(err)
	}
	_, err := New(background, &faultRepository{Repository: db, fault: "replace"}, func(context.Context) (Player, error) { return p, nil })
	if err == nil {
		t.Fatal("normalization failure ignored")
	}
}
