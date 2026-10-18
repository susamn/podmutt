package playback

import "time"

type positionClock struct {
	id       int64
	position float64
	moving   bool
	at       time.Time
}

// anchorPosition is actor-only. Queue/metadata-only publications must not reset
// the clock to an old sample; transport/entry/position changes re-anchor it.
func (e *Engine) anchorPosition() {
	current := e.state.Current
	if current == nil {
		e.clock = positionClock{}
		e.state.ClockAt = time.Time{}
		return
	}
	moving := e.active && e.ready && e.state.Status == Playing
	clock := &e.clock
	if clock.id != current.ID || clock.position != current.PositionSeconds || clock.moving != moving {
		clock.id, clock.position, clock.moving = current.ID, current.PositionSeconds, moving
		clock.at = time.Time{}
		if moving {
			clock.at = time.Now()
		}
	}
	e.state.ClockAt = clock.at
}
