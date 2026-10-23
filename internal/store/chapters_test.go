package store

import (
	"database/sql"
	"testing"
)

func TestChaptersStorageAndEpisodeMethods(t *testing.T) {
	s, _ := openStore(t)
	feedID := addFeed(t, s, "feed-chapters", true)

	episodes := []Episode{
		{
			GUID:         "ep-url",
			Title:        "Has Chapters URL",
			AudioURL:     "https://test.com/audio1.mp3",
			ChaptersURL:  "https://test.com/chapters.json",
			ChaptersJSON: "",
		},
		{
			GUID:         "ep-json",
			Title:        "Has Inline Chapters",
			AudioURL:     "https://test.com/audio2.mp3",
			ChaptersURL:  "",
			ChaptersJSON: `[{"index":1,"start":0,"title":"Intro"},{"index":2,"start":120,"title":"Topic"}]`,
		},
		{
			GUID:         "ep-none",
			Title:        "No Chapters",
			AudioURL:     "https://test.com/audio3.mp3",
			ChaptersURL:  "",
			ChaptersJSON: "",
		},
	}

	if err := s.UpsertEpisodes(ctx, feedID, episodes); err != nil {
		t.Fatal(err)
	}

	saved, err := s.Episodes(ctx, feedID)
	if err != nil || len(saved) != 3 {
		t.Fatalf("expected 3 episodes, got %v (err %v)", len(saved), err)
	}

	var epURL, epJSON, epNone Episode
	for _, ep := range saved {
		switch ep.GUID {
		case "ep-url":
			epURL = ep
		case "ep-json":
			epJSON = ep
		case "ep-none":
			epNone = ep
		}
	}

	// Verify epURL
	if !epURL.HasChapters() || epURL.ChaptersURL != "https://test.com/chapters.json" || epURL.ChaptersJSON != "" {
		t.Fatalf("unexpected epURL: %+v", epURL)
	}
	if parsed := epURL.ParsedChapters(); len(parsed) != 0 {
		t.Fatalf("epURL ParsedChapters should be empty before json fetch: %+v", parsed)
	}

	// Verify epJSON
	if !epJSON.HasChapters() || epJSON.ChaptersJSON == "" {
		t.Fatalf("unexpected epJSON: %+v", epJSON)
	}
	parsed := epJSON.ParsedChapters()
	if len(parsed) != 2 || parsed[0].Title != "Intro" || parsed[1].Start != 120 {
		t.Fatalf("unexpected parsed chapters in epJSON: %+v", parsed)
	}

	// Verify epNone
	if epNone.HasChapters() || epNone.ChaptersURL != "" || epNone.ChaptersJSON != "" {
		t.Fatalf("epNone should not have chapters: %+v", epNone)
	}

	// Test UpdateEpisodeChapters
	updatedJSON := `[{"index":1,"start":0,"title":"Discovered Intro"}]`
	if err := s.UpdateEpisodeChapters(ctx, epURL.ID, updatedJSON); err != nil {
		t.Fatal(err)
	}

	single, err := s.Episode(ctx, epURL.ID)
	if err != nil || single.ChaptersJSON != updatedJSON {
		t.Fatalf("UpdateEpisodeChapters failed: %+v (err %v)", single, err)
	}
	if parsed = single.ParsedChapters(); len(parsed) != 1 || parsed[0].Title != "Discovered Intro" {
		t.Fatalf("single ParsedChapters failed: %+v", parsed)
	}

	// Feed upsert with empty chapters_json must NOT overwrite existing chapters_json
	episodes[0].ChaptersJSON = ""
	if err := s.UpsertEpisodes(ctx, feedID, episodes[:1]); err != nil {
		t.Fatal(err)
	}
	single, err = s.Episode(ctx, epURL.ID)
	if err != nil || single.ChaptersJSON != updatedJSON {
		t.Fatalf("re-upsert with empty JSON overwrote existing cached chapters: %+v", single)
	}

	// A changed chapter-file URL invalidates the old file's cached timeline.
	episodes[0].ChaptersURL = "https://test.com/revised.json"
	if err := s.UpsertEpisodes(ctx, feedID, episodes[:1]); err != nil {
		t.Fatal(err)
	}
	single, err = s.Episode(ctx, epURL.ID)
	if err != nil || single.ChaptersJSON != "" || single.ChaptersURL != episodes[0].ChaptersURL {
		t.Fatalf("changed URL retained stale chapters: %+v, %v", single, err)
	}

	// A removed chapter declaration clears both availability and cache.
	episodes[0].ChaptersURL = ""
	if err := s.UpsertEpisodes(ctx, feedID, episodes[:1]); err != nil {
		t.Fatal(err)
	}
	single, err = s.Episode(ctx, epURL.ID)
	if err != nil || single.HasChapters() {
		t.Fatalf("removed declaration retained availability: %+v, %v", single, err)
	}
}

func TestChaptersSchemaMigrationFromV4(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", dir+"/migration_v5.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Apply migrations 1 through 4 (before chapters)
	if err := migrate(ctx, db, migrations[:4]); err != nil {
		t.Fatal(err)
	}
	var ver int
	if err := db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil || ver != 4 {
		t.Fatalf("expected version 4, got %d", ver)
	}

	// Insert sample episode at version 4
	res, err := db.Exec("INSERT INTO feeds (url, title, subscribed) VALUES ('https://test.com', 'Show', 1)")
	if err != nil {
		t.Fatal(err)
	}
	feedID, _ := res.LastInsertId()
	res, err = db.Exec("INSERT INTO episodes (feed_id, identity, title, audio_url) VALUES (?, 'ep1', 'Episode 1', 'https://test.com/audio.mp3')", feedID)
	if err != nil {
		t.Fatal(err)
	}
	epID, _ := res.LastInsertId()

	// Apply migration 5
	if err := migrate(ctx, db, migrations[:5]); err != nil {
		t.Fatalf("migration to v5 failed: %v", err)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil || ver != 5 {
		t.Fatalf("expected version 5, got %d", ver)
	}

	// Columns chapters_url and chapters_json should be queryable with default empty strings
	var chapURL, chapJSON string
	err = db.QueryRow("SELECT chapters_url, chapters_json FROM episodes WHERE id=?", epID).Scan(&chapURL, &chapJSON)
	if err != nil || chapURL != "" || chapJSON != "" {
		t.Fatalf("querying migrated chapters columns failed: %v, chapURL=%q, chapJSON=%q", err, chapURL, chapJSON)
	}
	// Existing v5 databases must also upgrade without losing chapter caches.
	if _, err := db.Exec("UPDATE episodes SET chapters_url='https://test.com/chapters',chapters_json='[]' WHERE id=?", epID); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	var origin string
	if err := db.QueryRow("SELECT chapters_json,chapters_origin FROM episodes WHERE id=?", epID).Scan(&chapJSON, &origin); err != nil || chapJSON != "[]" || origin != "" {
		t.Fatal("v6 upgrade lost metadata", chapJSON, origin, err)
	}
}
