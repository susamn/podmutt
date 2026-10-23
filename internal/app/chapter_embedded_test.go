package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/player"
)

func TestChapterPickerReportsNoEmbeddedChapters(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		deadline := time.Now().Add(time.Second)
		for !syncPlayer(t, &m).CanSeek && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		finishLibrary(&m, libraryKey(&m, 'C'))
		if !strings.Contains(m.notice, "No chapters found") || m.Model.ChapterSelection() != -1 {
			t.Fatal("no embedded chapters feedback", m.notice)
		}
	})
}

func TestChapterPickerDiscoversEmbeddedAudioOnlyAfterLoad(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		p.chapterData = []byte(`[{"time":0,"title":"Opening"},{"time":45,"title":"Main"}]`)
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if cmd := libraryKey(&m, 'C'); cmd != nil {
			t.Fatal("picker queried embedded chapters before file-loaded")
		}
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		deadline := time.Now().Add(time.Second)
		for !syncPlayer(t, &m).CanSeek && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		cmd := libraryKey(&m, 'C')
		if cmd == nil {
			t.Fatal("embedded discovery not scheduled")
		}
		finishLibrary(&m, cmd)
		if m.Model.ChapterSelection() != 0 || !strings.Contains(m.View().Content, "Main") {
			t.Fatal("embedded chapters missing", m.notice)
		}
		cached, err := s.store.Episode(s.ctx, first.ID)
		if err != nil || len(cached.ParsedChapters()) != 2 {
			t.Fatal("embedded cache not saved", cached, err)
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
		p.chapterData = []byte(`not json`)
		libraryKey(&m, 'C')
		if m.Model.ChapterSelection() != 0 {
			t.Fatal("cached chapters not reused")
		}
	})
}
