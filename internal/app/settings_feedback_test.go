package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/profile"
	"podmutt/internal/theme"
	"podmutt/internal/ui"
)

func TestSettingsPersistBeforeUIStateAndReportFailures(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for _, setting := range []string{"theme", "country"} {
			t.Run(fmt.Sprintf("%s/failure=%v", setting, fail), func(t *testing.T) {
				librarySession(t, func(s *session) {
					m := browserLibrary(t, s, &fixtureDirectory{})
					old := s.config
					if fail {
						if err := os.MkdirAll(s.paths.Config, 0700); err != nil {
							t.Fatal(err)
						}
					}
					m.Model = m.Model.WithSettings(ui.SettingsData{ActiveTheme: old.Theme, Themes: []string{"catppuccin"}}).OpenSettings()
					key := tea.KeyPressMsg{Code: tea.KeyEnter}
					if setting == "country" {
						key = tea.KeyPressMsg{Code: tea.KeyRight}
					}
					cmd := updateLibrary(&m, key)
					if cmd == nil {
						t.Fatal("no save command")
					}
					if again := updateLibrary(&m, key); again != nil {
						t.Fatal("duplicate write while save pending")
					}
					result := cmd()
					if s.config.Theme != old.Theme || s.config.Country != old.Country {
						t.Fatal("worker mutated UI state")
					}
					updateLibrary(&m, result)
					if fail {
						if s.config.Theme != old.Theme || s.config.Country != old.Country || !strings.Contains(m.notice, "Previous setting remains active") {
							t.Fatal("failed save lied", m.notice)
						}
						m.Model = m.Model.CloseSettings()
						libraryKey(&m, '!')
						if !strings.Contains(m.View().Content, "Error") {
							t.Fatal("failure inaccessible")
						}
					} else {
						cfg, err := profile.Load(s.paths.Config)
						if err != nil {
							t.Fatal(err)
						}
						if setting == "theme" && (cfg.Theme != "catppuccin" || s.config.Theme != "catppuccin") {
							t.Fatal("theme not persisted")
						}
						if setting == "country" && (cfg.Country == old.Country || cfg.Country != s.config.Country) {
							t.Fatal("country not persisted")
						}
					}
				})
			})
		}
	}
}

func TestThemeSignalDoesNotAcknowledgeAPendingOrFailedSave(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		m.Model = m.Model.WithSettings(ui.SettingsData{ActiveTheme: s.config.Theme, Themes: []string{"catppuccin"}}).OpenSettings()
		pending := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if pending == nil {
			t.Fatal("no pending save")
		}
		m.report("Save theme", errors.New("write failure"), "")
		updateLibrary(&m, themeReloadMsg{reload: true, themeName: s.config.Theme, palette: theme.Default()})
		if duplicate := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); duplicate != nil {
			t.Fatal("signal reset pending save guard")
		}
		if !strings.Contains(m.View().Content, "Save theme failed") {
			t.Fatal("signal cleared unrelated write failure")
		}
		updateLibrary(&m, pending())
		if s.config.Theme != "catppuccin" || strings.Contains(m.View().Content, "Save theme failed") {
			t.Fatal("actual acknowledgement did not resolve failure")
		}
	})
}

func TestThemeSignalCapturesCurrentChoiceAndDropsStaleReload(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		batch := updateLibrary(&m, themeSignal{})().(tea.BatchMsg)
		if len(batch) != 2 {
			t.Fatal("signal must rearm once and schedule finite reload")
		}
		reply := batch[1]().(themeReloadMsg)
		s.config.Theme = "catppuccin"
		m.Model = m.Model.WithTheme(theme.Catppuccin())
		if cmd := updateLibrary(&m, reply); cmd != nil || m.Model.Theme().Accent != theme.Catppuccin().Accent {
			t.Fatal("stale reload replaced new selection")
		}
		reply.themeName = "catppuccin"
		reply.palette = theme.Catppuccin()
		if cmd := updateLibrary(&m, reply); cmd != nil {
			t.Fatal("reload spawned extra signal subscription")
		}
	})
}
