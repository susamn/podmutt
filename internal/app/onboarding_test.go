package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/profile"
	"podmutt/internal/ui"
)

func TestFirstRunOnboardingDismissalAndPersistence(t *testing.T) {
	paths := isolatedProfile(t)
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	// Ensure fresh profile is NOT onboarded
	_ = os.Remove(filepath.Join(paths.State, "onboarded"))

	if err := withSession(profile.Defaults(), paths, func(s *session) error {
		refresher := feed.New(s.store, nil, 0)
		m, err := newLibrary(s, refresher)
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})
		if !m.onboarding {
			t.Fatal("expected onboarding to be active on fresh profile")
		}
		view := m.View().Content
		if !strings.Contains(view, "Welcome to podmutt") || !strings.Contains(view, "Quick Start") {
			t.Fatalf("expected onboarding dialog in view: %s", view)
		}

		// Non-dismissal keypresses are swallowed and do not dismiss onboarding
		updateLibrary(&m, tea.KeyPressMsg{Code: 'j', Text: "j"})
		if !m.onboarding || !strings.Contains(m.View().Content, "Welcome to podmutt") {
			t.Fatal("non-dismissal key dismissed onboarding")
		}

		// 'q' quits without persisting onboarded flag
		cmd := updateLibrary(&m, tea.KeyPressMsg{Code: 'q', Text: "q"})
		if cmd == nil || cmd() != tea.Quit() {
			t.Fatal("expected q to quit onboarding")
		}
		if s.paths.Onboarded() {
			t.Fatal("q should not mark profile as onboarded")
		}

		// Reopen on same profile: onboarding is still active
		m2, err := newLibrary(s, refresher)
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m2, tea.WindowSizeMsg{Width: 140, Height: 40})
		if !m2.onboarding {
			t.Fatal("expected onboarding to still be active after quit without dismissal")
		}

		// Dismiss with Enter
		updateLibrary(&m2, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m2.onboarding {
			t.Fatal("Enter should dismiss onboarding")
		}
		if !s.paths.Onboarded() {
			t.Fatal("dismissal should persist onboarded flag")
		}

		// Reopen on same profile: onboarding is now inactive
		m3, err := newLibrary(s, refresher)
		if err != nil {
			t.Fatal(err)
		}
		if m3.onboarding {
			t.Fatal("onboarding should not be active once dismissed")
		}
		if strings.Contains(m3.View().Content, "Welcome to podmutt") {
			t.Fatal("welcome modal should not appear once dismissed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestOnboardingDismissWithEscAndSpace(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEscape}, {Code: tea.KeySpace}} {
		t.Run(key.String(), func(t *testing.T) {
			paths := isolatedProfile(t)
			if err := paths.Prepare(); err != nil {
				t.Fatal(err)
			}
			_ = os.Remove(filepath.Join(paths.State, "onboarded"))

			if err := withSession(profile.Defaults(), paths, func(s *session) error {
				m, err := newLibrary(s, feed.New(s.store, nil, 0))
				if err != nil {
					t.Fatal(err)
				}
				if !m.onboarding {
					t.Fatal("expected onboarding")
				}
				updateLibrary(&m, key)
				if m.onboarding {
					t.Fatalf("%s should dismiss onboarding", key.String())
				}
				if !s.paths.Onboarded() {
					t.Fatal("expected onboarded to be persisted")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDetailsEmptyStateGuidance(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})
		m.Model = m.Model.WithFocus(ui.Details)
		view := m.View().Content
		// In Discover mode on fresh library: shows Apple Podcasts guidance
		if !strings.Contains(view, "Apple Podcasts") || !strings.Contains(view, "Enter a show to inspect") {
			t.Fatalf("expected discovery guidance in Details view: %s", view)
		}

		// Switch to My Library: shows library guidance
		finishLibrary(&m, m.switchBrowse())
		view = m.View().Content
		if !strings.Contains(view, "Select an episode") || !strings.Contains(view, "Quick keys") {
			t.Fatalf("expected library empty state guidance in Details view: %s", view)
		}
	})
}
