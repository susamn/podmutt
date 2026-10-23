package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/player"
)

func TestChapterPickerReportsCachedEmbeddedFailuresWithoutReprobing(t *testing.T) {
	for _, tc := range []struct{ name, raw, state, feedback string }{
		{"absent", "", "embedded-empty", "No chapters found"},
		{"invalid", "not json", "embedded-error", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			librarySession(t, func(s *session) {
				m, p := playerLibrary(t, s)
				p.chapterData = []byte(tc.raw)
				first, _ := m.selectedEpisode()
				finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
				entry, _ := p.EntryID(s.ctx)
				p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
				p.events <- player.Event{Name: "file-loaded"}
				deadline := time.Now().Add(time.Second)
				for time.Now().Before(deadline) {
					syncPlayer(t, &m)
					episode, err := s.store.Episode(s.ctx, first.ID)
					if err != nil {
						t.Fatal(err)
					}
					if episode.ChaptersState == tc.state {
						break
					}
					time.Sleep(time.Millisecond)
				}
				episode, err := s.store.Episode(s.ctx, first.ID)
				if err != nil || episode.ChaptersState != tc.state {
					t.Fatal("probe not cached", episode, err)
				}
				p.chapterData = []byte(`[{"time":0,"title":"Should never query"}]`)
				syncPlayer(t, &m)
				libraryKey(&m, 'C')
				if !strings.Contains(m.notice, tc.feedback) || m.Model.ChapterSelection() != -1 {
					t.Fatal("cached failure feedback", m.notice)
				}
			})
		})
	}
}

func TestChapterPickerDiscoversEmbeddedAudioOnLoad(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		p.chapterData = []byte(`[{"time":0,"title":"Opening"},{"time":45,"title":"Main"}]`)
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		libraryKey(&m, 'C')
		if m.Model.ChapterSelection() != -1 {
			t.Fatal("chapters visible before file-loaded")
		}
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			syncPlayer(t, &m)
			cached, err := s.store.Episode(s.ctx, first.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(cached.ParsedChapters()) == 2 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		cached, err := s.store.Episode(s.ctx, first.ID)
		if err != nil || len(cached.ParsedChapters()) != 2 {
			t.Fatal("embedded metadata not cached on load", cached, err)
		}
		p.chapterData = []byte(`not json`)
		syncPlayer(t, &m)
		libraryKey(&m, 'C')
		if m.Model.ChapterSelection() != 0 || !strings.Contains(m.View().Content, "Main") {
			t.Fatal("cached chapters missing", m.notice)
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
		libraryKey(&m, 'C')
		if m.Model.ChapterSelection() != 0 {
			t.Fatal("cached reopen failed")
		}
	})
}
