package playback

import "context"

// QueueLatest reconciles subscription arrivals in actor order with manual queue
// edits. It never loads a player, changes the current item or starts playback.
func (e *Engine) QueueLatest(ctx context.Context, feedID int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		current := int64(0)
		if e.state.Current != nil {
			current = e.state.Current.ID
		}
		queue, err := e.repo.EnqueueLatest(ctx, current, feedID)
		if err != nil {
			return err
		}
		e.state.Queue = queue
		e.invalidateRequeuedUndo()
		return nil
	})
	return err
}
