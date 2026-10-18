package library

import (
	"fmt"
	"strings"
	"testing"

	"podmutt/internal/store"
)

func TestSearchSubscribedTitlesDescriptionsAndCompletedHistory(t *testing.T) {
	db := database(t)
	fetcher := &fixtureFetcher{}
	service := New(db, fetcher)
	for _, subscribed := range []bool{true, false} {
		id, err := db.UpsertFeed(ctx, store.Feed{URL: fmt.Sprintf("https://example.test/%t", subscribed), Subscribed: subscribed})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertEpisodes(ctx, id, []store.Episode{{GUID: "a", Title: "Résumé", AudioURL: "https://example.test/audio", Description: "<b>Deep</b> \n dive <script>hidden-token</script>"}}); err != nil {
			t.Fatal(err)
		}
		rows, err := db.Episodes(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.SetCompleted(ctx, rows[0].ID, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"RÉSUMÉ", "deep   DIVE"} {
		hits, err := service.Search(ctx, query)
		if err != nil || len(hits) != 1 || !hits[0].Completed {
			t.Fatal(query, hits, err)
		}
		hits[0].Title = "caller mutation"
		again, err := service.Search(ctx, query)
		if err != nil || again[0].Title == "caller mutation" {
			t.Fatal("snapshot aliased")
		}
	}
	for _, query := range []string{"", "  ", "missing", "hidden-token", strings.Repeat("x", 129)} {
		hits, err := service.Search(ctx, query)
		if err != nil || len(hits) != 0 {
			t.Fatal(query, hits, err)
		}
	}
	if fetcher.calls != 0 {
		t.Fatal("cached search used network")
	}
	db.Close()
	if _, err := service.Search(ctx, "query"); err == nil {
		t.Fatal("storage failure hidden")
	}
}
func TestSearchCapsResults(t *testing.T) {
	db := database(t)
	id, err := db.UpsertFeed(ctx, store.Feed{URL: "https://example.test/feed", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	var episodes []store.Episode
	for i := 0; i < 105; i++ {
		episodes = append(episodes, store.Episode{GUID: fmt.Sprint(i), Title: "match", AudioURL: fmt.Sprintf("https://example.test/%d", i)})
	}
	if err := db.UpsertEpisodes(ctx, id, episodes); err != nil {
		t.Fatal(err)
	}
	hits, err := New(db, nil).Search(ctx, "match")
	if err != nil || len(hits) != 100 {
		t.Fatal(len(hits), err)
	}
}
