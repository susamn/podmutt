package ui

import (
	"context"
	"fmt"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
)

const progressHorizon = 2 * time.Second
const progressInterval = 250 * time.Millisecond

type progressTick struct{ generation uint64 }
type progressClock struct {
	ctx        context.Context
	now        func() time.Time
	wait       func(context.Context, time.Duration) error
	at         time.Time
	cancel     context.CancelFunc
	generation uint64
	running    bool
}

// WithProgress owns one cancelable presentation timer. It never polls mpv or
// writes checkpoints. Supplied clock/wait make ticks deterministic in tests.
func (m Model) WithProgress(ctx context.Context, now func() time.Time, wait func(context.Context, time.Duration) error) Model {
	if m.progress.cancel != nil {
		m.progress.cancel()
	}
	if now == nil {
		now = time.Now
	}
	if wait == nil {
		wait = waitFrame
	}
	m.progress = progressClock{ctx: ctx, now: now, wait: wait, at: now()}
	return m
}
func (m Model) SyncProgress() (Model, tea.Cmd) {
	clock := &m.progress
	if clock.now == nil {
		return m, nil
	}
	clock.at = clock.now()
	player := m.data.Player
	age := clock.at.Sub(player.ClockAt)
	moving := player.HasCurrent && player.Status == "playing" && !player.ClockAt.IsZero() && age >= 0 && age < progressHorizon
	if player.DurationSeconds > 0 && player.PositionSeconds+max(0, min(age.Seconds(), progressHorizon.Seconds())) >= player.DurationSeconds {
		moving = false
	}
	moving = moving && m.width >= 30 && m.height >= 12 && !m.Busy() && m.data.Prompt == "" && clock.ctx != nil && clock.ctx.Err() == nil
	if !moving {
		if clock.cancel != nil {
			clock.cancel()
			clock.cancel = nil
		}
		clock.running = false
		clock.generation++
		return m, nil
	}
	if clock.running {
		return m, nil
	}
	clock.running = true
	clock.generation++
	generation, wait := clock.generation, clock.wait
	ctx, cancel := context.WithCancel(clock.ctx)
	clock.cancel = cancel
	return m, func() tea.Msg {
		defer cancel()
		if wait(ctx, progressInterval) != nil {
			return nil
		}
		return progressTick{generation}
	}
}
func (m Model) displayPlayer() Player {
	player := m.data.Player
	if player.ClockAt.IsZero() && player.PositionSeconds == 0 && player.DurationSeconds == 0 {
		return player
	}
	position := player.PositionSeconds
	if math.IsNaN(position) || math.IsInf(position, 0) {
		position = 0
	}
	position = max(0, position)
	if player.Status == "playing" && !player.ClockAt.IsZero() && !m.progress.at.IsZero() {
		position += max(0, min(m.progress.at.Sub(player.ClockAt).Seconds(), progressHorizon.Seconds()))
	}
	duration := player.DurationSeconds
	if math.IsNaN(duration) || math.IsInf(duration, 0) {
		duration = 0
	}
	player.Duration = "--:--"
	player.Remaining = ""
	player.Progress = 0
	if duration > 0 {
		position = min(position, duration)
		player.Duration = progressTime(duration)
		player.Remaining = progressTime(math.Ceil(max(0, duration-position)))
		player.Progress = position / duration
	}
	player.Position = progressTime(position)
	return player
}
func FormatDuration(seconds float64) string {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		seconds = 0
	}
	value := int64(max(0, min(seconds, float64(math.MaxInt64)/2)))
	if value >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", value/3600, value/60%60, value%60)
	}
	return fmt.Sprintf("%d:%02d", value/60, value%60)
}

func progressTime(seconds float64) string {
	return FormatDuration(seconds)
}
