package playback

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrSeekUnavailable = errors.New("playback: seeking unavailable until an episode is loaded")

type seeker interface {
	SeekTo(context.Context, float64) error
}

const seekCorrectionWindow = 2 * time.Second

func (e *Engine) SeekBy(ctx context.Context, delta float64) error { return e.seek(ctx, delta, true) }
func (e *Engine) SeekTo(ctx context.Context, position float64) error {
	return e.seek(ctx, position, false)
}
func (e *Engine) seek(ctx context.Context, value float64, relative bool) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("playback: seek must be finite")
	}
	_, err := e.submit(ctx, func(ctx context.Context) error {
		device, ok := e.player.(seeker)
		if !ok || !e.active || !e.ready || e.state.Current == nil {
			return ErrSeekUnavailable
		}
		// Consume observations already queued before this action, including EOF. A
		// delayed seek must never target the next episode after a natural advance.
		id, entry := e.state.Current.ID, e.entryID
		for range 128 {
			select {
			case event, open := <-e.player.Events():
				if !open {
					return ErrSeekUnavailable
				}
				e.event(ctx, event)
			default:
				goto drained
			}
			if !e.active || e.state.Current == nil || e.state.Current.ID != id || e.entryID != entry {
				return ErrSeekUnavailable
			}
		}
	drained:
		position := value
		if relative {
			base := e.state.Current.PositionSeconds
			if !time.Now().Before(e.seekUntil) {
				var err error
				base, err = e.player.Position(ctx)
				if err != nil {
					return err
				}
			}
			position = base + value
		}
		if math.IsNaN(position) || math.IsInf(position, 0) {
			return fmt.Errorf("playback: invalid seek position")
		}
		position = max(0, position)
		if duration := e.state.Current.DurationSeconds; duration > 0 {
			position = min(position, float64(duration))
		}
		if err := device.SeekTo(ctx, position); err != nil {
			e.state.Error = "Seek failed: " + err.Error()
			return err
		}
		e.state.Current.PositionSeconds = position
		e.seekTarget, e.seekUntil = position, time.Now().Add(seekCorrectionWindow)
		e.state.Error = ""
		if err := e.repo.Checkpoint(ctx, id, position); err != nil {
			return e.fail(ctx, err)
		}
		return nil
	})
	return err
}
