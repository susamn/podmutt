package store

import (
	"context"
	"testing"
)

func TestDiscoveryStoreCRUDAndRatings(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()

	appleID, err := s.SourceID(ctx, "apple")
	if err != nil || appleID != 1 {
		t.Fatalf("expected apple source id 1, got %d %v", appleID, err)
	}
	indexID, err := s.SourceID(ctx, "podcast_index")
	if err != nil || indexID != 2 {
		t.Fatalf("expected index source id 2, got %d %v", indexID, err)
	}
	if _, err := s.SourceID(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent source")
	}

	shows := []DiscoveredShow{
		{
			Title:            "The Daily",
			FeedURL:          "https://feeds.simplecast.com/thedaily",
			Description:      "Daily news podcast",
			Country:          "us",
			SourceID:         appleID,
			SourceExternalID: "1200361736",
		},
		{
			Title:            "Zeit Verbrechen",
			FeedURL:          "https://feeds.zeit.de/verbrechen",
			Description:      "True crime from Germany",
			Country:          "de",
			SourceID:         appleID,
			SourceExternalID: "1388301136",
		},
	}

	if err := s.UpsertDiscoveredShows(ctx, "us", appleID, shows); err != nil {
		t.Fatal(err)
	}

	// Search all US shows
	usShows, err := s.DiscoveredShows(ctx, "us", "", 50)
	if err != nil || len(usShows) != 1 || usShows[0].Title != "The Daily" {
		t.Fatalf("unexpected US shows: %+v %v", usShows, err)
	}

	// Search DE shows
	deShows, err := s.DiscoveredShows(ctx, "de", "", 50)
	if err != nil || len(deShows) != 1 || deShows[0].Title != "Zeit Verbrechen" {
		t.Fatalf("unexpected DE shows: %+v %v", deShows, err)
	}

	// Search by query
	crime, err := s.DiscoveredShows(ctx, "de", "Verbrechen", 10)
	if err != nil || len(crime) != 1 || crime[0].Title != "Zeit Verbrechen" {
		t.Fatalf("unexpected query result: %+v %v", crime, err)
	}

	// Upsert ratings
	showID := usShows[0].ID
	if err := s.UpsertShowRating(ctx, showID, appleID, 4.8, 14200); err != nil {
		t.Fatal(err)
	}

	ratings, err := s.ShowRatings(ctx, showID)
	if err != nil || len(ratings) != 1 || ratings[0].Rating != 4.8 || ratings[0].ReviewCount != 14200 {
		t.Fatalf("unexpected ratings: %+v %v", ratings, err)
	}

	// Validate rating bounds
	if err := s.UpsertShowRating(ctx, showID, appleID, 5.5, 10); err == nil {
		t.Fatal("expected error for rating > 5")
	}
	if err := s.UpsertShowRating(ctx, showID, appleID, -0.5, 10); err == nil {
		t.Fatal("expected error for rating < 0")
	}

	// Update existing rating
	if err := s.UpsertShowRating(ctx, showID, appleID, 4.9, 15000); err != nil {
		t.Fatal(err)
	}
	ratings, err = s.ShowRatings(ctx, showID)
	if err != nil || len(ratings) != 1 || ratings[0].Rating != 4.9 || ratings[0].ReviewCount != 15000 {
		t.Fatalf("expected updated rating: %+v %v", ratings, err)
	}

	// DiscoveredShows should reflect the joined rating
	usShows, err = s.DiscoveredShows(ctx, "us", "Daily", 10)
	if err != nil || len(usShows) != 1 || usShows[0].Rating != 4.9 || usShows[0].ReviewCount != 15000 {
		t.Fatalf("expected joined rating in DiscoveredShows: %+v", usShows)
	}
}
