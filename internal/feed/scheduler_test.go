package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func resultWithin(t *testing.T, results <-chan Result) Result {
	t.Helper()
	select {
	case result, ok := <-results:
		if !ok {
			t.Fatal("results closed early")
		}
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("refresh result timed out")
	}
	return Result{}
}
func closedWithin(t *testing.T, results <-chan Result) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-results:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("refresh did not stop")
		}
	}
}

func TestStartupManualAndPeriodicRefresh(t *testing.T) {
	var requests atomic.Int32
	entered, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			entered <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, rssFixture)
	}))
	defer server.Close()
	db := database(t)
	addFeed(t, db, server.URL)
	r := New(db, nil, 0)
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	ticks := make(chan time.Time, 1)
	var stopped atomic.Bool
	results, err := r.start(lifetime, ticks, func() { stopped.Store(true) })
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("no startup refresh")
	}
	if _, err := r.Start(lifetime); err == nil {
		t.Fatal("duplicate scheduler started")
	}
	for range 20 {
		r.RefreshNow()
	}
	close(release)
	for range 2 {
		if result := resultWithin(t, results); result.Error != nil {
			t.Fatal(result.Error)
		}
	}
	select {
	case result := <-results:
		t.Fatalf("manual requests did not coalesce: %+v", result)
	case <-time.After(20 * time.Millisecond):
	}
	ticks <- time.Now()
	if result := resultWithin(t, results); result.Error != nil {
		t.Fatal(result.Error)
	}
	cancel()
	closedWithin(t, results)
	if !stopped.Load() || requests.Load() != 3 {
		t.Fatalf("scheduler lifecycle: stopped=%v requests=%d", stopped.Load(), requests.Load())
	}
}

func TestConcurrencyBoundAndCancellation(t *testing.T) {
	var active, peak atomic.Int32
	entered := make(chan struct{}, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); current > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, current) {
				break
			}
		}
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	db := database(t)
	for i := range 8 {
		addFeed(t, db, fmt.Sprintf("%s/%d", server.URL, i))
	}
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	results, err := New(db, nil, time.Hour).Start(lifetime)
	if err != nil {
		t.Fatal(err)
	}
	for range parallelism {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	select {
	case <-entered:
		t.Fatal("more than four requests started concurrently")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	closedWithin(t, results)
	if peak.Load() > parallelism {
		t.Fatalf("concurrency = %d", peak.Load())
	}
	for deadline := time.Now().Add(time.Second); active.Load() != 0; {
		if time.Now().After(deadline) {
			t.Fatal("HTTP work survived cancellation")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestIncrementalResultsAndPartialFailure(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad":
			w.WriteHeader(503)
			return
		case "/slow":
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, rssFixture)
	}))
	defer server.Close()
	db := database(t)
	for _, path := range []string{"/fast", "/bad", "/slow"} {
		addFeed(t, db, server.URL+path)
	}
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	results, err := New(db, nil, time.Hour).Start(lifetime)
	if err != nil {
		t.Fatal(err)
	}
	failures := 0
	for range 2 {
		if result := resultWithin(t, results); result.Error != nil {
			failures++
		}
	}
	if failures != 1 {
		t.Fatal("partial failure blocked independent results")
	}
	close(release)
	if result := resultWithin(t, results); result.Error != nil {
		t.Fatal(result.Error)
	}
	cancel()
	closedWithin(t, results)
}

func TestListingFailureAndEmptyLibrary(t *testing.T) {
	db := database(t)
	for _, repo := range []Repository{brokenRepository{db, "list"}, db} {
		lifetime, cancel := context.WithCancel(ctx)
		results, err := New(repo, nil, time.Hour).Start(lifetime)
		if err != nil {
			t.Fatal(err)
		}
		if _, broken := repo.(brokenRepository); broken {
			if result := resultWithin(t, results); result.FeedID != 0 || result.Error == nil {
				t.Fatal("global refresh failure not reported")
			}
		}
		cancel()
		closedWithin(t, results)
	}
}
