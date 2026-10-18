package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResolveWebsiteAlternatesRedirectBaseAndFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/site/page", 302)
		case "/site/page":
			fmt.Fprint(w, `<!doctype html><html><head><base href="/feeds/"><script>fake</script><LINK REL="ALTERNATE" TYPE="application/rss+xml; charset=utf-8" HREF="bad"><link rel="alternate" type="application/atom+xml" href="ok"></head><body>Welcome</body></html>`)
		case "/feeds/bad":
			w.WriteHeader(404)
		case "/feeds/ok":
			fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><title>Resolved show</title><entry><id>one</id><title>Episode</title><link rel="enclosure" href="/audio.mp3" type="audio/mpeg"/></entry></feed>`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	r := New(nil, server.Client(), 0)
	result, err := r.Resolve(context.Background(), server.URL+"/start#ignored")
	if err != nil || result.URL != server.URL+"/feeds/ok" || result.Fetched.Document.Title != "Resolved show" || len(result.Fetched.Document.Episodes) != 1 {
		t.Fatal(result, err)
	}
	direct, err := r.Resolve(context.Background(), result.URL)
	if err != nil || direct.URL != result.URL {
		t.Fatal(direct, err)
	}
}
func TestAlternateDiscoveryBoundsAndUnsafeLinks(t *testing.T) {
	body := `<html><head><!-- <link rel="alternate" type="application/rss+xml" href="/comment"> --><link rel="alternate" type="application/rss+xml" href="file:///secret"><link rel="alternate" type="application/rss+xml" href="https://user:secret@example.test/rss"><link rel="icon" type="application/rss+xml" href="/icon">`
	for i := 0; i < 8; i++ {
		body += fmt.Sprintf(`<link rel="alternate" type="application/rss+xml" href="/%d">`, i)
	}
	links := alternateFeeds([]byte(body), "https://example.test/site")
	if len(links) != 3 || links[0] != "https://example.test/0" {
		t.Fatal(links)
	}
	if len(alternateFeeds([]byte(`<body><link rel="alternate" type="application/rss+xml" href="/wrong">`), "https://example.test")) != 0 {
		t.Fatal("body links accepted")
	}
	if len(alternateFeeds([]byte(body), "invalid")) != 0 {
		t.Fatal("bad base")
	}
	if len(alternateFeeds([]byte(strings.Repeat(" ", 1<<20)+body), "https://example.test")) != 0 {
		t.Fatal("HTML scan unbounded")
	}
}
func TestResolverErrorsCancellationAndConcurrentSlots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/site":
			fmt.Fprint(w, `<link rel="alternate" type="application/rss+xml" href="/rss">`)
		case "/rss":
			fmt.Fprint(w, `<rss><channel><title>Valid</title></channel></rss>`)
		case "/broken":
			fmt.Fprint(w, `<link rel="alternate" type="application/rss+xml" href="/not-feed">`)
		case "/slow":
			<-r.Context().Done()
		case "/redirect":
			http.Redirect(w, r, "file:///secret", 302)
		default:
			fmt.Fprint(w, "<html/>")
		}
	}))
	defer server.Close()
	r := New(nil, server.Client(), 0)
	for _, address := range []string{"file:///secret", server.URL + "/broken", server.URL + "/not-feed", server.URL + "/redirect"} {
		if _, err := r.Resolve(context.Background(), address); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := r.Resolve(ctx, server.URL+"/slow"); err == nil {
		t.Fatal("cancellation ignored")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := r.Resolve(ctx, fmt.Sprintf("%s/site?i=%d", server.URL, i)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}
