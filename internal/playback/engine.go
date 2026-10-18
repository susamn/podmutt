// Package playback serializes player actions, history, and queue advancement.
// UI and CLI clients use this service rather than mutating playback independently.
package playback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"podmutt/internal/player"
	"podmutt/internal/store"
)

var ErrClosed = errors.New("playback: session closed")
var ErrNoEpisode = errors.New("playback: no current episode")

const operationTimeout = 3 * time.Second

type Status string

const (
	Stopped Status = "stopped"
	Playing Status = "playing"
	Paused  Status = "paused"
	Failed  Status = "failed"
)

type State struct {
	ClockAt time.Time // last moving position anchor; queue-only updates preserve it
	Current *store.Episode
	Queue   []store.Episode
	Status  Status
	Error   string
}

type Repository interface {
	Current(context.Context) (store.Playback, error)
	Episode(context.Context, int64) (store.Episode, error)
	Queue(context.Context) ([]store.Episode, error)
	Checkpoint(context.Context, int64, float64) error
	SetCompleted(context.Context, int64, bool) error
	ReplaceQueue(context.Context, []int64) error
	Activate(context.Context, int64, float64, []int64) error
	EnqueueLatest(context.Context, int64, int64) ([]store.Episode, error)
}

type Player interface {
	Load(context.Context, string, float64) error
	EntryID(context.Context) (int64, error)
	Play(context.Context) error
	Pause(context.Context) error
	Stop(context.Context) error
	Position(context.Context) (float64, error)
	Events() <-chan player.Event
	Err() error
	Close() error
}

type Factory func(context.Context) (Player, error)
type action func(context.Context) error

type request struct {
	ctx   context.Context
	apply action
	reply chan result
}
type result struct {
	state State
	err   error
}

type Engine struct {
	ctx          context.Context
	cancel       context.CancelFunc
	repo         Repository
	factory      Factory
	player       Player
	playerCancel context.CancelFunc
	state        State
	clock        positionClock
	entryID      int64
	seenID       int64
	seen         bool
	active       bool
	ready        bool
	requests     chan request
	updates      chan State
	done         chan struct{}
	watchers     subscriptions
	closeErr     error // published by closing done
}

// New restores state without starting mpv or autoplaying. Consume Updates for
// coalesced snapshots; completion events themselves are never coalesced.
func New(ctx context.Context, repo Repository, factory Factory) (*Engine, error) {
	ticker := time.NewTicker(5 * time.Second)
	e, err := newEngine(ctx, repo, factory, ticker.C, ticker.Stop)
	if err != nil {
		ticker.Stop()
	}
	return e, err
}

func newEngine(ctx context.Context, repo Repository, factory Factory, ticks <-chan time.Time, stop func()) (*Engine, error) {
	if factory == nil {
		return nil, fmt.Errorf("playback: player factory required")
	}
	queue, err := repo.Queue(ctx)
	if err != nil {
		return nil, err
	}
	state := State{Status: Stopped, Queue: queue}
	current, err := repo.Current(ctx)
	if err == nil {
		episode, lookupErr := repo.Episode(ctx, current.EpisodeID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		state.Current = &episode
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// Legacy queues could contain the current entry as well as the current
	// pointer. Normalize that representation without a schema migration/autoplay.
	if state.Current != nil && !state.Current.Completed {
		pending := make([]store.Episode, 0, len(state.Queue))
		for _, item := range state.Queue {
			if item.ID != state.Current.ID {
				pending = append(pending, item)
			}
		}
		if len(pending) != len(state.Queue) {
			if err := repo.ReplaceQueue(ctx, queueIDs(pending)); err != nil {
				return nil, err
			}
			state.Queue = pending
		}
	}
	lifetime, cancel := context.WithCancel(ctx)
	e := &Engine{ctx: lifetime, cancel: cancel, repo: repo, factory: factory, state: state,
		requests: make(chan request), updates: make(chan State, 1), done: make(chan struct{})}
	go e.run(ticks, stop)
	return e, nil
}

func (e *Engine) run(ticks <-chan time.Time, stop func()) {
	defer close(e.done)
	defer close(e.updates)
	defer e.closeWatchers()
	defer stop()
	e.publish()
	for {
		var events <-chan player.Event
		if e.player != nil {
			events = e.player.Events()
		}
		select {
		case <-e.ctx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
			e.closeErr = e.save(ctx, true)
			e.closeErr = errors.Join(e.closeErr, e.closePlayer())
			cancel()
			return
		case req := <-e.requests:
			ctx, cancel := context.WithTimeout(req.ctx, operationTimeout)
			stopCancel := context.AfterFunc(e.ctx, cancel)
			var err error
			if err = ctx.Err(); err == nil && req.apply != nil {
				err = req.apply(ctx)
				e.publish()
			}
			stopCancel()
			cancel()
			req.reply <- result{e.snapshot(), err}
		case event, ok := <-events:
			ctx, cancel := context.WithTimeout(e.ctx, operationTimeout)
			if ok {
				e.event(ctx, event)
			} else {
				err := e.player.Err()
				if err == nil {
					err = player.ErrDisconnected
				}
				e.fail(ctx, err)
			}
			cancel()
			e.publish()
		case <-ticks:
			ctx, cancel := context.WithTimeout(e.ctx, operationTimeout)
			if err := e.save(ctx, false); err != nil {
				e.fail(ctx, err)
				e.publish()
			}
			cancel()
		}
	}
}

// Only the actor calls closePlayer. Cancel the private process lifetime AFTER
// orderly IPC shutdown/reap, including failed-start and nil-factory cleanup.
func (e *Engine) closePlayer() error {
	var err error
	if e.player != nil {
		err = e.player.Close()
		e.player = nil
	}
	if e.playerCancel != nil {
		e.playerCancel()
		e.playerCancel = nil
	}
	return err
}

func (e *Engine) snapshot() State {
	state := e.state
	if state.Current != nil {
		current := *state.Current
		state.Current = &current
	}
	state.Queue = append([]store.Episode(nil), state.Queue...)
	return state
}

func (e *Engine) publish() {
	e.anchorPosition()
	offer(e.updates, e.snapshot())
	e.publishWatchers()
}

func (e *Engine) submit(ctx context.Context, apply action) (State, error) {
	req := request{ctx: ctx, apply: apply, reply: make(chan result, 1)}
	select {
	case e.requests <- req:
	case <-e.done:
		return State{}, ErrClosed
	case <-ctx.Done():
		return State{}, ctx.Err()
	}
	select {
	case result := <-req.reply:
		return result.state, result.err
	case <-e.done:
		return State{}, ErrClosed
	case <-ctx.Done():
		return State{}, ctx.Err()
	}
}

func (e *Engine) Snapshot(ctx context.Context) (State, error) { return e.submit(ctx, nil) }
func (e *Engine) Updates() <-chan State                       { return e.updates }

func (e *Engine) Close(ctx context.Context) error {
	e.cancel()
	select {
	case <-e.done:
		return e.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
