package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/activity"
)

type activityChanged struct{ snapshot activity.Snapshot }

// Only finite work belongs here. Subscription waits, clocks, terminal probes,
// and steady-state audio playback must not make the application look busy.
func work(ctx context.Context, label string, run func(context.Context) tea.Msg) tea.Cmd {
	return trackedWork(ctx, label, activity.Foreground, run)
}
func trackedWork(ctx context.Context, label string, priority activity.Priority, run func(context.Context) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		ctx, finish := activity.Begin(ctx, label, priority)
		defer finish()
		return run(ctx)
	}
}
func commandLabel(command string) string {
	if strings.HasPrefix(command, "theme:") {
		return "Changing theme…"
	}
	if strings.HasPrefix(command, "country:") {
		return "Changing discovery country…"
	}
	switch command {
	case "seek-forward", "seek-backward":
		return "Seeking…"
	case "play":
		return "Starting playback…"
	case "pause":
		return "Pausing playback…"
	case "stop":
		return "Stopping playback…"
	case "next":
		return "Starting next episode…"
	default:
		return "Updating playback…"
	}
}
func (m shell) waitActivity() tea.Cmd {
	tracker := activity.FromContext(m.s.ctx)
	if tracker == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case <-m.s.ctx.Done():
			return nil
		case <-tracker.Changed():
			return activityChanged{tracker.Snapshot()}
		}
	}
}
