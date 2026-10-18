package playback

import (
	"context"
	"errors"
	"testing"
)

type lifetimePlayer struct {
	*fakePlayer
	lifetime  context.Context
	premature bool
}

func (p *lifetimePlayer) Position(ctx context.Context) (float64, error) {
	if err := p.lifetime.Err(); err != nil {
		return 0, err
	}
	return p.fakePlayer.Position(ctx)
}
func (p *lifetimePlayer) Close() error {
	if p.lifetime.Err() != nil {
		p.premature = true
		return errors.New("process killed before IPC cleanup")
	}
	return p.fakePlayer.Close()
}
func TestShutdownCheckpointsAndReapsBeforeProcessContextCancellation(t *testing.T) {
	for _, parentCancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "Close", true: "parent cancellation"}[parentCancel], func(t *testing.T) {
			db, p, episodes := setup(t)
			parent, cancel := context.WithCancel(background)
			defer cancel()
			wrapper := &lifetimePlayer{fakePlayer: p}
			e, err := New(parent, db, func(ctx context.Context) (Player, error) { wrapper.lifetime = ctx; return wrapper, nil })
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close(background)
			if err = e.PlayEpisode(background, episodes[0].ID); err != nil {
				t.Fatal(err)
			}
			ready(t, e, p, 10)
			p.setPosition(79) // final checkpoint must query the still-live player
			if parentCancel {
				cancel()
			}
			if err = e.Close(background); err != nil {
				t.Fatal(err)
			}
			saved, err := db.Current(background)
			if err != nil || saved.PositionSeconds != 79 {
				t.Fatal("final checkpoint lost", saved, err)
			}
			if wrapper.premature || wrapper.lifetime.Err() == nil || p.closed != 1 {
				t.Fatal("lifetime/reap ordering", wrapper.premature, p.closed)
			}
		})
	}
}
func TestFailedFactoryReleasesDetachedProcessContext(t *testing.T) {
	db, _, episodes := setup(t)
	var lifetime context.Context
	e, err := New(background, db, func(ctx context.Context) (Player, error) { lifetime = ctx; return nil, testFailure })
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close(background)
	if err = e.PlayEpisode(background, episodes[0].ID); err == nil {
		t.Fatal("factory failure ignored")
	}
	if lifetime == nil || lifetime.Err() == nil {
		t.Fatal("failed factory leaked lifetime")
	}
}
