package discovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDirectoryRequestNormalizationAndCacheIsolation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if q.Get("media") != "podcast" || q.Get("entity") != "podcast" || q.Get("limit") != "25" || q.Get("term") != "Go podcast" {
			t.Error(q)
		}
		fmt.Fprint(w, `{"results":[{"collectionName":"Show","feedUrl":"https://example.test/rss","collectionViewUrl":"https://podcasts.apple.com/example"},{"trackName":"fallback","feedUrl":"https://example.test/other"},{"collectionName":"dup","feedUrl":"https://example.test/rss"},{"collectionName":"bad","feedUrl":"https://user:secret@example.test/rss"},{"collectionName":"bad","feedUrl":"file:///secret"},{"feedUrl":"https://example.test/no-title"}]}`)
	}))
	defer server.Close()
	a := NewApple(nil)
	a.endpoint = server.URL
	a.interval = 0
	first, err := a.Search(context.Background(), "  Go   podcast\n")
	if err != nil || len(first) != 2 || first[1].Title != "fallback" || first[0].SourceURL == "" {
		t.Fatal(first, err)
	}
	first[0].Title = "mutated"
	second, err := a.Search(context.Background(), "go podcast")
	if err != nil || second[0].Title != "Show" || calls.Load() != 1 {
		t.Fatal(second, err, calls.Load())
	}
	a.cache["go podcast"] = cached{expires: time.Now().Add(-time.Second)}
	if _, err := a.Search(context.Background(), "Go podcast"); err != nil || calls.Load() != 2 {
		t.Fatal(err, calls.Load())
	}
	if _, err := a.SearchFresh(context.Background(), "Go podcast"); err != nil || calls.Load() != 3 {
		t.Fatal("fresh request reused provider TTL", err, calls.Load())
	}
	for _, query := range []string{"  ", strings.Repeat("界", 129)} {
		if _, err := a.Search(context.Background(), query); !errors.Is(err, ErrQuery) {
			t.Fatal(err)
		}
	}
}
func TestTopLimitFreshBypassesInMemoryCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/chart" {
			fmt.Fprint(w, `{"feed":{"results":[{"id":"42"}]}}`)
			return
		}
		fmt.Fprint(w, `{"results":[{"collectionId":42,"collectionName":"Fresh chart","feedUrl":"https://example.test/rss"}]}`)
	}))
	defer server.Close()
	a := NewApple(nil)
	a.chartEndpoint = server.URL + "/chart"
	a.lookupEndpoint = server.URL + "/lookup"
	a.interval = 0
	a.cache["\x00top-us-100"] = cached{shows: []Show{{Title: "stale", FeedURL: "https://example.test/stale"}}, expires: time.Now().Add(time.Hour)}
	shows, err := a.TopLimitFresh(context.Background(), 100)
	if err != nil || calls.Load() != 2 || len(shows) != 1 || shows[0].Title != "Fresh chart" {
		t.Fatal(shows, err, calls.Load())
	}
}

func TestDirectoryBoundsMalformedAndRateLimits(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"bad JSON", 200, `{"results":`, "invalid"}, {"HTTP", 503, "secret response", "HTTP 503"},
		{"rate", 429, "secret response", "rate limited"}, {"large", 200, strings.Repeat("x", maxResponse+1), "too large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "999999999")
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			a := NewApple(nil)
			a.endpoint = server.URL
			a.interval = 0
			if _, err := a.Search(context.Background(), "test"); err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "secret") {
				t.Fatal(err)
			}
			if test.status == 429 && time.Until(a.next) < 25*time.Second {
				t.Fatal("ignored retry after")
			}
		})
	}
}
func TestDirectoryCapCacheEvictionAndFieldBounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[`)
		for i := 0; i < 30; i++ {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"collectionName":%q,"feedUrl":"https://example.test/%d","collectionViewUrl":"bad"}`, strings.Repeat("界", 600), i)
		}
		fmt.Fprint(w, `]}`)
	}))
	defer server.Close()
	a := NewApple(nil)
	a.endpoint = server.URL
	a.interval = 0
	for i := 0; i < 128; i++ {
		a.cache[fmt.Sprint(i)] = cached{}
	}
	shows, err := a.Search(context.Background(), "test")
	if err != nil || len(shows) != 25 || len([]rune(shows[0].Title)) != 512 || shows[0].SourceURL != "" || len(a.cache) != 1 {
		t.Fatal(len(shows), err)
	}
}
func TestDirectoryCancellationQuotaAndRedirects(t *testing.T) {
	a := NewApple(nil)
	a.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := a.Search(ctx, "test"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	<-a.gate
	a.next = time.Now().Add(time.Second)
	reserved := a.next
	ctx, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()
	if _, err := a.Search(ctx, "test"); !errors.Is(err, context.DeadlineExceeded) || a.next != reserved {
		t.Fatal(err, "canceled search reserved quota")
	}
	canceled, cancel3 := context.WithCancel(context.Background())
	cancel3()
	if _, err := a.Search(canceled, "test"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://user:secret@example.test/path", http.StatusFound)
	}))
	defer server.Close()
	a = NewApple(nil)
	a.endpoint = server.URL
	if _, err := a.Search(context.Background(), "private query"); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "https://example.test", nil)
	if err := a.client.CheckRedirect(req, make([]*http.Request, 6)); err == nil {
		t.Fatal("redirect loop allowed")
	}
	sentinel := errors.New("caller limit")
	a = NewApple(&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return sentinel }})
	if err := a.client.CheckRedirect(req, nil); err != sentinel {
		t.Fatal(err)
	}
}
func TestRetryDelay(t *testing.T) {
	for _, value := range []string{"", "invalid", "-100", "999999999", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)} {
		delay := retryDelay(value)
		if delay < 3*time.Second || delay > 30*time.Second {
			t.Fatal(value, delay)
		}
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, errors.New("secret reader") }
func (brokenBody) Close() error             { return nil }
func TestDirectoryReadAndRequestErrors(t *testing.T) {
	for _, test := range []struct {
		body    io.ReadCloser
		length  int64
		failure bool
	}{
		{body: brokenBody{}}, {body: io.NopCloser(strings.NewReader("")), length: maxResponse + 1}, {failure: true},
	} {
		a := NewApple(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			if test.failure {
				return nil, errors.New("secret network")
			}
			return &http.Response{StatusCode: 200, Body: test.body, ContentLength: test.length, Header: make(http.Header)}, nil
		})})
		if _, err := a.Search(context.Background(), "test"); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
	a := NewApple(nil)
	a.endpoint = ":"
	if _, err := a.Search(context.Background(), "test"); err == nil {
		t.Fatal("bad endpoint")
	}
}
