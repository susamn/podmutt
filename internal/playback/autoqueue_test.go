package playback

import (
	"context"
	"sync"
	"testing"
)

func TestLatestArrivalDoesNotInterruptOrResurrect(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 12)
	if err := e.QueueLatest(background, 0); err != nil {
		t.Fatal(err)
	}
	s := state(t, e)
	if s.Status != Playing || s.Current.ID != items[0].ID || s.Current.PositionSeconds != 12 || len(s.Queue) != 1 || s.Queue[0].GUID != "a" {
		t.Fatal("arrival changed transport", s)
	}
	if err := e.RemoveQueued(background, s.Queue[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := e.QueueLatest(background, 0); err != nil {
		t.Fatal(err)
	}
	if len(state(t, e).Queue) != 0 {
		t.Fatal("removal undone")
	}
	if err := e.QueueLatest(background, -1); err == nil {
		t.Fatal("queue latest error swallowed")
	}
}

func TestLatestSerializedWithManualQueueWhileStopped(t *testing.T) {
	db, _, items := setup(t)
	e, err := New(background, db, func(context.Context) (Player, error) {
		t.Error("autoqueue started player")
		return nil, context.Canceled
	})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close(background)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := e.QueueLatest(background, 0); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := e.Enqueue(background, items[0].ID); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	s := state(t, e)
	if s.Status != Stopped || s.Current != nil || len(s.Queue) != 2 {
		t.Fatal("auto/manual race", s)
	}
}
