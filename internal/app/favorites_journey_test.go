package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/ui"
)

func TestFavoritesEndToEndUserJourney(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})

		// 1. Initial State: Favorites list is empty, Details shows no heart
		finishLibrary(&m, m.reload())
		episodes := m.rows
		if len(episodes) < 2 {
			t.Fatal("need at least 2 episodes in fixture")
		}
		target := episodes[0]

		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(target.ID))
		m.renderDetails()
		if strings.Contains(m.View().Content, "Favorited") {
			t.Fatal("initial details showed favorited")
		}

		// 2. Enqueue target episode into Queue
		finishLibrary(&m, libraryKey(&m, 'a'))
		state := syncPlayer(t, &m)
		if len(state.Queue) != 1 || state.Queue[0].ID != target.ID {
			t.Fatalf("enqueue failed: %+v", state)
		}

		// 3. Tab cycling order: Podcasts (0) -> Episodes (1) -> Queue (2) -> Favorites (4) -> Podcasts (0)
		m.Model = m.Model.WithFocus(ui.Podcasts)
		for _, want := range []ui.Panel{ui.Episodes, ui.Queue, ui.Favorites, ui.Podcasts} {
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab})
			if m.Model.Focus() != want {
				t.Fatalf("Tab want %v, got %v", want, m.Model.Focus())
			}
		}
		for _, want := range []ui.Panel{ui.Favorites, ui.Queue, ui.Episodes, ui.Podcasts} {
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			if m.Model.Focus() != want {
				t.Fatalf("Shift+Tab want %v, got %v", want, m.Model.Focus())
			}
		}

		// 4. Numeric shortcuts: 5 focuses Favorites, 6 focuses Player
		updateLibrary(&m, tea.KeyPressMsg{Code: '5', Text: "5"})
		if m.Model.Focus() != ui.Favorites {
			t.Fatalf("key '5' did not focus Favorites: %v", m.Model.Focus())
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: '6', Text: "6"})
		if m.Model.Focus() != ui.PlayerPanel {
			t.Fatalf("key '6' did not focus PlayerPanel: %v", m.Model.Focus())
		}

		// 5. 'm' outside Queue does nothing
		for _, p := range []ui.Panel{ui.Podcasts, ui.Episodes, ui.Details, ui.Favorites, ui.PlayerPanel} {
			m.Model = m.Model.WithFocus(p)
			if cmd := libraryKey(&m, 'm'); cmd != nil || m.confirming != nil {
				t.Fatalf("key 'm' in pane %v opened menu", p)
			}
		}

		// 6. Focus Queue: press 'm' on target episode
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(target.ID))
		libraryKey(&m, 'm')
		if m.confirming == nil || !strings.Contains(m.View().Content, "Episode Actions") || !strings.Contains(m.View().Content, "Mark as favorite") {
			t.Fatalf("expected Episode Actions menu:\n%s", m.View().Content)
		}

		// 7. Cancel with Esc
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.confirming != nil || m.Model.Busy() {
			t.Fatal("escape did not cancel menu")
		}
		if len(m.snapshot.Favorites) != 0 {
			t.Fatal("cancelled favorite added to snapshot")
		}

		// 8. Open menu again and confirm with Enter
		libraryKey(&m, 'm')
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if !strings.Contains(m.notice, "Added to favorites") {
			t.Fatalf("missing notice: %s", m.notice)
		}

		// Snapshot reloaded: Favorites panel contains the episode!
		if len(m.snapshot.Favorites) != 1 || m.snapshot.Favorites[0].ID != target.ID || !m.snapshot.FavoriteIDs[target.ID] {
			t.Fatalf("favorites snapshot mismatch: %+v, %+v", m.snapshot.Favorites, m.snapshot.FavoriteIDs)
		}

		// 9. Inspect episode: Details shows heart badge
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(target.ID))
		m.renderDetails()
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "♥ Favorited") {
			t.Fatalf("Details missing '♥ Favorited':\n%s", view)
		}

		// 10. Pressing 'm' in Queue again on already-favorited episode
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(target.ID))
		libraryKey(&m, 'm')
		if !strings.Contains(m.View().Content, "Already in favorites") {
			t.Fatalf("menu missing 'Already in favorites':\n%s", m.View().Content)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if !strings.Contains(m.notice, "Already in favorites") {
			t.Fatalf("unexpected notice for duplicate favorite: %s", m.notice)
		}

		// 11. Remove episode from Queue (d -> y) -> Queue is empty, but episode SURVIVES in Favorites!
		libraryKey(&m, 'd')
		finishLibrary(&m, libraryKey(&m, 'y'))
		syncPlayer(t, &m)
		if len(syncPlayer(t, &m).Queue) != 0 {
			t.Fatal("queue removal failed")
		}
		if len(m.snapshot.Favorites) != 1 || m.snapshot.Favorites[0].ID != target.ID {
			t.Fatal("queue removal deleted favorite")
		}

		// 12. Mark completed / unsubscribe: still in Favorites
		if err := s.store.SetCompleted(s.ctx, target.ID, true); err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetSubscribed(s.ctx, target.FeedID, false); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, m.reload())
		if len(m.snapshot.Favorites) != 1 || m.snapshot.Favorites[0].ID != target.ID {
			t.Fatal("completion/unsubscription deleted favorite")
		}

		// 13. Focus Favorites (panel 5) and play selected episode with Enter
		m.Model = m.Model.WithFocus(ui.Favorites).Select(ui.Favorites, fmt.Sprint(target.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if syncPlayer(t, &m).Current.ID != target.ID {
			t.Fatal("Enter in Favorites did not play episode")
		}

		// 14. Open Details from Favorites with 'i', Esc returns to Favorites
		m.Model = m.Model.WithFocus(ui.Favorites).Select(ui.Favorites, fmt.Sprint(target.ID))
		libraryKey(&m, 'i')
		if m.Model.Focus() != ui.Details || !strings.Contains(m.View().Content, "♥ Favorited") {
			t.Fatalf("Details not opened from Favorites:\n%s", m.View().Content)
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.Model.Focus() != ui.Favorites {
			t.Fatalf("Esc in Details did not return to Favorites: %v", m.Model.Focus())
		}

		// 15. In Favorites: press 'd' to un-favorite
		libraryKey(&m, 'd')
		if m.confirming == nil || !strings.Contains(m.View().Content, "Remove from favorites? y/n") {
			t.Fatalf("expected un-favorite confirmation dialog:\n%s", m.View().Content)
		}
		// Cancel with 'n'
		libraryKey(&m, 'n')
		if len(m.snapshot.Favorites) != 1 {
			t.Fatal("cancel un-favorited episode")
		}

		// Confirm with 'y'
		libraryKey(&m, 'd')
		finishLibrary(&m, libraryKey(&m, 'y'))
		if !strings.Contains(m.notice, "Removed from favorites") {
			t.Fatalf("unexpected notice: %s", m.notice)
		}
		if len(m.snapshot.Favorites) != 0 || m.snapshot.FavoriteIDs[target.ID] {
			t.Fatalf("favorite still present after removal: %+v", m.snapshot)
		}

		// Details no longer shows heart
		m.renderDetails()
		if strings.Contains(m.View().Content, "♥ Favorited") {
			t.Fatal("Details still showed favorited after removal")
		}
	})
}

func TestFavoritesGuardsAndEdgeCases(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})

		// 1. Empty queue: 'm' shows notice
		m.Model = m.Model.WithFocus(ui.Queue)
		libraryKey(&m, 'm')
		if !strings.Contains(m.notice, "Select an episode in Queue first") {
			t.Fatalf("expected notice for empty queue 'm', got %s", m.notice)
		}

		// 2. Queue editing lock blocks 'm'
		m.queueEditing = true
		if cmd := m.openQueueActions(); cmd != nil || m.confirming != nil {
			t.Fatal("queueEditing did not block openQueueActions")
		}
		if cmd := m.removeFavorite(); cmd != nil || m.confirming != nil {
			t.Fatal("queueEditing did not block removeFavorite")
		}
		m.queueEditing = false

		// 3. Empty favorites: removeFavorite returns nil
		m.Model = m.Model.WithFocus(ui.Favorites)
		if cmd := m.removeFavorite(); cmd != nil || m.confirming != nil {
			t.Fatal("empty favorites allowed removeFavorite")
		}

		// 4. Enqueue and test 'm' with '1' and 'y' confirmations
		episodes := m.rows
		target := episodes[0]
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(target.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))

		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(target.ID))
		libraryKey(&m, 'm')
		// Confirm with '1'
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: '1', Text: "1"}))
		if !strings.Contains(m.notice, "Added to favorites") {
			t.Fatalf("confirming with '1' failed: %s", m.notice)
		}

		// 5. Test error reporting for favoriteEdited
		updateLibrary(&m, favoriteEdited{err: errors.New("boom")})
		if !strings.Contains(m.notice, "boom") && !strings.Contains(m.View().Content, "Error") {
			t.Fatal("favoriteEdited error not reported")
		}

		// 6. Test 'q' and 'ctrl+c' in confirmationKey for favorite and unfavorite
		m.confirming = &queueConfirmation{episodeID: target.ID, action: "favorite"}
		if cmd := m.confirmationKey(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd == nil {
			t.Fatal("'q' did not trigger tea.Quit")
		}
		m.confirming = &queueConfirmation{episodeID: target.ID, action: "unfavorite"}
		if cmd := m.confirmationKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
			t.Fatal("ctrl+c did not trigger tea.Quit")
		}
		m.confirming = nil

		// 7. Enqueue from Favorites pane with 'a'
		m.Model = m.Model.WithFocus(ui.Favorites).Select(ui.Favorites, fmt.Sprint(target.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))
		state := syncPlayer(t, &m)
		if len(state.Queue) < 1 {
			t.Fatal("enqueue from Favorites failed")
		}
	})
}
