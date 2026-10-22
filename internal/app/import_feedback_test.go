package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"podmutt/internal/feedback"
	"podmutt/internal/opml"
	"podmutt/internal/store"
)

func TestImportOutputDoesNotEchoURLsParserOrTerminalPayloads(t *testing.T) {
	var out bytes.Buffer
	importFeeds(t.Context(), []opml.Feed{{URL: "https://user:private-secret@example.test/rss?token=private-secret"}, {Title: "Show\x1b[2J https://example.test/?token=private-secret", URL: "https://example.test/rss"}}, func(_ context.Context, _ string) error {
		return fmt.Errorf("raw parser private-secret https://user:private-secret@example.test")
	}, &out, 2)
	if strings.Contains(out.String(), "private-secret") || strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "Feed 1") || !strings.Contains(out.String(), "[link]") {
		t.Fatal(out.String())
	}
}

func TestOPMLReceiptFailureReportsSavedSubscriptionsNotUnqualifiedSuccess(t *testing.T) {
	paths := isolatedProfile(t)
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.State, "podcasts.db")
	db, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.Exec(`CREATE TRIGGER fail_receipt BEFORE INSERT ON autoqueue_seen BEGIN SELECT RAISE(FAIL, 'private-secret receipt failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<rss><channel><title>Fixture</title><item><guid>a</guid><title>Episode</title><enclosure url="/audio.mp3"/></item></channel></rss>`)
	}))
	defer server.Close()
	opmlPath := filepath.Join(t.TempDir(), "import.opml")
	if err := os.WriteFile(opmlPath, []byte(fmt.Sprintf(`<opml><body><outline text="Fixture" xmlUrl="%s/rss"/></body></opml>`, server.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = ImportOPML(opmlPath, &output)
	if err == nil {
		t.Fatal("bookkeeping failure ignored")
	}
	message := feedback.For("OPML import", err)
	if !strings.Contains(message.Text(), "Subscriptions were saved") || strings.Contains(message.Text(), "private-secret") || strings.Contains(output.String(), "private-secret") {
		t.Fatal(message, output.String())
	}
	reopened, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.Feeds(t.Context(), true)
	if err != nil || len(saved) != 1 {
		t.Fatal("saved subscriptions lost", saved, err)
	}
}
