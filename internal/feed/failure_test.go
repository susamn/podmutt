package feed

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"podmutt/internal/store"
)

var injected = errors.New("injected failure")

type brokenRepository struct {
	Repository
	fail string
}

func (r brokenRepository) Feeds(ctx context.Context, only bool) ([]store.Feed, error) {
	if r.fail == "list" {
		return nil, injected
	}
	return r.Repository.Feeds(ctx, only)
}
func (r brokenRepository) UpsertEpisodes(ctx context.Context, id int64, episodes []store.Episode) error {
	if r.fail == "episodes" {
		return injected
	}
	return r.Repository.UpsertEpisodes(ctx, id, episodes)
}
func (r brokenRepository) UpsertFeed(ctx context.Context, feed store.Feed) (int64, error) {
	if r.fail == "metadata" {
		return 0, injected
	}
	return r.Repository.UpsertFeed(ctx, feed)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, errors.New("secret response body") }

func TestPersistenceFailureDoesNotAdvanceValidators(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"new"`)
		io.WriteString(w, rssFixture)
	}))
	defer server.Close()
	for _, fault := range []string{"episodes", "metadata"} {
		t.Run(fault, func(t *testing.T) {
			db := database(t)
			feed := addFeed(t, db, server.URL)
			result := New(brokenRepository{db, fault}, nil, 0).Refresh(ctx, feed)
			if !errors.Is(result.Error, injected) {
				t.Fatalf("persistence error: %v", result.Error)
			}
			if saved := readFeed(t, db); saved.ETag != "" || saved.Title != "Cached title" {
				t.Fatal("validators committed before episodes")
			}
			items, err := db.Episodes(ctx, feed.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (fault == "episodes" && len(items) != 0) || (fault == "metadata" && len(items) != 2) {
				t.Fatalf("commit ordering: %d items", len(items))
			}
		})
	}
}

func TestRequestAndBodyErrorsAreRedacted(t *testing.T) {
	db := database(t)
	feed := addFeed(t, db, "https://example.test/feed?token=secret")
	for _, transport := range []roundTripFunc{
		func(*http.Request) (*http.Response, error) { return nil, errors.New("https://secret.invalid") },
		func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(brokenBody{})}, nil
		},
	} {
		result := New(db, &http.Client{Transport: transport}, 0).Refresh(ctx, feed)
		if result.Error == nil || strings.Contains(result.Error.Error(), "secret") {
			t.Fatalf("unsafe error: %v", result.Error)
		}
	}
	invalid := feed
	invalid.URL = "file:///tmp/feed"
	if result := New(db, nil, 0).Refresh(ctx, invalid); result.Error == nil {
		t.Fatal("invalid URL accepted")
	}
	invalid.ID = 0
	if result := New(db, nil, 0).Refresh(ctx, invalid); result.Error == nil {
		t.Fatal("unstored feed accepted")
	}
}

func TestRequestCancellationAndTimeout(t *testing.T) {
	db := database(t)
	feed := addFeed(t, db, "https://example.test/feed")
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	client := &http.Client{Transport: transport, Timeout: 10 * time.Millisecond}
	result := New(db, client, 0).Refresh(ctx, feed)
	if !errors.Is(result.Error, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", result.Error)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	result = New(db, client, 0).Refresh(cancelled, feed)
	if !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("cancel: %v", result.Error)
	}
}

func TestDuplicateFeedWaitCanBeCanceled(t *testing.T) {
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	db := database(t)
	feed := addFeed(t, db, server.URL)
	r := New(db, nil, 0)
	first, cancelFirst := context.WithCancel(ctx)
	defer cancelFirst()
	done := make(chan Result, 1)
	go func() { done <- r.Refresh(first, feed) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	second, cancelSecond := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancelSecond()
	if result := r.Refresh(second, feed); !errors.Is(result.Error, context.DeadlineExceeded) {
		t.Fatalf("duplicate wait: %v", result.Error)
	}
	cancelFirst()
	select {
	case result := <-done:
		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("first cancel: %v", result.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request leaked")
	}
}
