package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/chapters"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestChapterPickerUsesCurrentPlaybackAcrossPanesAndGuardsStaleJump(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		if cmd := libraryKey(&m, 'C'); cmd != nil || !strings.Contains(m.notice, "No current") {
			t.Fatal("C without playback started or sought audio", m.notice)
		}
		first, _ := m.selectedEpisode()
		list, _ := chapters.Serialize([]chapters.Chapter{
			{Index: 1, Start: 0, Title: "Intro"}, {Index: 2, Start: 60, Title: "Interview"},
		})
		first.ChaptersJSON = list
		if err := s.store.UpsertEpisodes(s.ctx, first.FeedID, []store.Episode{first}); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(`65`)}
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			state := syncPlayer(t, &m)
			if state.CanSeek && state.Current.PositionSeconds == 65 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if err := s.playback.Pause(s.ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.playback.SeekTo(s.ctx, 65); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		for _, focus := range []ui.Panel{ui.Podcasts, ui.Episodes, ui.Queue, ui.Details, ui.Favorites} {
			m.Model = m.Model.WithFocus(focus)
			libraryKey(&m, 'C')
			if m.Model.ChapterSelection() != 1 || !strings.Contains(m.View().Content, "Interview") {
				t.Fatal("picker missing or current chapter not selected", focus, m.Model.ChapterSelection(), m.state.Current.PositionSeconds, m.state.Current.ChaptersJSON, m.notice)
			}
			libraryKey(&m, 'k')
			if m.Model.ChapterSelection() != 0 {
				t.Fatal("k failed")
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyPgDown})
			if m.Model.ChapterSelection() != 1 {
				t.Fatal("paging down failed")
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyPgUp})
			if m.Model.ChapterSelection() != 0 {
				t.Fatal("paging up failed")
			}
			libraryKey(&m, 'G')
			if m.Model.ChapterSelection() != 1 {
				t.Fatal("last chapter failed")
			}
			libraryKey(&m, 'g')
			if m.Model.ChapterSelection() != 0 {
				t.Fatal("first chapter failed")
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
			if m.Model.Focus() != focus || m.Model.ChapterSelection() != -1 {
				t.Fatal("Esc lost focus or sought")
			}
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		libraryKey(&m, 'C')
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		state := syncPlayer(t, &m)
		if state.Current.ID != first.ID || state.Current.PositionSeconds != 60 || state.Status != "paused" {
			t.Fatal("chapter jump changed episode/transport or position", state)
		}
		libraryKey(&m, 'C')
		request := m.chapterRequest
		if err := s.playback.Stop(s.ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.playback.PlayEpisode(s.ctx, first.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.playback.SeekChapter(s.ctx, request.episodeID, request.playbackID, 600); err == nil {
			t.Fatal("stale picker jump accepted after same-episode replacement")
		}
		syncPlayer(t, &m)
		if m.Model.ChapterSelection() != -1 {
			t.Fatal("picker remained open after replacement")
		}
	})
}
