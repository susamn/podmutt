package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestFavoritesStorageAndIndependence(t *testing.T) {
	s, path := openStore(t)
	feed := addFeed(t, s, "feed-fav", true)
	episodes := addEpisodes(t, s, feed)
	e1, e2 := episodes[0].ID, episodes[1].ID

	// Initial state: none are favorites
	for _, id := range []int64{e1, e2} {
		fav, err := s.IsFavorite(ctx, id)
		if err != nil || fav {
			t.Fatalf("initial IsFavorite(%d) = %v, %v", id, fav, err)
		}
	}
	favs, err := s.FavoriteEpisodes(ctx)
	if err != nil || len(favs) != 0 {
		t.Fatalf("initial favorites: %v, %v", favs, err)
	}

	// Add favorite e1
	if err := s.AddFavorite(ctx, e1); err != nil {
		t.Fatal(err)
	}
	// Idempotent: adding again succeeds without duplicates
	if err := s.AddFavorite(ctx, e1); err != nil {
		t.Fatal(err)
	}

	// Add favorite e2 slightly later
	time.Sleep(10 * time.Millisecond)
	if err := s.AddFavorite(ctx, e2); err != nil {
		t.Fatal(err)
	}

	fav1, err := s.IsFavorite(ctx, e1)
	if err != nil || !fav1 {
		t.Fatalf("expected e1 favorite, got %v, %v", fav1, err)
	}
	fav2, err := s.IsFavorite(ctx, e2)
	if err != nil || !fav2 {
		t.Fatalf("expected e2 favorite, got %v, %v", fav2, err)
	}

	// FavoriteIDs map check
	idMap, err := s.FavoriteIDs(ctx)
	if err != nil || len(idMap) != 2 || !idMap[e1] || !idMap[e2] {
		t.Fatalf("FavoriteIDs = %+v, %v", idMap, err)
	}

	// FavoriteEpisodes ordering (e2 was added later, so it appears first)
	list, err := s.FavoriteEpisodes(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("FavoriteEpisodes = %+v, %v", list, err)
	}
	if list[0].ID != e2 || list[1].ID != e1 {
		t.Fatalf("order mismatch: got [%d, %d], want [%d, %d]", list[0].ID, list[1].ID, e2, e1)
	}
	// Verify metadata fields are populated
	if list[0].Title == "" || list[0].FeedTitle != "feed-fav" {
		t.Fatalf("incomplete metadata in favorites: %+v", list[0])
	}

	// Queue changes must NOT affect favorites
	if err := s.ReplaceQueue(ctx, []int64{e1, e2}); err != nil {
		t.Fatal(err)
	}
	// Clear queue
	if err := s.ReplaceQueue(ctx, nil); err != nil {
		t.Fatal(err)
	}
	list, err = s.FavoriteEpisodes(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("favorites lost after queue clear: %v, %v", len(list), err)
	}

	// Unsubscribing feed must NOT delete favorites
	if err := s.SetSubscribed(ctx, feed, false); err != nil {
		t.Fatal(err)
	}
	list, err = s.FavoriteEpisodes(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("favorites lost after unsubscribe: %v, %v", len(list), err)
	}

	// Playing or marking complete must NOT remove favorites
	if err := s.Checkpoint(ctx, e1, 42); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCompleted(ctx, e1, true); err != nil {
		t.Fatal(err)
	}
	list, err = s.FavoriteEpisodes(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("favorites lost after completion: %v, %v", len(list), err)
	}

	// Reopen database: favorites persist across restarts
	s.Close()
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	list, err = reopened.FavoriteEpisodes(ctx)
	if err != nil || len(list) != 2 || list[0].ID != e2 || list[1].ID != e1 {
		t.Fatalf("favorites failed persistence: %+v, %v", list, err)
	}

	// Explicit removal from favorites
	if err := reopened.RemoveFavorite(ctx, e2); err != nil {
		t.Fatal(err)
	}
	fav2, err = reopened.IsFavorite(ctx, e2)
	if err != nil || fav2 {
		t.Fatalf("e2 still favorite after removal: %v", fav2)
	}
	list, err = reopened.FavoriteEpisodes(ctx)
	if err != nil || len(list) != 1 || list[0].ID != e1 {
		t.Fatalf("remaining favorites mismatch: %+v, %v", list, err)
	}

	// Removing non-existent favorite is a safe no-op
	if err := reopened.RemoveFavorite(ctx, e2); err != nil {
		t.Fatal(err)
	}
}

func TestFavoritesValidationAndForeignKeys(t *testing.T) {
	s, _ := openStore(t)
	// Invalid episode IDs
	if err := s.AddFavorite(ctx, 0); err == nil {
		t.Fatal("expected error for episode ID 0")
	}
	if err := s.AddFavorite(ctx, -1); err == nil {
		t.Fatal("expected error for negative episode ID")
	}
	if err := s.RemoveFavorite(ctx, 0); err == nil {
		t.Fatal("expected error for removal of ID 0")
	}
	isFav, err := s.IsFavorite(ctx, -5)
	if err != nil || isFav {
		t.Fatalf("IsFavorite negative ID = %v, %v", isFav, err)
	}

	// Non-existent episode ID fails foreign key constraint
	if err := s.AddFavorite(ctx, 99999); err == nil {
		t.Fatal("expected foreign key failure for non-existent episode")
	}
}

func TestFavoritesSchemaMigrationFromV3(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", dir+"/migration.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Apply migrations 1, 2, 3 (simulating an existing database prior to favorites)
	if err := migrate(ctx, db, migrations[:3]); err != nil {
		t.Fatal(err)
	}
	var ver int
	if err := db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil || ver != 3 {
		t.Fatalf("expected version 3, got %d", ver)
	}

	// Add sample data at version 3
	res, err := db.Exec("INSERT INTO feeds (url, title, subscribed) VALUES ('https://test.com', 'Test Show', 1)")
	if err != nil {
		t.Fatal(err)
	}
	feedID, _ := res.LastInsertId()
	res, err = db.Exec("INSERT INTO episodes (feed_id, identity, title, audio_url) VALUES (?, 'ep1', 'Episode 1', 'https://test.com/1.mp3')", feedID)
	if err != nil {
		t.Fatal(err)
	}
	epID, _ := res.LastInsertId()

	// Now apply migration 4 (all migrations)
	if err := migrate(ctx, db, migrations); err != nil {
		t.Fatalf("migration to v4 failed: %v", err)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil || ver != 4 {
		t.Fatalf("expected version 4, got %d", ver)
	}

	// Verify favorites table works with existing data
	if _, err := db.Exec("INSERT INTO favorites (episode_id) VALUES (?)", epID); err != nil {
		t.Fatalf("inserting favorite on migrated DB failed: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM favorites WHERE episode_id=?", epID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("favorite count = %d, want 1", count)
	}

	// Trying to migrate with missing migrations (downgrade) is rejected
	if err := migrate(ctx, db, migrations[:2]); err == nil {
		t.Fatal("expected downgrade rejection")
	}
}
