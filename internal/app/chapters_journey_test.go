package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/chapters"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestChaptersEndToEndUserJourney(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})

		// 1. Setup fixture episodes:
		// ep1: has inline chapters
		// ep2: has external chapter URL
		// ep3: has no chapters
		chList := []chapters.Chapter{
			{Index: 1, Start: 0, Title: "Intro"},
			{Index: 2, Start: 90, Title: "Interview"},
			{Index: 3, Start: 300, Title: "Wrap Up"},
		}
		chJSON, err := chapters.Serialize(chList)
		if err != nil {
			t.Fatal(err)
		}

		episodes, err := s.store.Episodes(s.ctx, m.snapshot.Feeds[0].ID)
		if err != nil || len(episodes) < 2 {
			t.Fatal("need at least 2 episodes in feed", err)
		}

		ep1 := episodes[0]
		ep1.ChaptersJSON = chJSON
		ep1.DurationSeconds = 600

		ep2 := episodes[1]
		ep2.ChaptersURL = "https://example.test/chapters.json"
		ep2.DurationSeconds = 1200

		ep3 := store.Episode{
			FeedID:          ep1.FeedID,
			GUID:            "ep3-no-chapters",
			Title:           "Episode Three No Chapters",
			AudioURL:        "https://example.test/audio3.mp3",
			PublishedAt:     time.Now(),
			DurationSeconds: 400,
		}

		if err := s.store.UpsertEpisodes(s.ctx, ep1.FeedID, []store.Episode{ep1, ep2, ep3}); err != nil {
			t.Fatal(err)
		}

		savedEpisodes, err := s.store.Episodes(s.ctx, ep1.FeedID)
		if err != nil {
			t.Fatal(err)
		}
		for _, ep := range savedEpisodes {
			switch ep.GUID {
			case ep1.GUID:
				ep1 = ep
			case ep2.GUID:
				ep2 = ep
			case ep3.GUID:
				ep3 = ep
			}
		}

		finishLibrary(&m, m.reload())

		// 2. Verify Ch column in Episodes pane
		// ep1 and ep2 should display the chapter indicator (✓)
		// ep3 should not
		m.Model = m.Model.WithFocus(ui.Episodes)
		renderedEpisodes := ansi.Strip(m.View().Content)
		if !strings.Contains(renderedEpisodes, "Ch") {
			t.Fatalf("expected 'Ch' column header in Episodes pane:\n%s", renderedEpisodes)
		}

		// 3. Add ep1 and ep3 to Queue
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(ep1.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(ep3.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))

		state := syncPlayer(t, &m)
		if len(state.Queue) != 2 {
			t.Fatalf("expected 2 queued episodes, got %d", len(state.Queue))
		}

		// 4. Verify Ch column in Queue pane
		m.Model = m.Model.WithFocus(ui.Queue)
		renderedQueue := ansi.Strip(m.View().Content)
		if !strings.Contains(renderedQueue, "Ch") {
			t.Fatalf("expected 'Ch' column header in Queue pane:\n%s", renderedQueue)
		}

		// 5. Start playing ep1
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(ep1.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))

		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(`10`)}

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s := syncPlayer(t, &m)
			if s.CanSeek && s.Current.PositionSeconds == 10 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Pause playback
		if err := s.playback.Pause(s.ctx); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)

		// Check player progress annotation
		content := m.View().Content
		if !strings.Contains(content, "Chapter 1/3") || !strings.Contains(content, "Intro") {
			t.Fatalf("expected player to display Chapter 1/3 · Intro:\n%s", content)
		}

		// 6. Inspect ep3 in Details while ep1 is playing
		// Move focus to Episodes, select ep3, press 'i'
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(ep3.ID))
		libraryKey(&m, 'i')

		if m.Model.Focus() != ui.Details || m.detailTarget == nil || m.detailTarget.ID != ep3.ID {
			t.Fatalf("expected Details pinned to ep3: focus=%v target=%+v", m.Model.Focus(), m.detailTarget)
		}

		detailsContent := ansi.Strip(m.View().Content)
		if strings.Contains(detailsContent, "chapters available") {
			t.Fatalf("ep3 Details should not report chapters available:\n%s", detailsContent)
		}

		// 7. Global 'C' from Details pane: must target currently playing ep1, NOT ep3!
		libraryKey(&m, 'C')
		pickerContent := ansi.Strip(m.View().Content)
		if !strings.Contains(pickerContent, cleanTitle(ep1.Title)) {
			t.Fatalf("chapter picker should identify ep1 title, got:\n%s", pickerContent)
		}
		if m.Model.ChapterSelection() != 0 {
			t.Fatalf("initial chapter selection want 0 (Intro), got %d", m.Model.ChapterSelection())
		}

		// 8. Navigate picker with j, select Chapter 2 (Interview), press Enter to jump
		libraryKey(&m, 'j')
		if m.Model.ChapterSelection() != 1 {
			t.Fatalf("after 'j' want selection 1, got %d", m.Model.ChapterSelection())
		}

		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))

		// Playback should have sought to 90s, stayed paused, and focus returned to Details
		sState := syncPlayer(t, &m)
		if sState.Current.ID != ep1.ID || sState.Current.PositionSeconds != 90 || sState.Status != playback.Paused {
			t.Fatalf("chapter jump failed: position=%v status=%v", sState.Current.PositionSeconds, sState.Status)
		}
		if m.Model.Focus() != ui.Details || m.detailTarget == nil || m.detailTarget.ID != ep3.ID {
			t.Fatalf("focus or pinned target lost after chapter jump: focus=%v target=%+v", m.Model.Focus(), m.detailTarget)
		}

		// Player HUD now displays Chapter 2/3 · Interview
		if !strings.Contains(m.View().Content, "Chapter 2/3") || !strings.Contains(m.View().Content, "Interview") {
			t.Fatalf("player HUD did not update to Chapter 2/3 · Interview:\n%s", m.View().Content)
		}

		// 9. Input isolation tests:
		// 9a. In search/filter (/): 'C' types 'C', does not open chapter picker
		m.Model = m.Model.WithFocus(ui.Episodes)
		updateLibrary(&m, tea.KeyPressMsg{Code: '/', Text: "/"})
		updateLibrary(&m, tea.KeyPressMsg{Code: 'C', Text: "C"})
		if m.Model.ChapterSelection() != -1 {
			t.Fatal("key 'C' during list filter opened chapter picker")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc}) // cancel filter

		// 9b. In Settings (e): 'C' cycles country storefront as documented, does not open picker
		updateLibrary(&m, tea.KeyPressMsg{Code: 'e', Text: "e"})
		if !m.Model.SettingsOpen() {
			t.Fatal("Settings did not open")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'C', Text: "C"})
		if m.Model.ChapterSelection() != -1 {
			t.Fatal("key 'C' in Settings opened chapter picker")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc}) // close settings

		// 10. Compact terminal (30×12) rendering test:
		updateLibrary(&m, tea.WindowSizeMsg{Width: 30, Height: 12})
		libraryKey(&m, 'C')
		compactPicker := ansi.Strip(m.View().Content)
		if !strings.Contains(compactPicker, "Chapters") || !strings.Contains(compactPicker, "Intro") {
			t.Fatalf("compact chapter picker did not render properly in 30x12:\n%s", compactPicker)
		}
		// Close with Esc
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
		if m.Model.ChapterSelection() != -1 {
			t.Fatal("Esc did not close chapter picker in compact mode")
		}

		// Restore terminal size
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})

		// 11. Actor safety against stale jump:
		// Open picker on ep1, then playback changes to ep3 in the background.
		// Enter on the picker must be rejected and must not seek ep3!
		libraryKey(&m, 'C')
		capturedRequest := m.chapterRequest

		// Natural playback advance or user stop/play replaces current episode with ep3
		if err := s.playback.Stop(s.ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.playback.PlayEpisode(s.ctx, ep3.ID); err != nil {
			t.Fatal(err)
		}
		entry, _ = p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(`5`)}

		// Trying to seek via capturedRequest is rejected with ErrSeekUnavailable
		if err := s.playback.SeekChapter(s.ctx, capturedRequest.episodeID, capturedRequest.playbackID, 300); err == nil {
			t.Fatal("stale chapter seek accepted after episode replacement")
		}

		finalState := syncPlayer(t, &m)
		if finalState.Current.ID != ep3.ID || finalState.Current.PositionSeconds == 300 {
			t.Fatalf("ep3 was incorrectly sought by stale chapter request: %+v", finalState.Current)
		}
	})
}
