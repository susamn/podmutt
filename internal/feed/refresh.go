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

	"podmutt/internal/activity"
	"podmutt/internal/chapters"
	"podmutt/internal/feedback"
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
type Fetched struct {
	Document           Document
	ETag, LastModified string
	NotModified        bool
}
type Refresher struct {
	repo     Repository
	client   *http.Client
	interval time.Duration
	slots    chan struct{}
	trigger  chan struct{}
	started  atomic.Bool
	mu       sync.Mutex
	flights  map[string]chan struct{}
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
	return &Refresher{repo: repo, client: &copyClient, interval: interval, slots: make(chan struct{}, parallelism), trigger: make(chan struct{}, 1), flights: make(map[string]chan struct{})}
}
func (r *Refresher) acquire(ctx context.Context, key string) (func(), error) {
	for {
		r.mu.Lock()
		if flight, exists := r.flights[key]; exists {
			r.mu.Unlock()
			select {
			case <-flight:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		flight := make(chan struct{})
		r.flights[key] = flight
		r.mu.Unlock()
		return func() { r.mu.Lock(); delete(r.flights, key); close(flight); r.mu.Unlock() }, nil
	}
}
func (r *Refresher) withFeed(ctx context.Context, feed store.Feed, action func(context.Context) error) error {
	if _, err := httpurl.Parse(feed.URL); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	release, err := r.acquire(ctx, feed.URL)
	if err != nil {
		return err
	}
	defer release()
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return action(ctx)
}

// Fetch reads without persistence, sharing refresh's transport, limits and locks.
// Use it to validate an add/preview before creating a stored subscription.
func (r *Refresher) Fetch(ctx context.Context, source store.Feed) (Fetched, error) {
	ctx, finish := activity.Begin(ctx, "Loading podcast feed…", activity.Foreground)
	defer finish()
	var result Fetched
	err := r.withFeed(ctx, source, func(ctx context.Context) error {
		var err error
		result, err = r.fetch(ctx, source)
		return err
	})
	return result, err
}

// Refresh commits episode data before metadata/cache validators. The per-feed
// lock remains held through persistence, not merely the HTTP request.
func (r *Refresher) Refresh(ctx context.Context, source store.Feed) (result Result) {
	ctx, finish := activity.Begin(ctx, "Refreshing podcast feed…", activity.Background)
	defer finish()
	result.FeedID = source.ID
	if source.ID <= 0 {
		result.Error = fmt.Errorf("feed must be stored before refresh")
		return
	}
	result.Error = r.withFeed(ctx, source, func(ctx context.Context) error {
		fetched, err := r.fetch(ctx, source)
		if err != nil {
			return err
		}
		result.NotModified = fetched.NotModified
		if !fetched.NotModified {
			if err := r.repo.UpsertEpisodes(ctx, source.ID, fetched.Document.Episodes); err != nil {
				return err
			}
			result.EpisodeCount = len(fetched.Document.Episodes)
			source.Title, source.ArtworkURL = fetched.Document.Title, fetched.Document.ArtworkURL
		}
		source.ETag, source.LastModified = fetched.ETag, fetched.LastModified
		_, err = r.repo.UpsertFeed(ctx, source)
		return err
	})
	if result.Error == nil {
		// Episode metadata is committed before fetching optional chapter files.
		// Cached successes/empty/error results are not re-fetched each cycle.
		r.prefetchChapters(ctx, source.ID)
	}
	return
}

type chapterFileRepository interface {
	PendingChapterFiles(context.Context, int64) ([]store.ChapterFile, error)
	CacheEpisodeChapters(context.Context, int64, string, string) (bool, error)
	MarkChaptersChecked(context.Context, int64, string, string) error
}

func (r *Refresher) prefetchChapters(ctx context.Context, feedID int64) {
	repo, ok := r.repo.(chapterFileRepository)
	if !ok {
		return
	}
	files, err := repo.PendingChapterFiles(ctx, feedID)
	if err != nil {
		return
	}
	jobs := make(chan store.ChapterFile)
	var workers sync.WaitGroup
	for range min(parallelism, len(files)) {
		workers.Go(func() {
			for file := range jobs {
				list, err := r.fetchChapters(ctx, file.URL, activity.Background)
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					_ = repo.MarkChaptersChecked(ctx, file.EpisodeID, file.URL, "error")
					continue
				}
				if len(list) == 0 {
					_ = repo.MarkChaptersChecked(ctx, file.EpisodeID, file.URL, "empty")
					continue
				}
				encoded, err := chapters.Serialize(list)
				if err == nil {
					_, _ = repo.CacheEpisodeChapters(ctx, file.EpisodeID, file.URL, encoded)
				}
			}
		})
	}
send:
	for _, file := range files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	workers.Wait()
}

// FetchChapters downloads and parses an external Podcasting 2.0 JSON chapters file.
func (r *Refresher) FetchChapters(ctx context.Context, address string) ([]chapters.Chapter, error) {
	return r.fetchChapters(ctx, address, activity.Foreground)
}

func (r *Refresher) fetchChapters(ctx context.Context, address string, priority activity.Priority) ([]chapters.Chapter, error) {
	ctx, finish := activity.Begin(ctx, "Loading chapters…", priority)
	defer finish()
	if _, err := httpurl.Parse(address); err != nil {
		return nil, feedback.New(feedback.InvalidURL, "invalid chapters URL")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, feedback.Wrap(feedback.InvalidURL, err)
	}
	req.Header.Set("User-Agent", "podmutt/dev")
	req.Header.Set("Accept", "application/json+chapters, application/json")
	response, err := r.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		return nil, feedback.Wrap(feedback.Network, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, feedback.StatusError(response.StatusCode)
	}
	if response.ContentLength > MaxBytes {
		return nil, feedback.New(feedback.InvalidResponse, "chapters file too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, feedback.Wrap(feedback.InvalidResponse, err)
	}
	if len(data) > MaxBytes {
		return nil, feedback.New(feedback.InvalidResponse, "chapters file too large")
	}
	list, err := chapters.ParseJSON(data)
	if err != nil {
		return nil, feedback.Wrap(feedback.InvalidResponse, err)
	}
	return list, nil
}
