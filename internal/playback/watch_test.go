package playback

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIndependentWatchersAndSnapshotIsolation(t *testing.T) {
	db, p, items := setup(t)
	if err := db.Checkpoint(background, items[0].ID, 12); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceQueue(background, []int64{items[1].ID}); err != nil {
		t.Fatal(err)
	}
	engine, _ := engineFor(t, db, p)
	first, err := engine.Watch(background)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Watch(background)
	if err != nil {
		t.Fatal(err)
	}
	a, b := <-first, <-second
	a.Current.Title = "changed"
	a.Queue[0].ID = 999
	if b.Current.Title == "changed" || b.Queue[0].ID == 999 || state(t, engine).Current.Title == "changed" {
		t.Fatal("watchers share mutable state")
	}
	if err := engine.Play(background); err != nil {
		t.Fatal(err)
	}
	for _, stream := range []<-chan State{first, second, engine.Updates()} {
		select {
		case update := <-stream:
			if update.Status != Playing {
				t.Fatalf("subscriber lost update: %+v", update)
			}
		case <-time.After(time.Second):
			t.Fatal("one subscriber stole another's update")
		}
	}
}
func TestWatcherCancellationCoalescingAndShutdown(t *testing.T) {
	db, p, _ := setup(t)
	engine, _ := engineFor(t, db, p)
	ctx, cancel := context.WithCancel(background)
	stream, err := engine.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if err := engine.Stop(background); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	closed := make(chan struct{})
	go func() {
		for range stream {
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("canceled watcher stayed open")
	}
	if _, err := engine.Watch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled watch: %v", err)
	}
	live, err := engine.Watch(background)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(background); err != nil {
		t.Fatal(err)
	}
	for range live {
	}
	if _, err := engine.Watch(background); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed watch: %v", err)
	}
}
