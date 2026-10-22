package playback

import (
	"context"
	"podmutt/internal/feedback"
	"slices"
)

var ErrNoQueueUndo = feedback.New(feedback.NoUndo, "playback: no queue removal to undo")

type queueRemoval struct {
	id, previous, next int64
}

func (e *Engine) invalidateRequeuedUndo() {
	if e.removed == nil {
		return
	}
	for _, item := range e.state.Queue {
		if item.ID == e.removed.id {
			e.removed = nil
			return
		}
	}
}

// RemoveQueuedUndoable records one successful pending-only removal inside the
// actor. A stale selection or failed write never replaces the previous undo.
func (e *Engine) RemoveQueuedUndoable(ctx context.Context, id int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error { return e.removeQueuedUndoable(ctx, id) })
	return err
}

func (e *Engine) removeQueuedUndoable(ctx context.Context, id int64) error {
	ids := queueIDs(e.state.Queue)
	index := slices.Index(ids, id)
	if index < 0 {
		return ErrQueueChanged
	}
	removed := &queueRemoval{id: id}
	if index > 0 {
		removed.previous = ids[index-1]
	}
	if index+1 < len(ids) {
		removed.next = ids[index+1]
	}
	if err := e.removeQueued(ctx, []int64{id}); err != nil {
		return err
	}
	e.removed = removed
	return nil
}

// RemoveQueueItemUndoable removes a confirmed identity from the visible stack.
// If it is current at execution time, save and stop before clearing the durable
// pointer. Never advance or mark complete. Undo restores pending work, not audio.
func (e *Engine) RemoveQueueItemUndoable(ctx context.Context, id int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if e.state.Current == nil || e.state.Current.ID != id {
			return e.removeQueuedUndoable(ctx, id)
		}
		if err := e.stop(ctx); err != nil {
			return err
		}
		if err := e.repo.ClearCurrent(ctx, id); err != nil {
			return err
		}
		removed := &queueRemoval{id: id}
		if len(e.state.Queue) > 0 {
			removed.next = e.state.Queue[0].ID
		}
		e.state.Current, e.state.Error = nil, ""
		e.removed = removed
		return nil
	})
	return err
}

// UndoQueueRemoval merges one item into the live queue, never restores a stale
// snapshot. Prefer its surviving next neighbor, then previous, else append.
// Explicit re-enqueue, playback of that item, or clear invalidates the undo.
func (e *Engine) UndoQueueRemoval(ctx context.Context) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if e.removed == nil {
			return ErrNoQueueUndo
		}
		removed := *e.removed
		ids := queueIDs(e.state.Queue)
		index := slices.Index(ids, removed.next)
		if index < 0 {
			index = slices.Index(ids, removed.previous)
			if index < 0 {
				index = len(ids)
			} else {
				index++
			}
		}
		return e.setQueue(ctx, slices.Insert(ids, index, removed.id))
	})
	return err
}
