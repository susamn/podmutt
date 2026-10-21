package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/activity"
)

func TestActivityFixedRowAcrossLayoutsAndModals(t *testing.T) {
	now := time.Now()
	snapshot := activity.Snapshot{Revision: 2, Operations: []activity.Operation{
		{Label: "Searching Apple Podcasts…", Priority: activity.Foreground, Started: now.Add(-time.Second)},
		{Label: "Refreshing feeds…", Priority: activity.Background, Started: now.Add(-time.Second)},
	}}
	for _, size := range [][2]int{{30, 12}, {70, 20}, {100, 24}, {160, 44}} {
		for _, icons := range []string{"unicode", "plain", "nerd"} {
			m := resize(New(fixture()), size[0], size[1]).WithActivityClock(context.Background(), nil).WithActivity(snapshot).WithIcons(icons)
			m.activity.at = now
			idle := m.WithActivity(activity.Snapshot{Revision: 3})
			if rowContaining(t, m.View().Content, "Now playing") != rowContaining(t, idle.View().Content, "Now playing") {
				t.Fatal("activity moved panes")
			}
			for _, dialog := range []*Dialog{nil, {Title: "Search", Body: strings.Repeat("some text\n", 30), InputLabel: "Query", Hint: "Enter search"}} {
				view := ansi.Strip(m.WithDialog(dialog).View().Content)
				rows := strings.Split(view, "\n")
				if len(rows) != size[1] || !strings.Contains(rows[len(rows)-2], "Searching") {
					t.Fatalf("status hidden %v:\n%s", size, view)
				}
				for _, row := range rows {
					if ansi.StringWidth(row) > size[0] {
						t.Fatal("overflow", row)
					}
				}
				if icons == "plain" && !strings.Contains("|/-\\", string([]rune(rows[len(rows)-2])[0])) {
					t.Fatal("non-ASCII spinner")
				}
			}
		}
	}
	m := New(Data{}).WithActivity(snapshot)
	m.width = 120
	m.activity.at = now
	if !strings.Contains(m.statusLine(), "1 background") {
		t.Fatal(m.statusLine())
	}
	snapshot.Operations[1].Priority = activity.Foreground
	m = m.WithActivity(snapshot)
	if !strings.Contains(m.statusLine(), "1 more") {
		t.Fatal(m.statusLine())
	}
}

func TestActivityTickDelayCancellationAndStaleReplies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wait := func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	now := time.Now()
	s := activity.Snapshot{Revision: 4, Operations: []activity.Operation{{Label: "Refreshing feeds…", Started: now, Completed: 3, Total: 12}}}
	m := New(Data{Notice: "Ready"}).WithActivityClock(ctx, wait).WithActivity(s)
	m.width = 100
	var cmd tea.Cmd
	m, cmd = m.SyncActivity()
	if cmd == nil || strings.Contains(m.statusLine(), "Refreshing") {
		t.Fatal("no delay")
	}
	if _, next := m.SyncActivity(); next != nil {
		t.Fatal("duplicate timer")
	}
	generation := m.activity.generation
	m.activity.at = now.Add(time.Second)
	if !strings.Contains(m.statusLine(), "3/12") {
		t.Fatal("missing measurable progress")
	}
	updated, next := m.Update(cmd())
	m = updated.(Model)
	if next == nil || m.activity.frame != 1 {
		t.Fatal("not animated")
	}
	m = m.WithActivity(activity.Snapshot{Revision: 3})
	if len(m.activity.snapshot.Operations) != 1 {
		t.Fatal("stale snapshot applied")
	}
	m = m.WithActivity(activity.Snapshot{Revision: 5})
	m, cmd = m.SyncActivity()
	if cmd != nil || m.activity.running || !strings.Contains(m.statusLine(), "Ready") {
		t.Fatal("did not stop")
	}
	if _, cmd = m.Update(activityTick{generation}); cmd != nil {
		t.Fatal("stale tick restarted")
	}
	s.Revision = 6
	m = m.WithActivity(s)
	m, cmd = m.SyncActivity()
	cancel()
	if cmd() != nil {
		t.Fatal("canceled timer produced tick")
	}
	// A tick already queued before cancellation also cannot restart the timer.
	m, cmd = m.SyncActivity()
	if cmd != nil || m.activity.running {
		t.Fatal("canceled lifetime still running")
	}
}

func TestActivitySanitizesLabelsAndAlwaysShowsStatus(t *testing.T) {
	m := resize(New(Data{Notice: "Retry with R\x1b[2J\nnow"}), 30, 12)
	if !strings.Contains(strings.Split(ansi.Strip(m.View().Content), "\n")[10], "Retry with R") {
		t.Fatal("compact notice hidden")
	}
	m = m.WithActivity(activity.Snapshot{Operations: []activity.Operation{{Label: "Unsafe\x1b[2J\ntext", Started: time.Now().Add(-time.Second)}}})
	m.activity.at = time.Now()
	if strings.Contains(m.statusLine(), "\x1b") || strings.Contains(m.statusLine(), "\n") {
		t.Fatal("unsafe status")
	}
	m = m.WithActivity(activity.Snapshot{Revision: 1})
	m.data.Notice = ""
	if strings.TrimSpace(m.statusLine()) != "" {
		t.Fatal("idle row not blank")
	}
}
