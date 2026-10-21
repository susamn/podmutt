package activity

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLifecycleNestedProgressPriorityAndIsolation(t *testing.T) {
	tracker := New()
	root := WithTracker(context.Background(), tracker)
	background, finish := Begin(root, "Refreshing feeds…", Background)
	_, foregroundDone := Begin(root, "Searching…", Foreground)
	nested, nestedDone := Begin(background, "Nested fetch", Foreground)
	if nested != background {
		t.Fatal("nested operation replaced context")
	}
	nestedDone()
	Progress(background, 1, 3)
	Progress(background, 0, 3)
	s := tracker.Snapshot()
	if len(s.Operations) != 2 || s.Operations[0].Label != "Searching…" || s.Operations[1].Completed != 1 || s.Operations[1].Total != 3 {
		t.Fatalf("snapshot: %+v", s)
	}
	s.Operations[0].Label = "mutated"
	if tracker.Snapshot().Operations[0].Label != "Searching…" {
		t.Fatal("snapshot aliases tracker")
	}
	foregroundDone()
	foregroundDone()
	finish()
	Progress(background, 3, 3)
	if len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("unfinished work")
	}
	select {
	case <-tracker.Changed():
	default:
		t.Fatal("no wakeup")
	}
	select {
	case <-tracker.Changed():
		t.Fatal("changes not coalesced")
	default:
	}
}

func TestCancellationConcurrentFinishAndNoTracker(t *testing.T) {
	tracker := New()
	root, cancel := context.WithCancel(WithTracker(context.Background(), tracker))
	ctx, finish := Begin(root, "Waiting", Background)
	cancel()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { Progress(ctx, 2, 3); finish(); _ = tracker.Snapshot() })
	}
	wg.Wait()
	if len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("cancellation leaked operation")
	}
	_, end := Begin(root, "Canceled", Foreground)
	end()
	plain := context.Background()
	same, end := Begin(plain, "No tracker", Foreground)
	end()
	Progress(plain, 0, 0)
	if same != plain || FromContext(nil) != nil || FromContext(plain) != nil {
		t.Fatal("optional tracker")
	}
}

func TestCancellationRemovesBlockedWorkWithoutFinish(t *testing.T) {
	tracker := New()
	ctx, cancel := context.WithCancel(WithTracker(context.Background(), tracker))
	_, finish := Begin(ctx, "Blocked", Foreground)
	defer finish()
	<-tracker.Changed()
	cancel()
	select {
	case <-tracker.Changed():
	case <-time.After(time.Second):
		t.Fatal("canceled work stuck")
	}
	if len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("still busy")
	}
}

func TestStablePriorityAndProgressBounds(t *testing.T) {
	tracker := New()
	root := WithTracker(context.Background(), tracker)
	first, done1 := Begin(root, "first", Background)
	defer done1()
	_, done2 := Begin(root, "second", Background)
	defer done2()
	Progress(first, -1, -1)
	Progress(first, 20, 10)
	s := tracker.Snapshot()
	if s.Operations[0].Label != "first" || s.Operations[0].Completed != 10 {
		t.Fatal(s)
	}
}
