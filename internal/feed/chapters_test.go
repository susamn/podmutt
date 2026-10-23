package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseRSSChaptersPodcasting20AndPodlove(t *testing.T) {
	xmlData := []byte(`
	<rss version="2.0" xmlns:podcast="http://podcastindex.org/namespace/1.0" xmlns:psc="http://podlove.org/simple-chapters">
		<channel>
			<title>Chapters Feed</title>
			<item>
				<guid>ep1</guid>
				<title>Podcasting 2.0 Episode</title>
				<enclosure url="https://example.com/ep1.mp3" type="audio/mpeg" />
				<podcast:chapters url="https://example.com/ep1-chapters.json" type="application/json+chapters" />
			</item>
			<item>
				<guid>ep2</guid>
				<title>Podlove Episode</title>
				<enclosure url="https://example.com/ep2.mp3" type="audio/mpeg" />
				<psc:chapters version="1.2">
					<psc:chapter start="00:00:00" title="Intro" />
					<psc:chapter start="00:03:00" title="Discussion" />
				</psc:chapters>
			</item>
			<item>
				<guid>ep3</guid>
				<title>Regular Episode</title>
				<enclosure url="https://example.com/ep3.mp3" type="audio/mpeg" />
			</item>
		</channel>
	</rss>`)

	doc, err := Parse(xmlData, "https://example.com/feed.xml")
	if err != nil {
		t.Fatal(err)
	}

	if len(doc.Episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(doc.Episodes))
	}

	// Episode 1 has chapters_url
	if doc.Episodes[0].ChaptersURL != "https://example.com/ep1-chapters.json" {
		t.Fatalf("expected chapters URL, got %q", doc.Episodes[0].ChaptersURL)
	}
	if !doc.Episodes[0].HasChapters() {
		t.Fatal("expected ep1 HasChapters to be true")
	}

	// Episode 2 has inline Podlove chapters in chapters_json
	if !strings.Contains(doc.Episodes[1].ChaptersJSON, "Discussion") {
		t.Fatalf("expected chapters JSON with Discussion, got %q", doc.Episodes[1].ChaptersJSON)
	}
	parsed := doc.Episodes[1].ParsedChapters()
	if len(parsed) != 2 || parsed[0].Title != "Intro" || parsed[1].Title != "Discussion" {
		t.Fatalf("unexpected parsed chapters: %+v", parsed)
	}
	if !doc.Episodes[1].HasChapters() {
		t.Fatal("expected ep2 HasChapters to be true")
	}

	// Episode 3 has neither
	if doc.Episodes[2].HasChapters() || doc.Episodes[2].ChaptersURL != "" || doc.Episodes[2].ChaptersJSON != "" {
		t.Fatalf("expected no chapters for ep3: %+v", doc.Episodes[2])
	}
}

func TestFetchChaptersHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chapters.json" {
			w.Header().Set("Content-Type", "application/json+chapters")
			fmt.Fprint(w, `{"version": "1.2.0", "chapters": [{"startTime": 0, "title": "Intro"}, {"startTime": 60, "title": "Topic"}]}`)
			return
		}
		if r.URL.Path == "/500.json" {
			w.WriteHeader(500)
			return
		}
		if r.URL.Path == "/bad.json" {
			fmt.Fprint(w, `not json`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	r := New(nil, nil, 0)

	// Valid chapters
	list, err := r.FetchChapters(context.Background(), server.URL+"/chapters.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Title != "Intro" || list[1].Start != 60 {
		t.Fatalf("unexpected chapters: %+v", list)
	}

	// 500 status
	if _, err := r.FetchChapters(context.Background(), server.URL+"/500.json"); err == nil {
		t.Fatal("expected error on 500")
	}

	// Bad json
	if _, err := r.FetchChapters(context.Background(), server.URL+"/bad.json"); err == nil {
		t.Fatal("expected error on malformed json")
	}

	// Invalid URL
	if _, err := r.FetchChapters(context.Background(), "not-a-url"); err == nil {
		t.Fatal("expected error on invalid URL")
	}

	// Canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.FetchChapters(ctx, server.URL+"/chapters.json"); err == nil {
		t.Fatal("expected error on canceled context")
	}

	// Timeout context
	ctxTime, cancelTime := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelTime()
	time.Sleep(time.Millisecond)
	if _, err := r.FetchChapters(ctxTime, server.URL+"/chapters.json"); err == nil {
		t.Fatal("expected error on timeout")
	}
}
