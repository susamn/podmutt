package artwork

import (
	"container/list"
	"context"
	"errors"
	"image"
	"io"
	"net/http"
	"sync"
	"time"

	"podmutt/internal/activity"
	"podmutt/internal/httpurl"
)

var ErrUnavailable = errors.New("artwork unavailable")

type cacheEntry struct {
	key     string
	data    []byte
	expires time.Time
}

// Fetcher owns a per-foreground, compressed-byte LRU (8 MiB, at most 32 images).
// Images returned to callers are independent decodes, not mutable cached handles.
// It does no disk writes and starts no background goroutines. Call Image from a
// cancelable UI command; at most two loads/decodes can run concurrently.
type Fetcher struct {
	client        *http.Client
	gate          chan struct{}
	mu            sync.Mutex
	cache         map[string]*list.Element
	lru           *list.List
	bytes, budget int
}

func New(client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	if copy.Timeout <= 0 || copy.Timeout > 10*time.Second {
		copy.Timeout = 10 * time.Second
	}
	previous := copy.CheckRedirect
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
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
	return &Fetcher{client: &copy, gate: make(chan struct{}, 2), cache: make(map[string]*list.Element), lru: list.New(), budget: 8 << 20}
}
func (f *Fetcher) Image(ctx context.Context, episodeURL, showURL string) (image.Image, error) {
	ctx, finish := activity.Begin(ctx, "Loading artwork…", activity.Background)
	defer finish()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case f.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-f.gate }()
	seen := make(map[string]bool)
	for _, address := range []string{episodeURL, showURL} {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if len(address) > 4096 {
			continue
		}
		u, err := httpurl.Parse(address)
		if err != nil || seen[u.String()] {
			continue
		}
		address = u.String()
		seen[address] = true
		data := f.cached(address)
		if data == nil {
			data, err = f.download(ctx, address)
			if err != nil {
				continue
			}
		}
		img, err := Decode(data)
		if err != nil {
			continue
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		f.remember(address, data)
		return img, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, ErrUnavailable
}
func (f *Fetcher) download(ctx context.Context, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.Header.Set("User-Agent", "podmutt/dev")
	response, err := f.client.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > MaxBytes {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
	if err != nil || len(data) > MaxBytes {
		return nil, ErrUnavailable
	}
	return data, nil
}
func (f *Fetcher) cached(key string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if element := f.cache[key]; element != nil {
		entry := element.Value.(cacheEntry)
		if time.Now().Before(entry.expires) {
			f.lru.MoveToFront(element)
			return entry.data
		}
		f.remove(element)
	}
	return nil
}
func (f *Fetcher) remove(element *list.Element) {
	entry := element.Value.(cacheEntry)
	f.bytes -= len(entry.data)
	delete(f.cache, entry.key)
	f.lru.Remove(element)
}
func (f *Fetcher) remember(key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if element := f.cache[key]; element != nil {
		f.remove(element)
	}
	if len(data) > f.budget {
		return
	}
	for f.bytes+len(data) > f.budget || f.lru.Len() >= 32 {
		f.remove(f.lru.Back())
	}
	f.cache[key] = f.lru.PushFront(cacheEntry{key, data, time.Now().Add(24 * time.Hour)})
	f.bytes += len(data)
}
