package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/feedback"
	"podmutt/internal/theme"
)

func assertModalFrame(t *testing.T, rendered string, width, maxHeight int, wants ...string) {
	t.Helper()
	plain := ansi.Strip(rendered)
	rows := strings.Split(plain, "\n")
	if len(rows) > maxHeight {
		t.Fatalf("modal height %d exceeds %d:\n%s", len(rows), maxHeight, plain)
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != width {
			t.Fatalf("modal row width %d, want %d: %q", ansi.StringWidth(row), width, row)
		}
	}
	for _, want := range wants {
		if !strings.Contains(plain, want) {
			t.Fatalf("modal missing %q:\n%s", want, plain)
		}
	}
}

func TestUnifiedModalDesignForDialogsSettingsHelpAndFeedback(t *testing.T) {
	m := New(Data{}).WithTheme(theme.Default()).WithSettings(SettingsData{
		ActiveTheme: "catppuccin", Themes: []string{"catppuccin", "solarized", "cyberpunk"},
		ConfigPath: "/public/config.toml", ThemePath: "/public/theme.toml", MPVPath: "mpv",
		RefreshMin: 30, Icons: "unicode", Country: "us",
	})

	dialog := m.renderDialogModal(&Dialog{
		Title: "Remove episode?", Body: "Episode title\nPodcast show\nProgress/history kept.",
		Hint: "y remove · n/Esc cancel", TruncateBody: true,
	}, 64, 18)
	assertModalFrame(t, dialog, 64, 18, "Remove episode?", "DETAILS", "Episode title", "y remove · n/Esc cancel")
	if !strings.Contains(ansi.Strip(dialog), "│  DETAILS") {
		t.Fatal("dialog section lacks shared left padding", ansi.Strip(dialog))
	}

	input := m.renderDialogModal(&Dialog{Title: "Find podcasts", Body: "Search subscriptions", InputLabel: "Query", InputValue: "science", Hint: "Enter search · Esc cancel"}, 64, 18)
	assertModalFrame(t, input, 64, 18, "DETAILS", "INPUT", "Query:  science▌", "Enter search · Esc cancel")

	settings := m.renderSettingsModal(64, 18)
	assertModalFrame(t, settings, 64, 18, "Settings", "APPEARANCE", "CONFIGURATION", "Country", "j/k select · Enter apply")
	if !strings.Contains(settings, "\x1b[") {
		t.Fatal("settings selection lost visual highlight")
	}

	help := m.renderHelpModal(64, 22)
	assertModalFrame(t, help, 64, 22, "Keys", "PLAYBACK", "EPISODES", "NAVIGATION", "QUEUE", "GENERAL", "FAVORITES", "Space", "Pause / resume", "a", "Enqueue", "A", "Enqueue visible", "? or Esc close")

	m = m.WithFeedback(feedback.Message{Scope: "Queue edit", Severity: "error", Summary: "Could not remove episode.", Action: "Retry or check the database file."})
	m.messageOpen = true
	errorModal := m.feedbackModal(64, 18)
	assertModalFrame(t, errorModal, 64, 18, "Error", "ACTION REQUIRED", "Could not remove episode.", "! or Esc dismiss")
}

func TestUnifiedModalsRemainUsableAtMinimumTerminal(t *testing.T) {
	m := resize(New(Data{}).WithTheme(theme.Default()).WithSettings(SettingsData{
		ActiveTheme: "one", Themes: []string{"one", "two", "three"}, Country: "us",
	}), 30, 12)
	available := m.height - statusRows
	width, height := min(72, m.width-2), min(22, available-2)

	cases := []struct{ name, rendered, recovery string }{
		{"confirmation", m.renderDialogModal(&Dialog{Title: "Remove?", Body: strings.Repeat("Long episode title ", 20), Hint: "y remove · Esc cancel", TruncateBody: true}, width, height), "Esc cancel"},
		{"input", m.renderPromptModal("https://example.test/feed", width, height), "Esc cancel"},
		{"settings", m.renderSettingsModal(width, height), "Esc close"},
		{"help", m.renderHelpModal(width, height), "Esc close"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertModalFrame(t, tc.rendered, width, height, tc.recovery) })
	}

	m = m.WithFeedback(feedback.Message{Scope: "Test", Severity: "warning", Summary: strings.Repeat("Warning details ", 20), Action: "Retry later."})
	m.messageOpen = true
	assertModalFrame(t, m.feedbackModal(width, height), width, height, "Esc dismiss")
}
