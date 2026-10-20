package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/playback"
	"podmutt/internal/ui"
)

func assertEpisodeMembership(t *testing.T, m libraryShell, id int64, queued, current bool) {
	t.Helper()
	for _, item := range m.Model.Visible(ui.Episodes) {
		if item.ID == fmt.Sprint(id) {
			if item.Queued != queued || item.Current != current {
				t.Fatalf("episode %d: queued=%t current=%t, want %t/%t", id, item.Queued, item.Current, queued, current)
			}
			return
		}
	}
	t.Fatal("episode not visible", id)
}

func TestEnterImmediatelyMarksCurrentAndPreviousWithoutStreamOrNavigation(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		first, second := m.rows[0], m.rows[1]
		assertEpisodeMembership(t, m, first.ID, false, false)
		var previous playback.State
		for i, episode := range m.rows[:2] {
			m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(episode.ID))
			cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Enter did not issue play")
			}
			reply, ok := cmd().(playerEdited)
			if !ok || reply.err != nil || reply.state == nil || reply.state.Current.ID != episode.ID {
				t.Fatalf("play reply lacks authoritative membership: %+v", reply)
			}
			updateLibrary(&m, reply) // intentionally do not consume any playback stream/read continuation
			assertEpisodeMembership(t, m, episode.ID, true, true)
			if len(m.Model.Visible(ui.Queue)) != i+1 || len(m.state.Queue) != i {
				t.Fatal("visible and actor pending queue confused")
			}
			if !strings.Contains(ansi.Strip(m.View().Content), "+   "+episode.Title) {
				t.Fatal("immediate plus marker missing", m.View().Content)
			}
			if i == 0 {
				previous = *reply.state
			}
		}
		assertEpisodeMembership(t, m, first.ID, true, false)
		assertEpisodeMembership(t, m, second.ID, true, true)
		updateLibrary(&m, playbackUpdate{previous, true})
		assertEpisodeMembership(t, m, second.ID, true, true)
		if m.state.Current.ID != second.ID {
			t.Fatal("stale publication replaced current")
		}
		// Markers survive local filtering, inspection and resizing, without a new play.
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: second.Title})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		libraryKey(&m, 'i')
		for _, size := range [][2]int{{30, 12}, {70, 20}, {100, 24}, {180, 56}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			assertEpisodeMembership(t, m, second.ID, true, true)
			if !m.Model.HasFilter(ui.Episodes) || m.detailTarget == nil || m.detailTarget.ID != second.ID {
				t.Fatal("marker update disturbed reading/filter")
			}
		}
	})
}

func TestStreamMembershipIncludesCurrentUntilCompletedAndStopped(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		id := m.rows[0].ID
		if err := s.playback.PlayEpisode(s.ctx, id); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		assertEpisodeMembership(t, m, id, true, true)
		if err := s.playback.Pause(s.ctx); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		assertEpisodeMembership(t, m, id, true, true)
		if err := s.playback.MarkCompleted(s.ctx, id, true); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m) // completing history does not stop paused current
		assertEpisodeMembership(t, m, id, true, true)
		if err := s.playback.Stop(s.ctx); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		assertEpisodeMembership(t, m, id, false, true)
		if len(m.Model.Visible(ui.Queue)) != 0 {
			t.Fatal("completed stopped current remained queued")
		}
	})
}

func TestEnqueueImmediatelyMarksWithoutStartingAudio(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		id := m.rows[0].ID
		finishLibrary(&m, libraryKey(&m, 'a'))
		assertEpisodeMembership(t, m, id, true, false)
		if p.loads != 0 || m.state.Current != nil || len(m.state.Queue) != 1 {
			t.Fatal("marking queued work started audio")
		}
	})
}
