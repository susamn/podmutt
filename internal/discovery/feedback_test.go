package discovery

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"podmutt/internal/feedback"
)

type deadlineBody struct{ ctx context.Context }

func (b deadlineBody) Read([]byte) (int, error) { <-b.ctx.Done(); return 0, b.ctx.Err() }
func (b deadlineBody) Close() error             { return nil }

func TestSearchDeadlineIncludesGateBackoffHeadersAndBody(t *testing.T) {
	for _, phase := range []string{"gate", "backoff", "headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			var requests atomic.Int32
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				requests.Add(1)
				if phase == "headers" {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: deadlineBody{r.Context()}}, nil
			})}
			a := NewApple(client)
			if phase == "gate" {
				a.gate <- struct{}{}
				defer func() { <-a.gate }()
			}
			if phase == "backoff" {
				a.next = time.Now().Add(time.Minute)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			_, err := a.Search(ctx, "science")
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("timeout identity lost", err)
			}
			m := feedback.Retry(feedback.For("Apple Podcasts search", err), "Press R to retry.")
			if m.Code != feedback.Timeout || !strings.Contains(m.Text(), "timed out") || strings.Contains(m.Text(), "context deadline") {
				t.Fatal(m)
			}
			if (phase == "gate" || phase == "backoff") && requests.Load() != 0 {
				t.Fatal("deadline made an HTTP request")
			}
		})
	}
}

func TestRatingsAndSearchShareQuotaGateAndCountryIsRaceSafe(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var active, peak atomic.Int32
	a := NewApple(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); current > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, current) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"results":[],"feed":{"entry":[]}}`))}, nil
	})})
	a.interval = 0
	var wg sync.WaitGroup
	wg.Go(func() { _, _ = a.Search(context.Background(), "science") })
	wg.Go(func() { _, _, _ = a.Ratings(context.Background(), "us", "123") })
	wg.Go(func() {
		for range 100 {
			a.SetCountry("de")
			_ = a.Country()
			a.SetCountry("us")
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("no request")
	}
	select {
	case <-entered:
		close(release)
		wg.Wait()
		t.Fatal("ratings bypassed search quota gate")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatal("concurrent provider quota use", peak.Load())
	}
}

func TestProviderErrorsKeepCategoriesWithoutLeakingResponses(t *testing.T) {
	for _, status := range []int{401, 403, 404, 429, 503, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			a := NewApple(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("malformed private-secret response"))}, nil
			})})
			_, err := a.Search(context.Background(), "science")
			if err == nil {
				t.Fatal("bad response accepted")
			}
			message := feedback.For("Apple Podcasts search", err)
			want := feedback.HTTP
			if status == 429 {
				want = feedback.RateLimited
			}
			if status == 200 {
				want = feedback.InvalidResponse
			}
			if message.Code != want || strings.Contains(message.Text(), "private-secret") {
				t.Fatal(message)
			}
		})
	}
}
