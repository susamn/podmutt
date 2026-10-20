package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"podmutt/internal/opml"
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
	want := "ok    One\nFAIL  https://2.test: boom\nok    Three\n2 imported, 1 failed\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
	if !strings.Contains(out.String(), "FAIL") {
		t.Fatal("missing failure line")
	}
}
