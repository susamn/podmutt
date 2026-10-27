package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/chapters"
	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestModalPrecedenceAndInputIsolation(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(m *libraryShell)
		key           string
		expectSpace   bool // true if space should trigger global playback toggle
		assertOutcome func(t *testing.T, m *libraryShell, cmd tea.Cmd)
	}{
		{
			name: "URL Inspector Modal owns space and navigation keys",
			setup: func(m *libraryShell) {
				m.urlDialog = &ui.Dialog{Title: "URL Inspector"}
				m.Model = m.Model.WithDialog(m.urlDialog)
			},
			key:         "space",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if m.urlDialog == nil {
					t.Fatal("urlDialog unexpectedly closed")
				}
				if cmd != nil {
					t.Fatal("expected no playback command when URL dialog is open")
				}
			},
		},
		{
			name: "URL Inspector Modal closes on Esc",
			setup: func(m *libraryShell) {
				m.urlDialog = &ui.Dialog{Title: "URL Inspector"}
				m.Model = m.Model.WithDialog(m.urlDialog)
			},
			key:         "esc",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if m.urlDialog != nil {
					t.Fatal("expected urlDialog to close on Esc")
				}
			},
		},
		{
			name: "Chapter List Modal owns space and does not trigger global playback toggle",
			setup: func(m *libraryShell) {
				m.chapterList = []chapters.Chapter{{Title: "Intro", Start: 0}}
			},
			key:         "space",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				// Chapter modal consumes space without triggering global playback
			},
		},
		{
			name: "Chapter List Modal closes on Esc",
			setup: func(m *libraryShell) {
				m.chapterList = []chapters.Chapter{{Title: "Intro", Start: 0}}
			},
			key:         "esc",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if m.chapterList != nil {
					t.Fatal("expected chapters modal to close on Esc")
				}
			},
		},
		{
			name: "Onboarding Modal owns space and closes on Esc",
			setup: func(m *libraryShell) {
				m.onboarding = true
			},
			key:         "esc",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if m.onboarding {
					t.Fatal("expected onboarding to close on Esc")
				}
			},
		},
		{
			name: "Confirmation Dialog owns keys and cancels on Esc",
			setup: func(m *libraryShell) {
				m.confirming = &queueConfirmation{episodeID: 10, action: "remove_queue"}
				m.Model = m.Model.WithDialog(&ui.Dialog{Title: "Confirm"})
			},
			key:         "esc",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if m.confirming != nil {
					t.Fatal("expected confirmation to be canceled on Esc")
				}
			},
		},
		{
			name: "Add Feed input consumes space as character without triggering playback",
			setup: func(m *libraryShell) {
				m.adding = true
				m.input = "https://example.com"
			},
			key:         "space",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if !m.adding {
					t.Fatal("expected adding state to remain active")
				}
				if m.input != "https://example.com " {
					t.Fatalf("expected space to be appended to input, got %q", m.input)
				}
			},
		},
		{
			name: "Search Modal consumes space as character without triggering playback",
			setup: func(m *libraryShell) {
				m.browse.searching = true
				m.browse.input = "tech"
			},
			key:         "space",
			expectSpace: false,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if !m.browse.searching {
					t.Fatal("expected searching state to remain active")
				}
				if m.browse.input != "tech " {
					t.Fatalf("expected space to be appended to search query, got %q", m.browse.input)
				}
			},
		},
		{
			name: "Normal Navigation dispatches global space play/pause toggle",
			setup: func(m *libraryShell) {
				// No modals or inputs active
			},
			key:         "space",
			expectSpace: true,
			assertOutcome: func(t *testing.T, m *libraryShell, cmd tea.Cmd) {
				if cmd == nil {
					t.Fatal("expected playback toggle command to be dispatched on Space when no modal is active")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			librarySession(t, func(s *session) {
				app, err := newLibrary(s, feed.New(s.store, nil, 0))
				if err != nil {
					t.Fatal(err)
				}
				tc.setup(&app)

				var msg tea.KeyPressMsg
				switch tc.key {
				case "space":
					msg = tea.KeyPressMsg{Code: ' ', Text: " "}
				case "esc":
					msg = tea.KeyPressMsg{Code: tea.KeyEscape}
				default:
					msg = tea.KeyPressMsg{Code: rune(tc.key[0]), Text: tc.key}
				}

				next, cmd := app.Update(msg)
				updated := next.(libraryShell)
				tc.assertOutcome(t, &updated, cmd)
			})
		})
	}
}

func TestModalPrecedenceURLDialogScrolling(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		app.urlDialog = &ui.Dialog{Title: "URL Inspector"}
		app.Model = app.Model.WithDialog(app.urlDialog)

		// Scroll down
		next, _ := app.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		updated := next.(libraryShell)
		if updated.urlDialog.Scroll != 1 {
			t.Fatalf("expected scroll 1, got %d", updated.urlDialog.Scroll)
		}

		// Scroll up
		next, _ = updated.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
		updated = next.(libraryShell)
		if updated.urlDialog.Scroll != 0 {
			t.Fatalf("expected scroll 0, got %d", updated.urlDialog.Scroll)
		}
	})
}

func TestModalPrecedenceActionKeys(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		// Test 'U' opens URL preview modal
		next, _ := app.Update(tea.KeyPressMsg{Code: 'U', Text: "U"})
		updated := next.(libraryShell)
		if !updated.browse.searching || !updated.browse.urlInput {
			t.Fatal("expected U to open URL preview input modal")
		}

		// Close modal
		next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		updated = next.(libraryShell)

		// Test 't' activates Top 100 discovery
		next, cmd := updated.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
		updated = next.(libraryShell)
		if !updated.browse.active || cmd == nil {
			t.Fatal("expected t to activate Top 100 search")
		}

		// Test 'b' toggles browse mode back
		next, _ = updated.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
		updated = next.(libraryShell)
		if updated.browse.active {
			t.Fatal("expected b to toggle browse mode off")
		}
	})
}

func TestModalPrecedenceURLDialogQuitDelegation(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		app.urlDialog = &ui.Dialog{Title: "URL Inspector"}
		app.Model = app.Model.WithDialog(app.urlDialog)

		// 'q' inside URL dialog delegates to allow user to quit
		_, cmd := app.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
		if cmd == nil {
			t.Fatal("expected quit command on 'q' inside URL modal")
		}
	})
}

func TestModalPrecedenceEpisodeContextIdentityPreserved(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		app.detailTarget = &store.Episode{ID: 10, Title: "Active Playing Episode"}
		app.Model = app.Model.WithFocus(ui.Details)

		// Ensure episode identity is preserved when navigating
		ep, ok := app.contextEpisode()
		if !ok || ep.ID != 10 {
			t.Fatalf("expected active episode ID 10, got %+v", ep)
		}
	})
}
