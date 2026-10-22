package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/activity"
	"podmutt/internal/feedback"
)

func TestErrorsRemainAccessibleAcrossPanesLayoutsActivityAndModals(t *testing.T) {
	message := feedback.Retry(feedback.For("Apple Podcasts search", context.DeadlineExceeded), "Press R to retry, or b to open Library.")
	for _, size := range [][2]int{{30, 12}, {70, 20}, {140, 40}} {
		for _, panel := range []Panel{Podcasts, Episodes, Queue, Details, PlayerPanel} {
			m := resize(New(fixture()), size[0], size[1]).WithFocus(panel).WithFeedback(message).WithStatus("Refresh finished", "")
			m = m.WithActivity(activity.Snapshot{Operations: []activity.Operation{{Label: "Loading artwork…", Started: time.Now().Add(-time.Second)}}})
			m.activity.at = time.Now()
			m = m.WithBackgroundFeedback(feedback.Message{Severity: "warning", Summary: "Another feed failed."})
			view := ansi.Strip(m.View().Content)
			rows := strings.Split(view, "\n")
			if len(rows) != size[1] || !strings.Contains(rows[len(rows)-2], "Error · ! details") {
				t.Fatal(size, panel, view)
			}
			// Error status must remain outside input/settings overlays, without stealing focus.
			overlay := m.WithDialog(&Dialog{Title: "Search", Body: "Keep typing", InputLabel: "Query"})
			if !strings.Contains(ansi.Strip(overlay.View().Content), "Error · ! details") {
				t.Fatal("modal hid feedback")
			}
			overlay = press(overlay, "!")
			if overlay.messageOpen {
				t.Fatal("message key hijacked dialog")
			}
			m = press(m, "!")
			if !m.Busy() || !m.messageOpen || m.Focus() != panel {
				t.Fatal("message viewer hijacked selection")
			}
			for range 60 {
				m = press(m, "j")
			}
			if !strings.Contains(ansi.Strip(m.View().Content), "Library") {
				t.Fatal("recovery action unreachable", size, m.View().Content)
			}
			for _, row := range strings.Split(ansi.Strip(m.View().Content), "\n") {
				if ansi.StringWidth(row) > size[0] {
					t.Fatal("overflow", size, row)
				}
			}
			m = press(m, "k", "up", "pgup", "ctrl+u", "g")
			m = press(m, "esc")
			if m.messageOpen || m.Focus() != panel || m.message.Summary != "" {
				t.Fatal("dismiss did not return")
			}
		}
	}
}

func TestFeedbackScopeRecoveryAndReadingPosition(t *testing.T) {
	message := feedback.Message{Scope: "Feed preview", Severity: "error", Summary: strings.Repeat("Long safe explanation. ", 100), Action: "Press R to retry."}
	m := resize(New(Data{}), 30, 12).WithFeedback(message)
	m = press(m, "!", "j", "j")
	offset := m.messageScroll
	m = m.WithFeedback(message).WithStatus("Library updated", "").ClearFeedback("Playback")
	if m.messageScroll != offset || !m.messageOpen {
		t.Fatal("unrelated update reset reading")
	}
	m = m.ClearFeedback("Feed preview")
	if m.messageOpen || m.message.Summary != "" {
		t.Fatal("successful retry did not clear error")
	}
	m = press(m, "!")
	if m.messageOpen {
		t.Fatal("empty message opened")
	}
	m = m.WithBackgroundFeedback(feedback.Message{Severity: "warning", Summary: "Feed refresh failed."})
	if !strings.Contains(m.statusLine(), "Warning · ! details") {
		t.Fatal(m.statusLine())
	}
	m = press(m, "!")
	m = m.WithBackgroundFeedback(feedback.Message{})
	if m.messageOpen {
		t.Fatal("resolved background warning left blank modal")
	}
	m = m.WithFeedback(feedback.Message{Severity: "info", Summary: "Safe\x1b[2J\nnotice"})
	if line := m.statusLine(); strings.Contains(line, "\x1b") || strings.Contains(line, "\n") || !strings.Contains(line, "Notice") {
		t.Fatal(line)
	}
	m = press(m, "!")
	if !strings.Contains(ansi.Strip(m.View().Content), "Message") {
		t.Fatal("info viewer")
	}
	m = press(m, "!")
	if m.messageOpen {
		t.Fatal("! did not dismiss")
	}
}

func TestBackgroundWarningDoesNotHideForegroundActivity(t *testing.T) {
	m := resize(New(Data{}), 140, 40).WithBackgroundFeedback(feedback.Message{Severity: "warning", Summary: "Some feeds failed."})
	m = m.WithActivity(activity.Snapshot{Operations: []activity.Operation{{Label: "Searching Apple Podcasts…", Started: time.Now().Add(-time.Second)}}})
	m.activity.at = time.Now()
	if line := m.statusLine(); !strings.Contains(line, "Searching") || !strings.Contains(line, "! warning") {
		t.Fatal(line)
	}
}
