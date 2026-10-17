package playback

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"podmutt/internal/store"
)

type faultRepository struct {
	Repository
	mu    sync.Mutex
	fault string
}

func (r *faultRepository) set(op string) { r.mu.Lock(); r.fault = op; r.mu.Unlock() }
func (r *faultRepository) check(op string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fault == op {
		return testFailure
	}
	return nil
}
func (r *faultRepository) Queue(ctx context.Context) ([]store.Episode, error) {
	if err := r.check("queue"); err != nil {
		return nil, err
	}
	return r.Repository.Queue(ctx)
}
func (r *faultRepository) Current(ctx context.Context) (store.Playback, error) {
	if err := r.check("current"); err != nil {
		return store.Playback{}, err
	}
	return r.Repository.Current(ctx)
}
func (r *faultRepository) Episode(ctx context.Context, id int64) (store.Episode, error) {
	if err := r.check("episode"); err != nil {
		return store.Episode{}, err
	}
	return r.Repository.Episode(ctx, id)
}
func (r *faultRepository) Checkpoint(ctx context.Context, id int64, position float64) error {
	if err := r.check("checkpoint"); err != nil {
		return err
	}
	return r.Repository.Checkpoint(ctx, id, position)
}
func (r *faultRepository) SetCompleted(ctx context.Context, id int64, completed bool) error {
	if err := r.check("complete"); err != nil {
		return err
	}
	return r.Repository.SetCompleted(ctx, id, completed)
}
func (r *faultRepository) ReplaceQueue(ctx context.Context, ids []int64) error {
	if err := r.check("replace"); err != nil {
		return err
	}
	return r.Repository.ReplaceQueue(ctx, ids)
}

func TestConstructorAndFactoryFailures(t *testing.T) {
	db, p, items := setup(t)
	if err := db.Checkpoint(background, items[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := New(background, db, nil); err == nil {
		t.Fatal("nil factory accepted")
	}
	for _, op := range []string{"queue", "current", "episode"} {
		r := &faultRepository{Repository: db, fault: op}
		if _, err := New(background, r, func(context.Context) (Player, error) { return p, nil }); !errors.Is(err, testFailure) {
			t.Fatalf("%s initialization: %v", op, err)
		}
	}
	for _, factory := range []Factory{
		func(context.Context) (Player, error) { return nil, testFailure },
		func(context.Context) (Player, error) { var p *fakePlayer; return p, testFailure },
		func(context.Context) (Player, error) { return nil, nil },
	} {
		e, err := New(background, db, factory)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Play(background); err == nil {
			t.Fatal("bad factory accepted")
		}
		if state(t, e).Status != Failed {
			t.Fatal("factory error not visible")
		}
		select {
		case snapshot := <-e.Updates():
			if snapshot.Status != Failed {
				t.Fatal("missing failure update")
			}
		default:
			t.Fatal("no state update")
		}
		e.Close(background)
	}
}

func TestPersistenceFailuresStopPlayback(t *testing.T) {
	for _, op := range []string{"checkpoint", "complete", "replace"} {
		t.Run(op, func(t *testing.T) {
			db, p, items := setup(t)
			r := &faultRepository{Repository: db}
			e, _ := engineFor(t, r, p)
			if op == "complete" {
				if err := db.SetCompleted(background, items[0].ID, true); err != nil {
					t.Fatal(err)
				}
			}
			r.set(op)
			if err := e.PlayEpisode(background, items[0].ID); err == nil {
				t.Fatal("persistence failure ignored")
			}
			if state(t, e).Status != Failed {
				t.Fatal("player not stopped after persistence failure")
			}
		})
	}
}

func TestBackgroundFailuresKeepQueue(t *testing.T) {
	for _, op := range []string{"checkpoint", "complete", "load"} {
		t.Run(op, func(t *testing.T) {
			db, p, items := setup(t)
			r := &faultRepository{Repository: db}
			e, ticks := engineFor(t, r, p)
			if err := e.Enqueue(background, items[1].ID); err != nil {
				t.Fatal(err)
			}
			if err := e.PlayEpisode(background, items[0].ID); err != nil {
				t.Fatal(err)
			}
			id := ready(t, e, p, 12)
			if op == "load" {
				p.setFailure(op)
			} else {
				r.set(op)
			}
			if op == "checkpoint" {
				ticks <- time.Now()
			} else {
				p.event("end-file", "eof", id)
			}
			eventually(t, func() bool { return state(t, e).Status == Failed })
			if len(state(t, e).Queue) != 1 {
				t.Fatal("failure consumed remaining queue")
			}
		})
	}
}

func TestManualMarkAndQueueFailureDoNotChangeTransport(t *testing.T) {
	db, p, items := setup(t)
	r := &faultRepository{Repository: db}
	e, _ := engineFor(t, r, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	r.set("complete")
	if err := e.MarkCompleted(background, items[0].ID, true); err == nil {
		t.Fatal("completion error ignored")
	}
	r.set("replace")
	if err := e.SetQueue(background, []int64{items[1].ID}); err == nil {
		t.Fatal("queue error ignored")
	}
	if state(t, e).Status != Playing || len(state(t, e).Queue) != 0 {
		t.Fatal("invalid edit changed playback")
	}
}

func TestLifetimeCancellationAndRepeatedClose(t *testing.T) {
	db, p, items := setup(t)
	ctx, cancel := context.WithCancel(background)
	e, err := New(ctx, db, func(context.Context) (Player, error) { return p, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := e.Close(background); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Snapshot(background); !errors.Is(err, ErrClosed) {
		t.Fatalf("request after shutdown: %v", err)
	}
	if err := e.Close(background); err != nil {
		t.Fatal(err)
	}
}
