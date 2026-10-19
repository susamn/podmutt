package app

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/library"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestEpisodeDetailsUseOneRSSContractAndRelevantHistory(t *testing.T) {
	episode := store.Episode{Title: "<b>RSS title</b>", Description: "<p>First paragraph</p><p>Second</p><script>hidden</script>", PublishedAt: time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), DurationSeconds: 4500}
	card := episodeDetails(episode, "<b>RSS show</b>", "same", false, false)
	if card.Title != "RSS title" || card.Show != "RSS show" || card.Date != "2026-04-12" || card.Duration != "1:15:00" || card.State != "Not started" || strings.Contains(card.Description, "hidden") || !strings.Contains(card.Description, "\n\n") {
		t.Fatal(card)
	}
	episode.ID, episode.FeedID, episode.FeedTitle = 10, 2, "RSS show"
	if cached := episodeDetails(episode, episode.FeedTitle, "same", false, false); !reflect.DeepEqual(cached, card) {
		t.Fatal("cached/preview formatting differs", cached, card)
	}
	episode.PositionSeconds = 42
	card = episodeDetails(episode, episode.FeedTitle, "same", true, true)
	if card.State != "In progress" || card.Position != "0:42" || !card.Queued || !card.Inspecting {
		t.Fatal(card)
	}
	episode.Completed = true
	card = episodeDetails(episode, episode.FeedTitle, "same", false, false)
	if card.State != "Completed" || card.Position != "" {
		t.Fatal("irrelevant resume position", card)
	}
	card = episodeDetails(store.Episode{}, "", "empty", false, false)
	if card.Title != "Untitled episode" || card.Show != "Unknown show" || card.Date != "" || card.Duration != "" {
		t.Fatal(card)
	}
}

func TestPreviewHistoryUsesMatchingFeedAndActorWithoutReplacingRSSMetadata(t *testing.T) {
	m := libraryShell{snapshot: library.Snapshot{Feeds: []store.Feed{{ID: 1, URL: "https://example.test/feed"}}}}
	m.browse.preview = &feed.Resolved{URL: "https://example.test/feed"}
	cached := store.Episode{ID: 2, FeedID: 1, GUID: "episode", Title: "Old cached title", AudioURL: "https://example.test/audio", PositionSeconds: 12}
	m.snapshot.Continue = []store.Episode{cached}
	live := cached
	live.PositionSeconds = 42
	m.state.Current = &live
	preview := store.Episode{GUID: "episode", Title: "Fresh RSS title", AudioURL: cached.AudioURL}
	got := m.detailHistory(preview)
	if got.Title != preview.Title || got.ID != cached.ID || got.PositionSeconds != 42 {
		t.Fatal(got)
	}
	preview.GUID = "different"
	if got := m.detailHistory(preview); got.ID != 0 {
		t.Fatal("matched only audio despite different GUID", got)
	}
	preview.GUID = ""
	if got := m.detailHistory(preview); got.ID != cached.ID {
		t.Fatal("missing GUID fallback", got)
	}
	m.browse.preview.URL = "https://other.test/feed"
	if got := m.detailHistory(preview); got.ID != 0 {
		t.Fatal("history leaked across feeds", got)
	}
}

func TestInspectOtherEpisodeSurvivesPlaybackAndCachedRefresh(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		current := m.state.Current.ID
		libraryKey(&m, 'j')
		selected, _ := m.selectedEpisode()
		finishLibrary(&m, libraryKey(&m, 'i'))
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 56})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		finishLibrary(&m, m.reload())
		if m.detailTarget == nil || m.detailTarget.ID != selected.ID || m.state.Current.ID != current || !strings.Contains(m.View().Content, selected.Title) || !strings.Contains(m.View().Content, "Inspecting") {
			t.Fatal("inspection replaced by current/refresh")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		if item, _ := m.Model.Selected(ui.Episodes); item.ID != episodeItem(selected, false).ID || m.Model.Focus() != ui.Episodes {
			t.Fatal("lost origin", item)
		}
		if p.loads != 1 {
			t.Fatal("inspection started audio")
		}
	})
}
