package app

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/feedback"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

type feedbackTransport func(*http.Request) (*http.Response, error)

func (f feedbackTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWebErrorsKeepCategoriesAndHideResponseContents(t *testing.T) {
	for _, source := range []string{"directory", "feed"} {
		for _, status := range []int{200, 401, 403, 404, 429, 503} {
			client := &http.Client{Transport: feedbackTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("malformed private-secret body"))}, nil
			})}
			var err error
			if source == "directory" {
				_, err = discovery.NewApple(client).Search(context.Background(), "science")
			} else {
				_, err = feed.New(nil, client, 0).Fetch(context.Background(), store.Feed{URL: "https://example.test/rss"})
			}
			if err == nil {
				t.Fatal("bad response accepted", source, status)
			}
			message := feedback.For("Search", err)
			want := feedback.HTTP
			if status == 200 {
				want = feedback.InvalidResponse
			}
			if status == 429 {
				want = feedback.RateLimited
			}
			if message.Code != want || strings.Contains(message.Text(), "private-secret") {
				t.Fatal(source, status, message)
			}
		}
	}
	_, err := httpurl.Parse("https://user:private-secret@example.test/rss")
	if message := feedback.For("Preview", err); message.Code != feedback.InvalidURL || strings.Contains(message.Text(), "private-secret") {
		t.Fatal(message)
	}
}
