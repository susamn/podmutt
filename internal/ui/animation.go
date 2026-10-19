package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

type animationTick struct{ generation uint64 }
type animation struct {
	ctx        context.Context
	wait       func(context.Context, time.Duration) error
	cancel     context.CancelFunc
	generation uint64
	frame      int
	running    bool
}

// WithAnimation uses the foreground lifetime. A supplied wait function permits
// deterministic clock tests without sleeps; nil uses a cancelable timer.
func (m Model) WithAnimation(ctx context.Context, wait func(context.Context, time.Duration) error) Model {
	if wait == nil {
		wait = waitFrame
	}
	m.animation = animation{ctx: ctx, wait: wait}
	return m
}
func waitFrame(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m Model) animationVisible() bool {
	if !m.decoration || m.compact() || m.Busy() {
		return false
	}
	_, player := m.rightCellHeights()
	width := m.wideLayout().queueWidth
	return player-6-len(m.playerNotices(width-2)) >= 2
}

// SyncAnimation is called after transport changes and resize. Pausing preserves
// the frame, stop/failure clears it. At most one context-bound timer is pending.
func (m Model) SyncAnimation() (Model, tea.Cmd) {
	playing := m.data.Player.Status == "playing" && m.animationVisible() && m.animation.ctx != nil && m.animation.ctx.Err() == nil
	if m.data.Player.Status != "playing" && m.data.Player.Status != "paused" {
		m.animation.frame = 0
	}
	if !playing {
		if m.animation.cancel != nil {
			m.animation.cancel()
			m.animation.cancel = nil
		}
		m.animation.running = false
		m.animation.generation++
		return m, nil
	}
	if m.animation.running {
		return m, nil
	}
	m.animation.running = true
	return m.scheduleAnimation()
}
func (m Model) scheduleAnimation() (Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.animation.ctx)
	m.animation.cancel = cancel
	m.animation.generation++
	generation, wait := m.animation.generation, m.animation.wait
	return m, func() tea.Msg {
		defer cancel()
		if wait(ctx, 180*time.Millisecond) != nil {
			return nil
		}
		return animationTick{generation}
	}
}
func (m Model) animationFrame(width int) string {
	if !m.animationVisible() || (m.data.Player.Status != "playing" && m.data.Player.Status != "paused") {
		return ""
	}
	const pattern = " .:-=+*#*+=-:. "
	width = max(0, min(40, width))
	out := make([]byte, width)
	for i := range out {
		out[i] = pattern[(i+m.animation.frame)%len(pattern)]
	}
	return "Decorative · " + string(out)
}
