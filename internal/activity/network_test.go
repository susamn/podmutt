package activity_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"podmutt/internal/activity"
	"podmutt/internal/artwork"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/store"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEveryWebClientReportsActivity(t *testing.T) {
	var cover bytes.Buffer
	if err := png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, label string
		priority    activity.Priority
		run         func(context.Context, *http.Client) error
	}{
		{"apple-search", "Searching Apple Podcasts…", activity.Foreground, func(ctx context.Context, c *http.Client) error {
			_, err := discovery.NewApple(c).Search(ctx, "science")
			return err
		}},
		{"apple-chart", "Updating Apple Podcasts charts…", activity.Foreground, func(ctx context.Context, c *http.Client) error { _, err := discovery.NewApple(c).Top(ctx); return err }},
		{"ratings", "Loading podcast ratings…", activity.Background, func(ctx context.Context, c *http.Client) error {
			_, _, err := discovery.NewApple(c).Ratings(ctx, "us", "1")
			return err
		}},
		{"index", "Searching Podcast Index…", activity.Foreground, func(ctx context.Context, c *http.Client) error {
			_, err := discovery.NewIndex("fixture-key", "fixture-secret", c).Search(ctx, "science")
			return err
		}},
		{"feed", "Loading podcast feed…", activity.Foreground, func(ctx context.Context, c *http.Client) error {
			_, err := feed.New(nil, c, 0).Fetch(ctx, store.Feed{URL: "https://example.test/rss"})
			return err
		}},
		{"preview", "Loading feed preview…", activity.Foreground, func(ctx context.Context, c *http.Client) error {
			_, err := feed.New(nil, c, 0).Resolve(ctx, "https://example.test/rss")
			return err
		}},
		{"artwork", "Loading artwork…", activity.Background, func(ctx context.Context, c *http.Client) error {
			_, err := artwork.New(c).Image(ctx, "https://example.test/cover", "")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, cancelRequest := range []bool{false, true} {
				tracker := activity.New()
				ctx, cancel := context.WithCancel(activity.WithTracker(context.Background(), tracker))
				entered, release := make(chan struct{}, 1), make(chan struct{})
				client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					select {
					case entered <- struct{}{}:
					default:
					}
					select {
					case <-release:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
					body := `{"results":[{"collectionId":1,"collectionName":"Science","feedUrl":"https://example.test/rss"}]}`
					switch {
					case strings.Contains(r.URL.Path, "podcasts.json"):
						body = `{"feed":{"results":[{"id":"1"}]}}`
					case strings.Contains(r.URL.Path, "customerreviews"):
						body = `{"feed":{"entry":[]}}`
					case strings.Contains(r.URL.Host, "podcastindex"):
						body = `{"status":true,"feeds":[{"id":1,"title":"Science","url":"https://example.test/rss"}]}`
					case r.URL.Path == "/rss":
						body = `<rss><channel><title>Science</title><item><title>Episode</title><enclosure url="https://example.test/audio.mp3" type="audio/mpeg"/></item></channel></rss>`
					case r.URL.Path == "/cover":
						body = cover.String()
					}
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})}
				done := make(chan error, 1)
				go func() { done <- tc.run(ctx, client) }()
				select {
				case <-entered:
				case <-time.After(time.Second):
					cancel()
					t.Fatal("no HTTP request")
				}
				s := tracker.Snapshot()
				if len(s.Operations) != 1 || s.Operations[0].Label != tc.label || s.Operations[0].Priority != tc.priority {
					cancel()
					t.Fatal(s)
				}
				if cancelRequest {
					cancel()
				} else {
					close(release)
				}
				select {
				case err := <-done:
					if (err != nil) != cancelRequest {
						t.Fatal(fmt.Sprintf("cancel=%v err=%v", cancelRequest, err))
					}
				case <-time.After(5 * time.Second):
					cancel()
					t.Fatal("worker stuck")
				}
				cancel()
				if len(tracker.Snapshot().Operations) != 0 {
					t.Fatal("activity not finished")
				}
			}
		})
	}
}
