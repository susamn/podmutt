package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDecorativeAnimationClockPauseStopResizeAndStaleTicks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := 0
	var pending context.Context
	m := resize(New(Data{Player: Player{Status: "playing", HasCurrent: true}}), 140, 44).WithPlayerDecoration(true).WithAnimation(ctx, func(ctx context.Context, _ time.Duration) error { waits++; pending = ctx; return ctx.Err() })
	m, cmd := m.SyncAnimation()
	if cmd == nil {
		t.Fatal("no clock")
	}
	tick := cmd()
	next, cmd := m.Update(tick)
	m = next.(Model)
	if m.animation.frame != 1 || waits != 1 || cmd == nil || !strings.Contains(ansi.Strip(m.View().Content), "Decorative") {
		t.Fatal("frame not advanced")
	}
	frozen := m.animationFrame(20)
	stale := cmd
	m = m.WithPlayback(nil, Player{Status: "paused", HasCurrent: true})
	m, cmd = m.SyncAnimation()
	if cmd != nil || m.animationFrame(20) != frozen {
		t.Fatal("pause didn't freeze")
	}
	next, cmd = m.Update(stale())
	m = next.(Model)
	if cmd != nil || m.animation.frame != 1 {
		t.Fatal("stale tick advanced paused animation")
	}
	m = m.WithPlayback(nil, Player{Status: "playing", HasCurrent: true})
	m, cmd = m.SyncAnimation()
	if cmd == nil {
		t.Fatal("resume failed")
	}
	next, resizeCmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	if resizeCmd != nil || m.animation.running || m.animationFrame(20) != "" {
		t.Fatal("compact animation active")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, cmd = m.Update(tea.WindowSizeMsg{Width: 140, Height: 44})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("wide didn't resume")
	}
	m = m.WithPlayback(nil, Player{Status: "stopped"})
	m, _ = m.SyncAnimation()
	if m.animation.frame != 0 || m.animationFrame(20) != "" {
		t.Fatal("stop didn't clear")
	}
	if pending == nil {
		t.Fatal("injected clock not used")
	}
	m = m.WithPlayback(nil, Player{Status: "playing"})
	m, cmd = m.SyncAnimation()
	cancel()
	next, cmd = m.Update(cmd())
	m = next.(Model)
	if cmd != nil {
		t.Fatal("shutdown scheduled tick")
	}
	m, _ = m.SyncAnimation()
	if m.animation.running {
		t.Fatal("shutdown running flag")
	}
}
func TestRealAnimationWaitCancellationAndIdle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(waitFrame(ctx, time.Hour), context.Canceled) {
		t.Fatal("timer ignored cancel")
	}
	if err := waitFrame(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	m := resize(New(Data{}), 140, 44).WithPlayerDecoration(true).WithAnimation(context.Background(), nil)
	if _, cmd := m.SyncAnimation(); cmd != nil {
		t.Fatal("idle ticking")
	}
	m = m.WithPlayback(nil, Player{Status: "playing"})
	m, cmd := m.SyncAnimation()
	if cmd == nil {
		t.Fatal("not started")
	}
	if _, again := m.SyncAnimation(); again != nil {
		t.Fatal("duplicate clock")
	}
	m.animation.cancel()
	if cmd() != nil {
		t.Fatal("canceled timer emitted frame")
	}
}
