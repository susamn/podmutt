package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"podmutt/internal/player"
	"podmutt/internal/store"
)

func (e *Engine) PlayEpisode(ctx context.Context, id int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error { return e.loadSelected(ctx, id) })
	return err
}

func (e *Engine) loadSelected(ctx context.Context, id int64) error {
	if e.active && e.state.Current != nil && e.state.Current.ID == id && !e.state.Current.Completed {
		return e.play(ctx)
	}
	return e.loadStack(ctx, id, true)
}
func (e *Engine) load(ctx context.Context, id int64) error { return e.loadStack(ctx, id, false) }
func (e *Engine) loadStack(ctx context.Context, id int64, retainCurrent bool) error {
	episode, err := e.repo.Episode(ctx, id)
	if err != nil {
		return err
	}
	if err := e.save(ctx, true); err != nil {
		return e.fail(ctx, err)
	}
	if e.state.Current != nil && e.state.Current.ID == id && e.active {
		episode.PositionSeconds = e.state.Current.PositionSeconds
	}
	if e.player == nil {
		// Actor cancellation starts cleanup; it must not kill mpv before the
		// final Position/IPC-close/reap sequence. Abrupt process death still
		// closes the inherited socket, and Player.Close retains its kill bound.
		lifetime, cancel := context.WithCancel(context.WithoutCancel(e.ctx))
		e.playerCancel = cancel
		created, err := e.factory(lifetime)
		if err != nil {
			return e.fail(ctx, err)
		}
		e.player = created
		if e.player == nil {
			return e.fail(ctx, fmt.Errorf("playback: player factory returned nil"))
		}
	}
	e.active, e.ready = false, false
	e.seekUntil = time.Time{}
	position := episode.PositionSeconds
	if episode.Completed {
		position = 0
	}
	if err := e.player.Load(ctx, episode.AudioURL, position); err != nil {
		return e.fail(ctx, err)
	}
	entryID, err := e.player.EntryID(ctx)
	if err != nil {
		return e.fail(ctx, err)
	}
	remaining := make([]int64, 0, len(e.state.Queue)+1)
	if retainCurrent && e.state.Current != nil && e.state.Current.ID != id && !e.state.Current.Completed {
		remaining = append(remaining, e.state.Current.ID)
	}
	for _, item := range e.state.Queue {
		if item.ID != id {
			remaining = append(remaining, item.ID)
		}
	}
	pending, ids, err := e.resolveQueue(ctx, remaining, id)
	if err != nil {
		return e.fail(ctx, err)
	}
	if err := e.repo.Activate(ctx, id, position, ids); err != nil {
		return e.fail(ctx, err)
	}
	episode.PositionSeconds, episode.Completed = position, false
	e.state.Current, e.state.Status, e.state.Error = &episode, Playing, ""
	e.state.Queue = pending
	if e.removed != nil && e.removed.id == id {
		e.removed = nil
	}
	e.entryID, e.active = entryID, true
	return nil
}

// Toggle atomically resolves selection and transport; zero means current only.
func (e *Engine) Toggle(ctx context.Context, selectedID int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if selectedID != 0 && (e.state.Current == nil || e.state.Current.ID != selectedID) {
			return e.loadSelected(ctx, selectedID)
		}
		if e.state.Status == Playing {
			return e.pause(ctx)
		}
		return e.play(ctx)
	})
	return err
}
func (e *Engine) Play(ctx context.Context) error {
	_, err := e.submit(ctx, e.play)
	return err
}
func (e *Engine) play(ctx context.Context) error {
	if e.state.Current == nil {
		return ErrNoEpisode
	}
	if !e.active {
		return e.load(ctx, e.state.Current.ID)
	}
	if err := e.player.Play(ctx); err != nil {
		return e.fail(ctx, err)
	}
	e.state.Status = Playing
	return nil
}

func (e *Engine) Pause(ctx context.Context) error {
	_, err := e.submit(ctx, e.pause)
	return err
}
func (e *Engine) pause(ctx context.Context) error {
	if !e.active {
		return nil
	}
	if err := e.player.Pause(ctx); err != nil {
		return e.fail(ctx, err)
	}
	e.state.Status = Paused
	if err := e.save(ctx, true); err != nil {
		return e.fail(ctx, err)
	}
	return nil
}

func (e *Engine) stop(ctx context.Context) error {
	if err := e.save(ctx, true); err != nil {
		return e.fail(ctx, err)
	}
	if e.player != nil {
		if err := e.player.Stop(ctx); err != nil {
			return e.fail(ctx, err)
		}
	}
	e.active, e.ready, e.state.Status = false, false, Stopped
	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	_, err := e.submit(ctx, e.stop)
	return err
}

// Next skips without marking completed. Only the explicit queue advances.
func (e *Engine) Next(ctx context.Context) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if len(e.state.Queue) == 0 {
			return e.stop(ctx)
		}
		return e.load(ctx, e.state.Queue[0].ID)
	})
	return err
}

func (e *Engine) SetQueue(ctx context.Context, ids []int64) error {
	ids = append([]int64(nil), ids...)
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if err := e.setQueue(ctx, ids); err != nil {
			return err
		}
		e.removed = nil
		return nil
	})
	return err
}

func (e *Engine) Enqueue(ctx context.Context, ids ...int64) error {
	ids = append([]int64(nil), ids...)
	_, err := e.submit(ctx, func(ctx context.Context) error {
		var all []int64
		for _, item := range e.state.Queue {
			all = append(all, item.ID)
		}
		return e.setQueue(ctx, append(all, ids...))
	})
	return err
}

func (e *Engine) resolveQueue(ctx context.Context, ids []int64, currentID int64) ([]store.Episode, []int64, error) {
	var queue []store.Episode
	var unique []int64
	seen := make(map[int64]bool)
	for _, id := range ids {
		if seen[id] || id == currentID {
			continue
		}
		item, err := e.repo.Episode(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		seen[id] = true
		unique, queue = append(unique, id), append(queue, item)
	}
	return queue, unique, nil
}

func (e *Engine) setQueue(ctx context.Context, ids []int64) error {
	currentID := int64(0)
	if e.state.Current != nil && !e.state.Current.Completed {
		currentID = e.state.Current.ID
	}
	queue, unique, err := e.resolveQueue(ctx, ids, currentID)
	if err != nil {
		return err
	}
	if err := e.repo.ReplaceQueue(ctx, unique); err != nil {
		return err
	}
	e.state.Queue = queue
	e.invalidateRequeuedUndo()
	return nil
}

// MarkCompleted changes history, not transport or queue order.
func (e *Engine) MarkCompleted(ctx context.Context, id int64, completed bool) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if err := e.repo.SetCompleted(ctx, id, completed); err != nil {
			return err
		}
		if completed && e.removed != nil && e.removed.id == id {
			e.removed = nil
		}
		if e.state.Current != nil && e.state.Current.ID == id {
			e.state.Current.Completed = completed
		}
		for i := range e.state.Queue {
			if e.state.Queue[i].ID == id {
				e.state.Queue[i].Completed = completed
			}
		}
		return nil
	})
	return err
}

func (e *Engine) save(ctx context.Context, poll bool) error {
	if !e.active || e.state.Current == nil {
		return nil
	}
	if poll && e.ready && !time.Now().Before(e.seekUntil) {
		position, err := e.player.Position(ctx)
		if err != nil {
			return err
		}
		if position < 0 || math.IsNaN(position) || math.IsInf(position, 0) {
			return fmt.Errorf("playback: invalid position")
		}
		e.state.Current.PositionSeconds = position
	}
	return e.repo.Checkpoint(ctx, e.state.Current.ID, e.state.Current.PositionSeconds)
}

func (e *Engine) fail(ctx context.Context, err error) error {
	// A timed-out/canceled request must still silence the player. Closing IPC
	// is stronger than issuing Stop with an already-canceled context.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), operationTimeout)
	defer cancel()
	err = errors.Join(err, e.save(cleanup, false))
	err = errors.Join(err, e.closePlayer())
	e.active, e.ready = false, false
	e.state.Status, e.state.Error = Failed, err.Error()
	return err
}

func (e *Engine) event(ctx context.Context, event player.Event) {
	if event.Name == "start-file" {
		e.seen = event.PlaylistEntryID != nil
		if e.seen {
			e.seenID = *event.PlaylistEntryID
		}
		return
	}
	if !e.active || e.state.Current == nil {
		return
	}
	if event.Name == "end-file" {
		if event.PlaylistEntryID == nil || *event.PlaylistEntryID != e.entryID {
			return
		}
		switch event.Reason {
		case "eof":
			e.active, e.ready = false, false
			if err := e.repo.SetCompleted(ctx, e.state.Current.ID, true); err != nil {
				e.fail(ctx, err)
				return
			}
			e.state.Current.Completed = true
			if err := e.repo.Checkpoint(ctx, e.state.Current.ID, e.state.Current.PositionSeconds); err != nil {
				e.fail(ctx, err)
				return
			}
			e.state.Status = Stopped
			if len(e.state.Queue) > 0 {
				if err := e.load(ctx, e.state.Queue[0].ID); err != nil {
					e.fail(ctx, err)
				}
			}
		case "error":
			e.fail(ctx, fmt.Errorf("playback: stream failed"))
		case "stop", "quit":
			if err := e.save(ctx, false); err != nil {
				e.fail(ctx, err)
				return
			}
			e.active, e.ready, e.state.Status = false, false, Stopped
		}
		return
	}
	if !e.seen || e.seenID != e.entryID {
		return
	}
	if event.Name == "file-loaded" {
		e.ready = true
	}
	if event.Name == "property-change" && event.Property == "duration" {
		var duration float64
		if json.Unmarshal(event.Data, &duration) == nil && duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) && duration < float64(math.MaxInt64)/2 {
			e.state.Current.DurationSeconds = int64(math.Ceil(duration))
		}
		return
	}
	if event.Name == "property-change" && event.Property == "time-pos" && string(event.Data) != "null" {
		var position float64
		if json.Unmarshal(event.Data, &position) == nil && position >= 0 && !math.IsNaN(position) && !math.IsInf(position, 0) {
			if time.Now().Before(e.seekUntil) && math.Abs(position-e.seekTarget) > 0.5 {
				return
			}
			e.seekUntil = time.Time{}
			e.ready = true
			e.state.Current.PositionSeconds = position
		}
	}
}
