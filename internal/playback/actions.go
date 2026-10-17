package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"podmutt/internal/player"
	"podmutt/internal/store"
)

func (e *Engine) PlayEpisode(ctx context.Context, id int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error { return e.load(ctx, id) })
	return err
}

func (e *Engine) load(ctx context.Context, id int64) error {
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
		created, err := e.factory(e.ctx)
		if err != nil {
			return e.fail(ctx, err)
		}
		e.player = created
		if e.player == nil {
			return e.fail(ctx, fmt.Errorf("playback: player factory returned nil"))
		}
	}
	e.active, e.ready = false, false
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
	if episode.Completed {
		if err := e.repo.SetCompleted(ctx, id, false); err != nil {
			return e.fail(ctx, err)
		}
	}
	if err := e.repo.Checkpoint(ctx, id, position); err != nil {
		return e.fail(ctx, err)
	}
	episode.PositionSeconds, episode.Completed = position, false
	e.state.Current, e.state.Status, e.state.Error = &episode, Playing, ""
	e.entryID, e.active = entryID, true
	remaining := make([]int64, 0, len(e.state.Queue))
	for _, item := range e.state.Queue {
		if item.ID != id {
			remaining = append(remaining, item.ID)
		}
	}
	if err := e.setQueue(ctx, remaining); err != nil {
		return e.fail(ctx, err)
	}
	return nil
}

func (e *Engine) Play(ctx context.Context) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
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
	})
	return err
}

func (e *Engine) Pause(ctx context.Context) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
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
	})
	return err
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
	_, err := e.submit(ctx, func(ctx context.Context) error { return e.setQueue(ctx, ids) })
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

func (e *Engine) setQueue(ctx context.Context, ids []int64) error {
	var queue []store.Episode
	var unique []int64
	seen := make(map[int64]bool)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		item, err := e.repo.Episode(ctx, id)
		if err != nil {
			return err
		}
		seen[id] = true
		unique, queue = append(unique, id), append(queue, item)
	}
	if err := e.repo.ReplaceQueue(ctx, unique); err != nil {
		return err
	}
	e.state.Queue = queue
	return nil
}

// MarkCompleted changes history, not transport or queue order.
func (e *Engine) MarkCompleted(ctx context.Context, id int64, completed bool) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if err := e.repo.SetCompleted(ctx, id, completed); err != nil {
			return err
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
	if poll && e.ready {
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
	if e.player != nil {
		err = errors.Join(err, e.player.Close())
		e.player = nil
	}
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
	if event.Name == "property-change" && event.Property == "time-pos" && string(event.Data) != "null" {
		var position float64
		if json.Unmarshal(event.Data, &position) == nil && position >= 0 && !math.IsNaN(position) && !math.IsInf(position, 0) {
			e.ready = true
			e.state.Current.PositionSeconds = position
		}
	}
}
