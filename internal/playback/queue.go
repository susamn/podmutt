package playback

import (
	"context"
	"errors"
	"podmutt/internal/feedback"
	"slices"

	"podmutt/internal/store"
)

var ErrQueueChanged = feedback.New(feedback.QueueChanged, "playback: queue changed; review it and try again")

func queueIDs(queue []store.Episode) []int64 {
	ids := make([]int64, len(queue))
	for i, item := range queue {
		ids[i] = item.ID
	}
	return ids
}

// MoveQueued edits the live queue by identity inside the actor. A stale UI must
// never reintroduce an entry consumed by EOF or overwrite concurrent enqueues.
func (e *Engine) MoveQueued(ctx context.Context, id int64, step int) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if step != -1 && step != 1 {
			return errors.New("playback: queue move must be one step")
		}
		ids := queueIDs(e.state.Queue)
		index := slices.Index(ids, id)
		if index < 0 {
			return ErrQueueChanged
		}
		target := index + step
		if target < 0 || target >= len(ids) {
			return nil
		}
		ids[index], ids[target] = ids[target], ids[index]
		return e.setQueue(ctx, ids)
	})
	return err
}

// RemoveQueued removes only captured identities; unrelated changes survive.
func (e *Engine) RemoveQueued(ctx context.Context, ids ...int64) error {
	ids = slices.Clone(ids)
	_, err := e.submit(ctx, func(ctx context.Context) error { return e.removeQueued(ctx, ids) })
	return err
}

func (e *Engine) removeQueued(ctx context.Context, ids []int64) error {
	keep := queueIDs(e.state.Queue)
	keep = slices.DeleteFunc(keep, func(id int64) bool { return slices.Contains(ids, id) })
	return e.setQueue(ctx, keep)
}

// ClearQueue refuses confirmation of a different queue, including a new order.
func (e *Engine) ClearQueue(ctx context.Context, expected []int64) error {
	expected = slices.Clone(expected)
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if !slices.Equal(queueIDs(e.state.Queue), expected) {
			return ErrQueueChanged
		}
		if err := e.setQueue(ctx, nil); err != nil {
			return err
		}
		e.removed = nil
		return nil
	})
	return err
}
