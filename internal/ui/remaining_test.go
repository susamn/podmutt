package ui

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestRemainingPercentIsBoundedAndUnknownIsNotInvented(t *testing.T) {
	for _, tc := range []struct {
		position, duration float64
		want               string
	}{
		{0, 100, "100%"}, {25, 100, "75%"}, {62, 120, "48%"}, {100, 100, "0%"}, {200, 100, "0%"}, {-20, 100, "100%"},
		{10, 0, "--"}, {0, -1, "--"}, {math.NaN(), 100, "--"}, {math.Inf(1), 100, "--"}, {math.Inf(-1), 100, "--"}, {0, math.NaN(), "--"}, {0, math.Inf(1), "--"},
	} {
		if got := FormatRemainingPercent(tc.position, tc.duration); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	for _, width := range []int{0, 1, 4, 7, 30, 35, 56, 100} {
		m := New(Data{})
		row := m.renderQueueRow(Item{ID: "long", Index: 123, PlainTitle: strings.Repeat("Title 界\x1b[2J ", 100), Channel: strings.Repeat("Show ", 100), DurationSeconds: 100, PositionSeconds: 25}, width, true)
		if ansi.StringWidth(row) > max(0, width-2) || strings.Contains(row, "\x1b[2J") {
			t.Fatal("unbounded/injected row", width, row)
		}
		if width >= 7 && !strings.HasSuffix(ansi.Strip(row), "75%") {
			t.Fatal("long identity pushed percentage out", width, row)
		}
	}
}

func TestCurrentPercentageUsesSameClockAndPauseAsPlayerWithoutMutatingQueue(t *testing.T) {
	at := time.Unix(1000, 0)
	anchor := at
	items := []Item{{ID: "current", PlainTitle: "Current identity", Current: true, PositionSeconds: 25, DurationSeconds: 100}, {ID: "pending", PlainTitle: "Saved work", PositionSeconds: 60, DurationSeconds: 120}}
	player := Player{HasCurrent: true, Status: "playing", PositionSeconds: 25, DurationSeconds: 100, ClockAt: anchor}
	m := resize(New(Data{Queue: items, Player: player}), 140, 40).WithProgress(context.Background(), func() time.Time { return at }, func(context.Context, time.Duration) error { return nil })
	check := func(want string) {
		t.Helper()
		if row := ansi.Strip(m.renderQueueRow(items[0], 60, false)); !strings.HasSuffix(row, want) {
			t.Fatal("current percentage disagrees with Player", row, want)
		}
	}
	check("75%")
	at = anchor.Add(time.Second)
	m, cmd := m.SyncProgress()
	if cmd == nil {
		t.Fatal("current clock was not armed")
	}
	check("74%")
	if row := ansi.Strip(m.renderQueueRow(items[1], 60, false)); !strings.HasSuffix(row, "50%") {
		t.Fatal("current clock changed pending saved progress", row)
	}
	if items[0].PositionSeconds != 25 || m.data.Player.PositionSeconds != 25 {
		t.Fatal("presentation changed source checkpoint")
	}
	player.Status = "paused"
	m = m.WithPlayback(items, player)
	m, cmd = m.SyncProgress()
	if cmd != nil {
		t.Fatal("paused percentage kept a clock")
	}
	check("75%")
	player.DurationSeconds = 0
	m = m.WithPlayback(items, player)
	check("--")
	player.DurationSeconds = 100
	for _, position := range []float64{math.NaN(), math.Inf(1)} {
		player.PositionSeconds = position
		m = m.WithPlayback(items, player)
		check("--")
	}
	for _, size := range [][2]int{{30, 12}, {70, 20}, {100, 24}, {140, 40}} {
		m = resize(m, size[0], size[1]).WithFocus(Queue)
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("percentage overflow after resize", size, line)
			}
		}
	}
}
