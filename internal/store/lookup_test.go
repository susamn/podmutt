package store

import (
	"database/sql"
	"errors"
	"testing"
)

func TestEpisodeLookup(t *testing.T) {
	s, _ := openStore(t)
	id := addFeed(t, s, "preview", false)
	items := addEpisodes(t, s, id)
	got, err := s.Episode(ctx, items[0].ID)
	if err != nil || got != items[0] {
		t.Fatalf("lookup = %+v, %v", got, err)
	}
	if _, err := s.Episode(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing episode: %v", err)
	}
	s.Close()
	if _, err := s.Episode(ctx, items[0].ID); err == nil {
		t.Fatal("closed lookup succeeded")
	}
}
