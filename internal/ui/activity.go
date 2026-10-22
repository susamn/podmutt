package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/activity"
)

const statusRows = 2 // fixed activity/status row, then key hints
const activityDelay = 150 * time.Millisecond

type activityTick struct{ generation uint64 }
type activityView struct {
	snapshot   activity.Snapshot
	ctx        context.Context
	wait       func(context.Context, time.Duration) error
	cancel     context.CancelFunc
	generation uint64
	running    bool
	frame      int
	at         time.Time
}

func (m Model) WithActivityClock(ctx context.Context, wait func(context.Context, time.Duration) error) Model {
	if wait == nil {
		wait = waitFrame
	}
	m.activity.ctx, m.activity.wait = ctx, wait
	return m
}
func (m Model) WithActivity(snapshot activity.Snapshot) Model {
	if snapshot.Revision >= m.activity.snapshot.Revision {
		m.activity.snapshot = snapshot
	}
	return m
}
func (m Model) SyncActivity() (Model, tea.Cmd) {
	if len(m.activity.snapshot.Operations) == 0 || m.activity.ctx == nil || m.activity.ctx.Err() != nil {
		if m.activity.cancel != nil {
			m.activity.cancel()
			m.activity.cancel = nil
		}
		m.activity.running = false
		m.activity.generation++
		m.activity.at = time.Time{}
		m.activity.frame = 0
		return m, nil
	}
	if m.activity.running {
		return m, nil
	}
	m.activity.running = true
	return m.scheduleActivity()
}
func (m Model) scheduleActivity() (Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.activity.ctx)
	m.activity.cancel = cancel
	m.activity.generation++
	generation, wait := m.activity.generation, m.activity.wait
	return m, func() tea.Msg {
		defer cancel()
		if wait(ctx, 100*time.Millisecond) != nil {
			return nil
		}
		return activityTick{generation}
	}
}
func (m Model) statusLine() string {
	text := clean(m.data.Notice, false)
	// In minimum layouts, use the reserved row for a warning that cannot fit
	// inside Player. Never spend its last transport/identity row on extra copy.
	playerHeight, playerWidth := m.wideLayout().player, m.wideLayout().playerWidth
	if m.compact() {
		_, _, playerHeight = m.compactHeights()
		playerWidth = m.width
	}
	notices := m.playerNotices(playerWidth - 2)
	budget := max(0, playerHeight-5)
	if budget < len(notices) {
		text = strings.TrimSpace(clean(notices[budget], false))
	}
	// No time.Now in rendering: a context-bound tick publishes presentation time.
	var visible []activity.Operation
	for _, op := range m.activity.snapshot.Operations {
		if !m.activity.at.Before(op.Started.Add(activityDelay)) {
			visible = append(visible, op)
		}
	}
	if len(visible) > 0 {
		op := visible[0]
		frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
		if m.icons == "plain" {
			frames = []rune("|/-\\")
		}
		text = string(frames[m.activity.frame%len(frames)]) + " " + clean(op.Label, false)
		if op.Total > 0 {
			text += fmt.Sprintf(" %d/%d", op.Completed, op.Total)
		}
		if len(visible) > 1 {
			background := 0
			for _, other := range visible[1:] {
				if other.Priority == activity.Background {
					background++
				}
			}
			if background == len(visible)-1 {
				text += fmt.Sprintf(" · %d background", background)
			} else {
				text += fmt.Sprintf(" · %d more", len(visible)-1)
			}
		}
	}
	if message := m.feedbackStatus(); message != "" {
		if m.message.Summary != "" || len(visible) == 0 {
			text = message
		} else {
			text += " · ! warning"
		}
	}
	if strings.TrimSpace(text) == "" {
		return strings.Repeat(" ", m.width)
	}
	return padRight(ansi.Truncate(text, m.width, "…"), m.width)
}
