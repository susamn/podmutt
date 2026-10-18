// Package discovery isolates the optional, no-key online show directory.
// It never searches remote episodes, downloads previews, or changes subscriptions.
package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"podmutt/internal/httpurl"
)

const maxResponse = 1 << 20
const maxResults = 25

var ErrQuery = errors.New("show search requires 1–128 characters")
var ErrUnavailable = errors.New("show search unavailable (network, TLS, or redirect error)")
var ErrRateLimited = errors.New("show search rate limited; try again later")

type Show struct{ Title, FeedURL, SourceURL string }
type cached struct {
	shows   []Show
	expires time.Time
}

// Apple serializes requests per foreground instance and spaces actual outbound
// calls. Cancellation while waiting never reserves future quota slots.
type Apple struct {
	client   *http.Client
	endpoint string
	gate     chan struct{}
	interval time.Duration
	next     time.Time
	cache    map[string]cached
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
	return &Apple{client: &copy, endpoint: "https://itunes.apple.com/search", gate: make(chan struct{}, 1), interval: 3 * time.Second, cache: make(map[string]cached)}
}
func (a *Apple) Search(ctx context.Context, query string) ([]Show, error) {
	query = strings.Join(strings.Fields(query), " ")
	if size := len([]rune(query)); size == 0 || size > 128 {
		return nil, ErrQuery
	}
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
	key := strings.ToLower(query)
	if entry, ok := a.cache[key]; ok && time.Now().Before(entry.expires) {
		return append([]Show(nil), entry.shows...), nil
	}
	if delay := time.Until(a.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.Header.Set("User-Agent", "podmutt/dev")
	req.Header.Set("Accept", "application/json")
	a.next = time.Now().Add(a.interval)
	response, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		a.next = time.Now().Add(retryDelay(response.Header.Get("Retry-After")))
		return nil, ErrRateLimited
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("show search returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxResponse {
		return nil, errors.New("show search response too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("show search response read failed")
	}
	if len(data) > maxResponse {
		return nil, errors.New("show search response too large")
	}
	var document struct {
		Results []struct{ CollectionName, TrackName, FeedURL, CollectionViewURL string }
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, errors.New("invalid show search response")
	}
	shows := make([]Show, 0, maxResults)
	seen := make(map[string]bool)
	for _, result := range document.Results {
		if len(result.FeedURL) > 4096 {
			continue
		}
		feedURL, err := httpurl.Parse(result.FeedURL)
		if err != nil || seen[feedURL.String()] {
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
	if len(a.cache) >= 128 {
		a.cache = make(map[string]cached)
	}
	a.cache[key] = cached{shows: append([]Show(nil), shows...), expires: time.Now().Add(10 * time.Minute)}
	return shows, nil
}
func retryDelay(value string) time.Duration {
	delay := 3 * time.Second
	if seconds, err := strconv.Atoi(value); err == nil {
		// Clamp before multiplying so a malicious header cannot overflow Duration.
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
