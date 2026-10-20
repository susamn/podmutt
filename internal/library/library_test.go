package library

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"podmutt/internal/feed"
	"podmutt/internal/store"
)

var ctx = context.Background()
var failure = errors.New("injected failure")

type fixtureFetcher struct {
	calls     int
	err       error
	unchanged bool
}

func (f *fixtureFetcher) Fetch(context.Context, store.Feed) (feed.Fetched, error) {
	f.calls++
	return feed.Fetched{Document: feed.Document{Title: "A show", Episodes: []store.Episode{
		{GUID: "a", Title: "Zulu", AudioURL: "https://example.test/a", PublishedAt: time.Unix(100, 0)},
		{GUID: "b", Title: "Alpha", AudioURL: "https://example.test/b", PublishedAt: time.Unix(200, 0)},
	}}, ETag: `"v1"`, NotModified: f.unchanged}, f.err
}
func database(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func TestAddSubscriptionViewsAndHistory(t *testing.T) {
	db := database(t)
	fetcher := &fixtureFetcher{}
	s := New(db, fetcher)
	id, err := s.Add(ctx, " https://example.test/feed#ignored ")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Read(ctx, id)
	if err != nil || len(snapshot.New) != 2 || len(snapshot.Show) != 2 || snapshot.Feeds[0].URL != "https://example.test/feed" {
		t.Fatalf("view: %+v, %v", snapshot, err)
	}
	episode := snapshot.Show[0]
	if err := db.Checkpoint(ctx, episode.ID, 12); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, episode.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Read(ctx, id)
	if err != nil || len(snapshot.New) != 0 || len(snapshot.Show) != 2 {
		t.Fatal("unsubscribed cache not available")
	}
	again, err := s.Add(ctx, "https://example.test/feed")
	if err != nil || again != id || fetcher.calls != 1 {
		t.Fatalf("duplicate add: %d %d %v", again, fetcher.calls, err)
	}
	snapshot, err = s.Read(ctx, id)
	if err != nil || len(snapshot.New) != 1 || !snapshot.Show[0].Completed || snapshot.Show[0].PositionSeconds != 12 || snapshot.Feeds[0].ETag != `"v1"` {
		t.Fatal("resubscribe lost metadata/history")
	}
}
func TestLibraryDestinationsHaveDistinctSources(t *testing.T) {
	db := database(t)
	s := New(db, &fixtureFetcher{})
	id, err := s.Add(ctx, "https://example.test/feed")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Checkpoint(ctx, snapshot.Show[0].ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, snapshot.Show[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertEpisodes(ctx, id, []store.Episode{{GUID: "c", Title: "Unstarted", AudioURL: "https://example.test/c"}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for view, count := range map[View]int{ShowView: 3, NewView: 1, ContinueView: 1, AllView: 3} {
		if rows := Episodes(snapshot, view, Newest); len(rows) != count || view.String() == "" {
			t.Fatal("wrong destination source", view, rows)
		}
	}
	if err := s.Subscribe(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Read(ctx, 0)
	if err != nil || len(snapshot.All) != 0 || len(snapshot.New) != 0 || len(snapshot.Continue) != 1 {
		t.Fatal("unsubscribe lost continuation or leaked subscriptions", snapshot, err)
	}
}

func TestSortCopiesAndDeterministicTies(t *testing.T) {
	snapshot := Snapshot{Show: []store.Episode{
		{ID: 2, Title: "<b>Zulu</b>", PublishedAt: time.Unix(100, 0)},
		{ID: 1, Title: "alpha", PublishedAt: time.Unix(200, 0)},
		{ID: 3, Title: "Alpha", PublishedAt: time.Unix(200, 0)},
	}}
	for order, want := range map[Order][]int64{Newest: {1, 3, 2}, Oldest: {2, 1, 3}, Title: {1, 3, 2}} {
		rows := Episodes(snapshot, ShowView, order)
		for i, id := range want {
			if rows[i].ID != id {
				t.Fatalf("%s: %+v", order, rows)
			}
		}
	}
	if snapshot.Show[0].ID != 2 {
		t.Fatal("sorting mutated snapshot")
	}
	if len(Episodes(snapshot, NewView, Newest)) != 0 {
		t.Fatal("new tab used show rows")
	}
}

type brokenRepo struct {
	Repository
	op     string
	writes int
}

func (r *brokenRepo) Feeds(ctx context.Context, only bool) ([]store.Feed, error) {
	if r.op == "feeds" {
		return nil, failure
	}
	return r.Repository.Feeds(ctx, only)
}
func (r *brokenRepo) Episodes(ctx context.Context, id int64) ([]store.Episode, error) {
	if (r.op == "all" && id == 0) || (r.op == "show" && id != 0) {
		return nil, failure
	}
	return r.Repository.Episodes(ctx, id)
}
func (r *brokenRepo) InProgress(ctx context.Context) ([]store.Episode, error) {
	if r.op == "continue" {
		return nil, failure
	}
	return r.Repository.InProgress(ctx)
}
func (r *brokenRepo) UpsertFeed(ctx context.Context, source store.Feed) (int64, error) {
	r.writes++
	if r.op == "feed" || (r.op == "validators" && r.writes == 2) {
		return 0, failure
	}
	return r.Repository.UpsertFeed(ctx, source)
}
func (r *brokenRepo) UpsertEpisodes(ctx context.Context, id int64, items []store.Episode) error {
	if r.op == "episodes" {
		return failure
	}
	return r.Repository.UpsertEpisodes(ctx, id, items)
}
func (r *brokenRepo) SetSubscribed(ctx context.Context, id int64, yes bool) error {
	if r.op == "subscribe" {
		return failure
	}
	return r.Repository.SetSubscribed(ctx, id, yes)
}

func TestReadAndWriteFailures(t *testing.T) {
	for _, op := range []string{"feeds", "all", "show", "continue", "feed", "episodes", "validators", "subscribe"} {
		t.Run(op, func(t *testing.T) {
			db := database(t)
			s := New(&brokenRepo{Repository: db, op: op}, &fixtureFetcher{})
			if op == "all" || op == "show" || op == "continue" {
				if _, err := s.Read(ctx, 1); !errors.Is(err, failure) {
					t.Fatal(err)
				}
				return
			}
			if _, err := s.Add(ctx, "https://example.test/feed"); !errors.Is(err, failure) {
				t.Fatalf("%s: %v", op, err)
			}
			feeds, err := db.Feeds(ctx, true)
			if err != nil || len(feeds) != 0 {
				t.Fatal("failed add subscribed incomplete data")
			}
		})
	}
}
func TestBadFeedDoesNotCreateRows(t *testing.T) {
	db := database(t)
	for _, fetcher := range []*fixtureFetcher{{err: failure}, {unchanged: true}} {
		if _, err := New(db, fetcher).Add(ctx, "https://example.test/feed"); err == nil {
			t.Fatal("invalid fetch accepted")
		}
	}
	if _, err := New(db, &fixtureFetcher{}).Add(ctx, "file:///tmp/feed"); err == nil {
		t.Fatal("invalid URL accepted")
	}
	if feeds, err := db.Feeds(ctx, false); err != nil || len(feeds) != 0 {
		t.Fatal("bad feed created rows")
	}
	if _, err := New(&brokenRepo{Repository: db, op: "feeds"}, nil).Read(ctx, 0); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func TestHTMLToPlainText(t *testing.T) {
	for raw, want := range map[string]string{
		"plain": "plain", "<p>Hello &amp; friends</p><br><div>World</div>": "Hello & friends\n\nWorld",
		"<style>hidden</style><script>hidden</script><b>Visible</b>": "Visible",
		"2 < 3 &amp; 4": "2 < 3 & 4",
	} {
		if got := PlainText(raw); got != want {
			t.Errorf("PlainText(%q)=%q, want %q", raw, got, want)
		}
	}
}

func TestAddHealsEmptyFeed(t *testing.T) {
	db := database(t)
	fetcher := &fixtureFetcher{}
	service := New(db, fetcher)

	id, err := db.UpsertFeed(ctx, store.Feed{URL: "https://example.test/feed", Title: "Empty Feed"})
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := db.Episodes(ctx, id)
	if err != nil || len(episodes) != 0 {
		t.Fatalf("expected 0 episodes, got %v", episodes)
	}

	addedID, err := service.Add(ctx, "https://example.test/feed")
	if err != nil || addedID != id {
		t.Fatalf("Add failed: %v, addedID=%d, want=%d", err, addedID, id)
	}
	episodes, err = db.Episodes(ctx, id)
	if err != nil || len(episodes) != 2 {
		t.Fatalf("expected 2 episodes populated, got %v", episodes)
	}
}
