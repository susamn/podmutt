package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/feedback"
	"podmutt/internal/theme"
)

func TestPopupLabelsFollowThemeAndKeepUntrustedValuesPlain(t *testing.T) {
	for _, pal := range []theme.Palette{theme.Default(), theme.SolarizedDark()} {
		section, label := popupSection("Configuration:", pal), popupLabel("Audio:", pal)
		if ansi.Strip(section) != "Configuration:" || ansi.Strip(label) != "Audio:" ||
			!strings.Contains(section, "\x1b[") || !strings.Contains(label, "\x1b[") || section == label {
			t.Fatal("popup structural styles missing", section, label)
		}
		multiword := popupLabel("Active file:", pal)
		if ansi.Strip(multiword) != "Active file:" || strings.Count(multiword, "\x1b[") < 2 {
			t.Fatal("multiword label lost its emphasis", multiword)
		}
		if want := "Enter apply · Esc cancel"; ansi.Strip(popupHint(want, pal)) != want {
			t.Fatal("shortcut styling changed wording")
		}
		m := New(Data{}).WithTheme(pal)
		dialog := &Dialog{
			Title: "Episode Actions", Body: "An episode\x1b[2J\nA show\n\n1. Mark as favorite",
			ActionLabel: "1. Mark as favorite", Hint: "Enter apply · Esc cancel",
		}
		view := m.renderDialogModal(dialog, 60, 12)
		plain := ansi.Strip(view)
		if !strings.Contains(plain, "An episode") || !strings.Contains(plain, "1. Mark as favorite") ||
			!strings.Contains(plain, "Enter apply · Esc cancel") || strings.Contains(plain, "\x1b") {
			t.Fatal("unsafe or missing action content", plain)
		}
		if !strings.Contains(view, popupLabel("1. Mark as favorite", pal)) {
			t.Fatal("action row not emphasized", view)
		}
		if ordinary := m.renderDialogModal(&Dialog{Body: "1. Mark as favorite"}, 60, 8); strings.Contains(ordinary, popupLabel("1. Mark as favorite", pal)) {
			t.Fatal("unmarked text styled as trusted action")
		}
	}
}

func TestPopupSectionsAcrossSettingsChaptersHelpAndRecovery(t *testing.T) {
	m := resize(New(Data{}).WithSettings(SettingsData{
		ActiveTheme: "Default", Themes: []string{"Default"}, ConfigPath: "/tmp/config.toml",
		ThemePath: "/tmp/colors.toml", MPVPath: "/usr/bin/mpv", Country: "us", RefreshMin: 30, Icons: "plain",
	}), 100, 30)
	settings := m.OpenSettings().View().Content
	for _, label := range []string{"Themes", "Configuration:", "Active file:", "Config:", "Refresh:", "Icons:", "Country:", "MPV:"} {
		if !strings.Contains(ansi.Strip(settings), label) {
			t.Fatal("settings label omitted", label)
		}
	}
	if !strings.Contains(settings, "1;4mConfiguration:") || !strings.Contains(settings, "1;4mMPV:") {
		t.Fatal("settings labels not styled")
	}
	m = m.CloseSettings().WithChapters(&ChapterPicker{Episode: "Episode", Show: "Show", Rows: []ChapterRow{{Time: "0:00", Title: "Opening"}}})
	chapters := m.View().Content
	if !strings.Contains(ansi.Strip(chapters), "Episode: Episode") || !strings.Contains(ansi.Strip(chapters), "Show: Show") ||
		!strings.Contains(ansi.Strip(chapters), "NOW PLAYING") || !strings.Contains(chapters, "1;4mNOW") {
		t.Fatal("chapter identity labels missing", chapters)
	}
	m = m.WithChapters(nil).WithFeedback(feedback.Message{Severity: "error", Summary: "Failed to play.", Action: "Try again."})
	next, _ := m.Update(tea.KeyPressMsg{Code: '!'})
	m = next.(Model)
	recovery := m.View().Content
	if !strings.Contains(recovery, "1;4mIssue") || !strings.Contains(recovery, "1;4mNext") ||
		!strings.Contains(ansi.Strip(recovery), "Try again.") {
		t.Fatal("recovery labels or action missing", recovery)
	}
	m = m.WithFeedback(feedback.Message{})
	next, _ = m.Update(tea.KeyPressMsg{Code: '!'})
	m = next.(Model)
	next, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = next.(Model)
	if help := m.View().Content; !strings.Contains(help, "1;4mCtrl+U") {
		t.Fatal("help shortcut not styled", help)
	}
}
