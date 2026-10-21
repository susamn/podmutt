package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"podmutt/internal/activity"
)

func TestScheduledRefreshReportsBatchThroughSharedTracker(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
			fmt.Fprint(w, rssFixture)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	tracker := activity.New()
	lifetime, cancel := context.WithCancel(activity.WithTracker(context.Background(), tracker))
	defer cancel()
	db := database(t)
	addFeed(t, db, server.URL+"/a")
	addFeed(t, db, server.URL+"/b")
	results, err := New(db, nil, time.Hour).Start(lifetime)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("refresh not started")
		}
	}
	snapshot := tracker.Snapshot()
	if len(snapshot.Operations) != 1 || snapshot.Operations[0].Label != "Refreshing feeds…" || snapshot.Operations[0].Total != 2 || snapshot.Operations[0].Completed != 0 {
		t.Fatal(snapshot)
	}
	close(release)
	for range 2 {
		if result := resultWithin(t, results); result.Error != nil {
			t.Fatal(result.Error)
		}
	}
	// Cancellation cleans activity even while the scheduler's worker goroutine unwinds.
	cancel()
	closedWithin(t, results)
	if len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("refresh stuck busy")
	}
}
