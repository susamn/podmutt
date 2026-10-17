package feed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

const parallelism = 4
const requestTimeout = 15 * time.Second

type Repository interface {
	Feeds(context.Context, bool) ([]store.Feed, error)
	UpsertEpisodes(context.Context, int64, []store.Episode) error
	UpsertFeed(context.Context, store.Feed) (int64, error)
}

type Result struct {
	FeedID       int64
	EpisodeCount int // parsed episodes, not a count of newly discovered episodes
	NotModified  bool
	Error        error
}

type Refresher struct {
	repo     Repository
	client   *http.Client
	interval time.Duration
	slots    chan struct{}
	trigger  chan struct{}
	started  atomic.Bool
	mu       sync.Mutex
	flights  map[int64]chan struct{}
}

// New copies the HTTP client before adding redirect policy. Zero/negative
// interval uses 30 minutes; every request has its own 15-second deadline.
func New(repo Repository, client *http.Client, interval time.Duration) *Refresher {
	if client == nil {
		client = &http.Client{}
	}
	copyClient := *client
	redirect := copyClient.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if _, err := httpurl.Parse(req.URL.String()); err != nil {
			return err
		}
		if len(via) > 5 {
			return fmt.Errorf("feed redirect limit exceeded")
		}
		if redirect != nil {
			return redirect(req, via)
		}
		return nil
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Refresher{repo: repo, client: &copyClient, interval: interval, slots: make(chan struct{}, parallelism),
		trigger: make(chan struct{}, 1), flights: make(map[int64]chan struct{})}
}

func (r *Refresher) acquire(ctx context.Context, id int64) (func(), error) {
	for {
		r.mu.Lock()
		if flight, exists := r.flights[id]; exists {
			r.mu.Unlock()
			select {
			case <-flight:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		flight := make(chan struct{})
		r.flights[id] = flight
		r.mu.Unlock()
		return func() { r.mu.Lock(); delete(r.flights, id); close(flight); r.mu.Unlock() }, nil
	}
}

// Refresh fetches one stored feed. The slot and per-feed lock are cancellable.
// Feed metadata/cache validators are committed only after all episodes succeed.
func (r *Refresher) Refresh(ctx context.Context, feed store.Feed) (result Result) {
	result.FeedID = feed.ID
	if feed.ID <= 0 {
		result.Error = fmt.Errorf("feed must be stored before refresh")
		return
	}
	if _, err := httpurl.Parse(feed.URL); err != nil {
		result.Error = err
		return
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	release, err := r.acquire(ctx, feed.ID)
	if err != nil {
		result.Error = err
		return
	}
	defer release()
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		result.Error = ctx.Err()
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		result.Error = fmt.Errorf("invalid feed request")
		return
	}
	req.Header.Set("User-Agent", "podmutt/dev")
	if feed.ETag != "" {
		req.Header.Set("If-None-Match", feed.ETag)
	}
	if feed.LastModified != "" {
		req.Header.Set("If-Modified-Since", feed.LastModified)
	}
	response, err := r.client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("feed request failed (network, TLS, or redirect error)")
		if errors.Is(err, context.Canceled) {
			result.Error = context.Canceled
		} else if errors.Is(err, context.DeadlineExceeded) {
			result.Error = context.DeadlineExceeded
		}
		return
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotModified:
		if feed.ETag == "" && feed.LastModified == "" {
			result.Error = fmt.Errorf("unexpected 304 without cached validators")
			return
		}
		result.NotModified = true
		feed.ETag = first(response.Header.Get("ETag"), feed.ETag)
		feed.LastModified = first(response.Header.Get("Last-Modified"), feed.LastModified)
	case http.StatusOK:
		if response.ContentLength > MaxBytes {
			result.Error = fmt.Errorf("feed response too large")
			return
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
		if err != nil {
			result.Error = fmt.Errorf("feed response read failed")
			if ctx.Err() != nil {
				result.Error = ctx.Err()
			}
			return
		}
		finalURL := req.URL.String()
		if response.Request != nil && response.Request.URL != nil {
			finalURL = response.Request.URL.String()
		}
		doc, err := Parse(data, finalURL)
		if err != nil {
			result.Error = err
			return
		}
		if err := r.repo.UpsertEpisodes(ctx, feed.ID, doc.Episodes); err != nil {
			result.Error = err
			return
		}
		result.EpisodeCount = len(doc.Episodes)
		feed.Title, feed.ArtworkURL = doc.Title, doc.ArtworkURL
		// A 200 without validators clears old ones, unlike a 304.
		feed.ETag, feed.LastModified = response.Header.Get("ETag"), response.Header.Get("Last-Modified")
	default:
		result.Error = fmt.Errorf("feed request returned HTTP %d", response.StatusCode)
		return
	}
	_, result.Error = r.repo.UpsertFeed(ctx, feed)
	return
}
