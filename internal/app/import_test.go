package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"podmutt/internal/opml"
	"podmutt/internal/profile"
	"podmutt/internal/store"
)

func TestImportFeedsReportsEachFeedAndFailures(t *testing.T) {
	feeds := []opml.Feed{{Title: "One", URL: "https://1.test"}, {URL: "https://2.test"}, {Title: "Three", URL: "https://3.test"}}
	var out bytes.Buffer
	failed := importFeeds(context.Background(), feeds, func(_ context.Context, url string) error {
		if url == "https://2.test" {
			return errors.New("boom")
		}
		return nil
	}, &out, 2)
	if failed != 1 {
		t.Fatalf("failed = %d", failed)
	}
	want := "ok    One\nFAIL  Feed 2: Import feed failed. Try the action again. If it keeps failing, report it at github.com/susamn/podmutt/issues with your podmutt version.\nok    Three\n2 feeds processed successfully, 1 failed\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
	if !strings.Contains(out.String(), "FAIL") {
		t.Fatal("missing failure line")
	}
}

func TestImportOPMLEndToEndAndAutoqueueSuppression(t *testing.T) {
	paths := isolatedProfile(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `<rss version="2.0"><channel><title>OPML Show</title>
			<item><title>Episode 1</title><enclosure url="https://example.test/ep1.mp3"/></item>
			<item><title>Episode 2</title><enclosure url="https://example.test/ep2.mp3"/></item>
		</channel></rss>`)
	}))
	defer server.Close()

	opmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Subscriptions</title></head>
  <body>
    <outline text="OPML Show" title="OPML Show" type="rss" xmlUrl="%s/feed"/>
    <outline text="Failing Show" type="rss" xmlUrl="%s/bad"/>
  </body>
</opml>`, server.URL, server.URL)

	opmlPath := filepath.Join(t.TempDir(), "subs.opml")
	if err := os.WriteFile(opmlPath, []byte(opmlContent), 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := ImportOPML(opmlPath, &out)
	if err == nil || !strings.Contains(err.Error(), "1 of 2 feeds could not be imported") {
		t.Fatalf("expected partial failure error, got %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "ok    OPML Show") || !strings.Contains(output, "FAIL  Failing Show") {
		t.Fatalf("unexpected output: %s", output)
	}

	// Verify database state directly
	dbPath := filepath.Join(paths.State, "podcasts.db")
	db, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	feeds, err := db.Feeds(context.Background(), true)
	if err != nil || len(feeds) != 1 || feeds[0].Title != "OPML Show" {
		t.Fatalf("expected 1 subscribed feed, got %v, err: %v", feeds, err)
	}
	episodes, err := db.Episodes(context.Background(), feeds[0].ID)
	if err != nil || len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %v, err: %v", episodes, err)
	}

	// Verify autoqueue flood was prevented by receipt seeding
	queued, err := db.EnqueueLatest(context.Background(), 0, 0)
	if err != nil || len(queued) != 0 {
		t.Fatalf("expected 0 queued episodes due to receipt seeding, got %v", queued)
	}
}

func TestImportOPMLErrors(t *testing.T) {
	isolatedProfile(t)
	var out bytes.Buffer

	// File does not exist
	if err := ImportOPML(filepath.Join(t.TempDir(), "missing.opml"), &out); err == nil {
		t.Fatal("expected error for non-existent file")
	}

	// Invalid XML
	badXML := filepath.Join(t.TempDir(), "bad.opml")
	if err := os.WriteFile(badXML, []byte("not xml"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportOPML(badXML, &out); err == nil {
		t.Fatal("expected error for invalid XML")
	}

	// Empty OPML (0 feeds)
	emptyOPML := filepath.Join(t.TempDir(), "empty.opml")
	if err := os.WriteFile(emptyOPML, []byte(`<opml version="2.0"><body></body></opml>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportOPML(emptyOPML, &out); err == nil || !strings.Contains(err.Error(), "no podcast feeds found") {
		t.Fatalf("expected 'no podcast feeds found', got %v", err)
	}
}

func TestImportOPMLProfileLocked(t *testing.T) {
	paths := isolatedProfile(t)
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	lock, err := paths.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	validOPML := filepath.Join(t.TempDir(), "test.opml")
	if err := os.WriteFile(validOPML, []byte(`<opml version="2.0"><body><outline xmlUrl="https://example.test/feed"/></body></opml>`), 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = ImportOPML(validOPML, &out)
	if !errors.Is(err, profile.ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}
}
