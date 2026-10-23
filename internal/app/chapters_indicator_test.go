package app

import (
	"strings"
	"testing"

	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestChapterAvailabilityForSavedAndPreviewEpisodes(t *testing.T) {
	cases := []struct {
		name, url, data, detail string
		marker                  bool
	}{
		{"unknown", "", "", "", false},
		{"advertised", "https://example.test/chapters.json", "", "checking", true},
		{"inline", "", `[{"index":1,"start":0,"title":"Intro"}]`, "1 available", true},
		{"cached", "https://example.test/chapters.json", `[{"index":1,"start":0,"title":"Intro"}]`, "1 available", true},
		{"invalid cached", "", "invalid", "unavailable", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep := store.Episode{ID: 1, Title: "A title", ChaptersURL: tc.url, ChaptersJSON: tc.data}
			for _, item := range []ui.Item{episodeItem(ep, false), previewEpisodeItem(0, ep, "Show"), queueItem(ep)} {
				if item.Chapters != tc.marker || strings.Contains(item.Title, "[Ch]") || !strings.Contains(item.Title, "A title") {
					t.Fatalf("chapter column/title: %+v", item)
				}
			}
			card := episodeDetails(ep, "Show", "episode:1", false, false)
			if !strings.Contains(card.Chapters, tc.detail) {
				t.Fatalf("chapter details %q, want %q", card.Chapters, tc.detail)
			}
		})
	}
}
