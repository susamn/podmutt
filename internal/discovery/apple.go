// Package discovery isolates the optional, no-key online show directory.
// It never searches remote episodes, downloads media, or changes subscriptions.
package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"podmutt/internal/httpurl"
)

const maxResponse = 1 << 20
const maxResults = 25

var ErrQuery = errors.New("show search requires 1–128 characters")
var ErrUnavailable = errors.New("show directory unavailable (network, TLS, or redirect error)")
var ErrRateLimited = errors.New("show directory rate limited; try again later")

type Show struct{ Title, FeedURL, SourceURL string }
type cached struct {
	shows   []Show
	expires time.Time
}
type appleRecord struct {
	CollectionID                                          int64
	CollectionName, TrackName, FeedURL, CollectionViewURL string
}

// Apple serializes requests per foreground instance. Canceled waits never
// reserve future quota slots. Chart, lookup and search share quota/cache policy.
type Apple struct {
	client                                  *http.Client
	endpoint, chartEndpoint, lookupEndpoint string
	gate                                    chan struct{}
	interval                                time.Duration
	next                                    time.Time
	cache                                   map[string]cached
}

func NewApple(client *http.Client) *Apple {
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	if copy.Timeout <= 0 || copy.Timeout > 15*time.Second {
		copy.Timeout = 15 * time.Second
	}
	previous := copy.CheckRedirect
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return ErrUnavailable
		}
		if _, err := httpurl.Parse(req.URL.String()); err != nil {
			return ErrUnavailable
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &Apple{client: &copy, endpoint: "https://itunes.apple.com/search", chartEndpoint: "https://rss.marketingtools.apple.com/api/v2/us/podcasts/top/25/podcasts.json", lookupEndpoint: "https://itunes.apple.com/lookup", gate: make(chan struct{}, 1), interval: 3 * time.Second, cache: make(map[string]cached)}
}
func (a *Apple) Search(ctx context.Context, query string) ([]Show, error) {
	query = strings.Join(strings.Fields(query), " ")
	if size := len([]rune(query)); size == 0 || size > 128 {
		return nil, ErrQuery
	}
	return a.load(ctx, strings.ToLower(query), func(ctx context.Context) ([]Show, error) {
		endpoint, err := url.Parse(a.endpoint)
		if err != nil {
			return nil, ErrUnavailable
		}
		q := endpoint.Query()
		q.Set("term", query)
		q.Set("media", "podcast")
		q.Set("entity", "podcast")
		q.Set("limit", strconv.Itoa(maxResults))
		endpoint.RawQuery = q.Encode()
		var document struct{ Results []appleRecord }
		if err := a.read(ctx, endpoint.String(), &document); err != nil {
			return nil, err
		}
		return normalize(document.Results), nil
	})
}

// Top returns the non-personalized US top-25 chart, resolved to public RSS URLs
// via one batch lookup. Apple artwork and preview assets are ignored entirely.
func (a *Apple) Top(ctx context.Context) ([]Show, error) {
	return a.load(ctx, "\x00top-us", func(ctx context.Context) ([]Show, error) {
		var chart struct {
			Feed struct{ Results []struct{ ID string } }
		}
		if err := a.read(ctx, a.chartEndpoint, &chart); err != nil {
			return nil, err
		}
		var ids []string
		rank := make(map[int64]int)
		for _, item := range chart.Feed.Results {
			id, err := strconv.ParseInt(item.ID, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			if _, ok := rank[id]; ok {
				continue
			}
			rank[id] = len(ids)
			ids = append(ids, strconv.FormatInt(id, 10))
			if len(ids) == maxResults {
				break
			}
		}
		if len(ids) == 0 {
			return nil, errors.New("Apple chart contains no usable shows")
		}
		endpoint, err := url.Parse(a.lookupEndpoint)
		if err != nil {
			return nil, ErrUnavailable
		}
		q := endpoint.Query()
		q.Set("id", strings.Join(ids, ","))
		q.Set("entity", "podcast")
		endpoint.RawQuery = q.Encode()
		var document struct{ Results []appleRecord }
		if err := a.read(ctx, endpoint.String(), &document); err != nil {
			return nil, err
		}
		var records []appleRecord
		for _, record := range document.Results {
			if _, ok := rank[record.CollectionID]; ok {
				records = append(records, record)
			}
		}
		sort.SliceStable(records, func(i, j int) bool { return rank[records[i].CollectionID] < rank[records[j].CollectionID] })
		shows := normalize(records)
		if len(shows) == 0 {
			return nil, errors.New("Apple chart has no available RSS feeds")
		}
		return shows, nil
	})
}
func (a *Apple) load(ctx context.Context, key string, fetch func(context.Context) ([]Show, error)) ([]Show, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case a.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-a.gate }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if entry, ok := a.cache[key]; ok && time.Now().Before(entry.expires) {
		return append([]Show(nil), entry.shows...), nil
	}
	shows, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	if len(a.cache) >= 128 {
		a.cache = make(map[string]cached)
	}
	a.cache[key] = cached{shows: append([]Show(nil), shows...), expires: time.Now().Add(10 * time.Minute)}
	return shows, nil
}
func (a *Apple) read(ctx context.Context, address string, document any) error {
	return a.readHeaders(ctx, address, document, nil)
}

// readHeaders shares bounded I/O/quota policy, not directory caches or secrets.
func (a *Apple) readHeaders(ctx context.Context, address string, document any, headers http.Header) error {
	if delay := time.Until(a.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("User-Agent", "podmutt/dev")
	req.Header.Set("Accept", "application/json")
	for name, values := range headers {
		req.Header[name] = append([]string(nil), values...)
	}
	a.next = time.Now().Add(a.interval)
	response, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		a.next = time.Now().Add(retryDelay(response.Header.Get("Retry-After")))
		return ErrRateLimited
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("show directory returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxResponse {
		return errors.New("show directory response too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("show directory response read failed")
	}
	if len(data) > maxResponse {
		return errors.New("show directory response too large")
	}
	if err := json.Unmarshal(data, document); err != nil {
		return errors.New("invalid show directory response")
	}
	return nil
}
func normalize(records []appleRecord) []Show {
	shows := make([]Show, 0, maxResults)
	seen := make(map[string]bool)
	for _, result := range records {
		if len(result.FeedURL) > 4096 {
			continue
		}
		feedURL, err := httpurl.Parse(result.FeedURL)
		if err != nil {
			continue
		}
		feedURL.Fragment = ""
		if seen[feedURL.String()] {
			continue
		}
		title := strings.TrimSpace(result.CollectionName)
		if title == "" {
			title = strings.TrimSpace(result.TrackName)
		}
		if title == "" {
			continue
		}
		if len([]rune(title)) > 512 {
			title = string([]rune(title)[:512])
		}
		source := ""
		if u, err := httpurl.Parse(result.CollectionViewURL); err == nil && len(result.CollectionViewURL) <= 4096 {
			source = u.String()
		}
		seen[feedURL.String()] = true
		shows = append(shows, Show{Title: title, FeedURL: feedURL.String(), SourceURL: source})
		if len(shows) == maxResults {
			break
		}
	}
	return shows
}
func retryDelay(value string) time.Duration {
	delay := 3 * time.Second
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds >= 30 {
			return 30 * time.Second
		}
		if seconds > 0 {
			delay = time.Duration(seconds) * time.Second
		}
	} else if date, err := http.ParseTime(value); err == nil {
		delay = time.Until(date)
	}
	return max(3*time.Second, min(30*time.Second, delay))
}
