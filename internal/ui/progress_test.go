package ui

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestProgressClockInterpolatesBoundsAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Unix(1000, 0)
	calls := 0
	wait := func(ctx context.Context, delay time.Duration) error {
		calls++
		if delay != 250*time.Millisecond {
			t.Fatal(delay)
		}
		return ctx.Err()
	}
	player := Player{Status: "playing", HasCurrent: true, PositionSeconds: 10, DurationSeconds: 100, ClockAt: now}
	m := resize(New(Data{Player: player, Details: strings.Repeat("details\n", 100)}), 140, 44).WithProgress(ctx, func() time.Time { return now }, wait).WithFocus(Details)
	m.scrollDetails(3)
	offset := m.detailScroll
	m, cmd := m.SyncProgress()
	if cmd == nil {
		t.Fatal("no progress timer")
	}
	generation := m.progress.generation
	if _, duplicate := m.SyncProgress(); duplicate != nil {
		t.Fatal("duplicate timer")
	}
	now = now.Add(500 * time.Millisecond)
	next, again := m.Update(cmd())
	m = next.(Model)
	if again == nil || calls != 1 || m.displayPlayer().Progress != .105 || m.displayPlayer().Remaining != "1:30" {
		t.Fatal("interpolation", m.displayPlayer(), calls)
	}
	if m.detailScroll != offset || m.Focus() != Details {
		t.Fatal("progress moved Details")
	}
	if _, bad := m.Update(progressTick{generation: generation}); bad != nil {
		t.Fatal("stale timer rescheduled")
	}
	old := m.progress.cancel
	player.Status = "paused"
	player.PositionSeconds = 10.5
	player.ClockAt = time.Time{}
	m = m.WithPlayback(nil, player)
	m, cmd = m.SyncProgress()
	if cmd != nil || m.progress.running {
		t.Fatal("paused clock ticking")
	}
	if old == nil || again() != nil {
		t.Fatal("pending timer was not canceled")
	}
	now = now.Add(time.Hour)
	if got := m.displayPlayer().Position; got != "0:10" {
		t.Fatal("pause advanced", got)
	}
	player.Status = "playing"
	player.ClockAt = now
	m = m.WithPlayback(nil, player)
	m, cmd = m.SyncProgress()
	if cmd == nil {
		t.Fatal("resume did not restart timer")
	}
	stale := cmd
	now = now.Add(3 * time.Second)
	m, cmd = m.SyncProgress()
	if cmd != nil || m.displayPlayer().Progress != .125 {
		t.Fatal("stalled source extrapolated beyond 2 seconds", m.displayPlayer())
	}
	if stale() != nil {
		t.Fatal("stale wait not canceled")
	}
	player.PositionSeconds = 50
	player.ClockAt = now
	m = m.WithPlayback(nil, player)
	m, cmd = m.SyncProgress()
	if cmd == nil || m.displayPlayer().Progress != .5 {
		t.Fatal("seek/source correction ignored")
	}
	m = m.WithDialog(&Dialog{Title: "Modal"})
	m, cmd = m.SyncProgress()
	if cmd != nil {
		t.Fatal("hidden progress ticking")
	}
	m = m.WithDialog(nil)
	m, cmd = m.SyncProgress()
	if cmd == nil {
		t.Fatal("visible progress did not restart")
	}
	cancel()
	if cmd() != nil {
		t.Fatal("tick survived foreground cancellation")
	}
	m, cmd = m.SyncProgress()
	if cmd != nil || m.progress.running {
		t.Fatal("canceled timer restarted")
	}
}

func TestProgressUnknownDurationAndResponsiveRendering(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, size := range [][2]int{{140, 44}, {70, 20}, {30, 12}} {
		m := resize(New(Data{Player: Player{Status: "playing", HasCurrent: true, PositionSeconds: 42, ClockAt: now}}), size[0], size[1]).WithProgress(context.Background(), func() time.Time { return now.Add(time.Second) }, func(context.Context, time.Duration) error { return nil })
		m, _ = m.SyncProgress()
		unknown := m.displayPlayer()
		if unknown.Position != "0:43" || unknown.Duration != "--:--" || unknown.Progress != 0 || unknown.Remaining != "" {
			t.Fatal(unknown)
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("progress overflow", size, line)
			}
		}
		m = m.WithPlayback(nil, Player{Status: "playing", HasCurrent: true, PositionSeconds: 99, DurationSeconds: 100, ClockAt: now})
		m, cmd := m.SyncProgress()
		if cmd != nil || m.displayPlayer().Progress != 1 {
			t.Fatal("end bound", m.displayPlayer())
		}
		if !strings.Contains(progressLine(m.displayPlayer(), 80), "-0:00") {
			t.Fatal("remaining time missing")
		}
		m = m.WithProgress(nil, nil, nil)
		m, _ = m.SyncProgress()
	}
	for _, seconds := range []float64{math.NaN(), math.Inf(1), -10} {
		if progressTime(seconds) != "0:00" {
			t.Fatal("unsafe time", seconds)
		}
	}
	if progressTime(3661) != "1:01:01" {
		t.Fatal("hour format")
	}
	if line := progressLine(Player{Position: "0:01", Duration: "0:02", Progress: .33}, 80); !strings.Contains(line, "▏") && !strings.Contains(line, "▎") && !strings.Contains(line, "▍") && !strings.Contains(line, "▌") && !strings.Contains(line, "▋") && !strings.Contains(line, "▊") && !strings.Contains(line, "▉") {
		t.Fatal("fractional bar not rendered", line)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00"},
		{45, "0:45"},
		{59, "0:59"},
		{60, "1:00"},
		{3599, "59:59"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{4500, "1:15:00"},
		{math.NaN(), "0:00"},
		{math.Inf(1), "0:00"},
		{math.Inf(-1), "0:00"},
		{-10, "0:00"},
	}
	for _, tc := range cases {
		got := FormatDuration(tc.seconds)
		if got != tc.expected {
			t.Errorf("FormatDuration(%v) = %q; want %q", tc.seconds, got, tc.expected)
		}
	}
}

func TestProgressTickGuardsAndPromptLifecycle(t *testing.T) {
	now := time.Now()
	m := resize(New(Data{Player: Player{Status: "playing", HasCurrent: true, ClockAt: now, DurationSeconds: 100}}), 140, 44).WithProgress(context.Background(), func() time.Time { return now }, func(context.Context, time.Duration) error { return nil })
	next, cmd := m.Update(progressTick{})
	m = next.(Model)
	if cmd != nil {
		t.Fatal("unarmed tick scheduled")
	}
	m = m.WithStatus("", "input")
	m, cmd = m.SyncProgress()
	if cmd != nil {
		t.Fatal("prompt ticking")
	}
	m = m.WithStatus("", "")
	m, cmd = m.SyncProgress()
	if cmd == nil {
		t.Fatal("prompt dismissal did not start")
	}
	m = m.WithProgress(context.Background(), func() time.Time { return now }, func(context.Context, time.Duration) error { return context.Canceled })
	m, cmd = m.SyncProgress()
	if cmd == nil || cmd() != nil {
		t.Fatal("failed wait produced tick")
	}
}
