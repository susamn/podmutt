package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTopBatchLookupPreservesChartOrderAndCaches(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/chart" {
			fmt.Fprint(w, `{"feed":{"results":[{"id":"20"},{"id":"20"},{"id":"bad"},{"id":"0"},{"id":"10"}]}}`)
			return
		}
		if r.URL.Query().Get("id") != "20,10" || r.URL.Query().Get("entity") != "podcast" {
			t.Error(r.URL.Query())
		}
		fmt.Fprint(w, `{"results":[{"collectionId":10,"collectionName":"Second","feedUrl":"https://example.test/second"},{"collectionId":99,"collectionName":"Not in chart","feedUrl":"https://example.test/other"},{"collectionId":20,"collectionName":"First","feedUrl":"https://example.test/first","collectionViewUrl":"https://podcasts.apple.com/us/podcast/example/id20"}]}`)
	}))
	defer server.Close()
	a := NewApple(nil)
	a.chartEndpoint = server.URL + "/chart"
	a.lookupEndpoint = server.URL + "/lookup"
	a.interval = 0
	shows, err := a.Top(context.Background())
	if err != nil || len(shows) != 2 || shows[0].Title != "First" {
		t.Fatal(shows, err)
	}
	shows[0].Title = "caller mutation"
	again, err := a.Top(context.Background())
	if err != nil || again[0].Title != "First" || calls.Load() != 2 {
		t.Fatal(again, err, calls.Load())
	}
}
func TestTopMalformedMissingLookupAndQuotaCancellation(t *testing.T) {
	for _, chart := range []string{`{`, `{"feed":{"results":[]}}`, `{"feed":{"results":[{"id":"1"}]}}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/chart" {
				fmt.Fprint(w, chart)
			} else {
				fmt.Fprint(w, `{"results":[]}`)
			}
		}))
		a := NewApple(nil)
		a.interval = 0
		a.chartEndpoint = server.URL + "/chart"
		a.lookupEndpoint = server.URL + "/lookup"
		if _, err := a.Top(context.Background()); err == nil {
			t.Fatal("bad chart/lookup accepted")
		}
		server.Close()
	}
	var lookup atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chart" {
			fmt.Fprint(w, `{"feed":{"results":[{"id":"1"}]}}`)
		} else {
			lookup.Add(1)
		}
	}))
	defer server.Close()
	a := NewApple(nil)
	a.chartEndpoint = server.URL + "/chart"
	a.lookupEndpoint = server.URL + "/lookup"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := a.Top(ctx); !errors.Is(err, context.DeadlineExceeded) || lookup.Load() != 0 {
		t.Fatal(err, lookup.Load())
	}
}
func TestTopLimitsIDBatchAndRedactsLookupFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chart" {
			fmt.Fprint(w, `{"feed":{"results":[`)
			for i := 1; i <= 30; i++ {
				if i > 1 {
					fmt.Fprint(w, ",")
				}
				fmt.Fprintf(w, `{"id":"%d"}`, i)
			}
			fmt.Fprint(w, `]}}`)
		} else {
			if len(strings.Split(r.URL.Query().Get("id"), ",")) != 25 {
				t.Error("batch cap ignored")
			}
			w.WriteHeader(503)
			fmt.Fprint(w, "secret failure")
		}
	}))
	defer server.Close()
	a := NewApple(nil)
	a.interval = 0
	a.chartEndpoint = server.URL + "/chart"
	a.lookupEndpoint = server.URL + "/lookup"
	if _, err := a.Top(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	a.lookupEndpoint = ":"
	if _, err := a.Top(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
