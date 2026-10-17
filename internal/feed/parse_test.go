package feed

import (
	"strings"
	"testing"
	"time"
)

const rssFixture = `<?xml version="1.0"?><rss xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"><channel>
<title>Show &amp; Friends</title><itunes:image href="/art.png"/>
<item><guid>a</guid><title>First</title><description><![CDATA[<p>Hello</p>]]></description>
<pubDate>Tue, 03 Jan 2023 10:00:00 +0000</pubDate><itunes:duration>01:02:03</itunes:duration>
<itunes:image href="episode.png"/><enclosure url="../first.mp3" type="audio/mpeg"/></item>
<item><guid>a</guid><title>Duplicate</title><enclosure url="/duplicate.mp3"/></item>
<item><title>Without GUID</title><enclosure url="/second.ogg" type="application/ogg"/></item>
<item><title>Video only</title><enclosure url="/movie.mp4" type="video/mp4"/></item>
<item><title>No audio</title></item></channel></rss>`

func TestRSSPodcastMetadata(t *testing.T) {
	doc, err := Parse([]byte(rssFixture), "https://example.test/feeds/rss.xml")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Show & Friends" || doc.ArtworkURL != "https://example.test/art.png" || len(doc.Episodes) != 2 {
		t.Fatalf("document: %+v", doc)
	}
	item := doc.Episodes[0]
	if item.GUID != "a" || item.Title != "First" || item.Description != "<p>Hello</p>" || item.DurationSeconds != 3723 || item.AudioURL != "https://example.test/first.mp3" || item.ArtworkURL != "https://example.test/feeds/episode.png" {
		t.Fatalf("episode: %+v", item)
	}
	if item.PublishedAt != time.Date(2023, 1, 3, 10, 0, 0, 0, time.UTC) {
		t.Fatalf("date: %v", item.PublishedAt)
	}
	if doc.Episodes[1].GUID != "" || !doc.Episodes[1].PublishedAt.IsZero() {
		t.Fatal("optional metadata invented")
	}
}

func TestAtomNamespacesAndBaseURLs(t *testing.T) {
	raw := `<feed xmlns="http://www.w3.org/2005/Atom" xml:base="https://cdn.example.test/base/">
<title>Atom Show</title><logo>logo.png</logo>
<entry xml:base="episodes/"><id>urn:episode:1</id><title>Atom episode</title>
<link href="https://example.test/web-page"/>
<link rel="alternate" href="https://example.test/web-page"/>
<link rel="enclosure" type="audio/mp4; codecs=aac" href="one.m4a"/>
<published>2023-01-03T12:00:00+02:00</published><summary type="xhtml"><div>Summary</div></summary></entry>
<entry><title>Web page only</title><link href="https://example.test/not-audio"/></entry></feed>`
	doc, err := Parse([]byte(raw), "https://example.test/feed")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Episodes) != 1 || doc.ArtworkURL != "https://cdn.example.test/base/logo.png" {
		t.Fatalf("atom: %+v", doc)
	}
	episode := doc.Episodes[0]
	if episode.GUID != "urn:episode:1" || episode.AudioURL != "https://cdn.example.test/base/episodes/one.m4a" || episode.Description != "<div>Summary</div>" {
		t.Fatalf("atom episode: %+v", episode)
	}
	if episode.PublishedAt.Hour() != 10 {
		t.Fatal("date not normalized to UTC")
	}
}

func TestRDFAndOptionalFallbacks(t *testing.T) {
	raw := `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><channel xml:base="/podcasts/">
<title>RDF show</title><image><url>show.png</url></image></channel><item>
<title>RDF episode</title><summary>Summary</summary><updated>2023-01-03T10:00:00Z</updated><duration>bad</duration>
<enclosure url="file:///tmp/no"/><enclosure url="http://user:pass@example.test/private"/>
<enclosure url="audio.mp3" type="application/octet-stream"/></item></rdf:RDF>`
	doc, err := Parse([]byte(raw), "https://example.test/feed")
	if err != nil {
		t.Fatal(err)
	}
	if doc.ArtworkURL != "https://example.test/podcasts/show.png" || len(doc.Episodes) != 1 || doc.Episodes[0].Description != "Summary" || doc.Episodes[0].DurationSeconds != 0 {
		t.Fatalf("RDF: %+v", doc)
	}
	if doc.Episodes[0].AudioURL != "https://example.test/audio.mp3" {
		t.Fatal("RDF base URL lost")
	}
	atom, err := Parse([]byte(`<feed><icon>/icon.png</icon><entry><content>Content</content><link rel="enclosure" href="a.mp3"/></entry><entry xml:base="file:///tmp/"><link rel="enclosure" href="bad"/></entry></feed>`), "https://example.test/feed")
	if err != nil || atom.ArtworkURL != "https://example.test/icon.png" || len(atom.Episodes) != 1 || atom.Episodes[0].Description != "Content" {
		t.Fatalf("fallbacks: %+v, %v", atom, err)
	}
}

func TestInvalidFeeds(t *testing.T) {
	for _, raw := range []string{`<html/>`, `<rss/>`, `<rss>`, `<feed xml:base="file:///tmp/"/>`, `<rss><channel xml:base="file:///tmp/"/></rss>`, `<?xml version="1.0" encoding="ISO-8859-1"?><rss/>`, strings.Repeat("x", MaxBytes+1), `<feed>` + strings.Repeat(`<entry/>`, maxEpisodes+1) + `</feed>`} {
		if _, err := Parse([]byte(raw), "https://example.test/feed"); err == nil {
			t.Fatalf("invalid feed accepted (length %d)", len(raw))
		}
	}
	if _, err := Parse([]byte(`<feed/>`), "file:///tmp/feed"); err == nil {
		t.Fatal("invalid source accepted")
	}
}

func TestOptionalDateAndDuration(t *testing.T) {
	for raw, want := range map[string]int64{"90": 90, "1:30": 90, "01:02:03": 3723, "": 0, "-1": 0, "bad": 0, "1:2:3:4": 0, "999999999999999999999": 0, "1000001": 0, "999999:59:59": 0} {
		if got := duration(raw); got != want {
			t.Errorf("duration(%q)=%d, want %d", raw, got, want)
		}
	}
	for _, raw := range []string{"Tue, 3 Jan 2023 10:00:00 +0000", "03 Jan 23 10:00 +0000", "03 Jan 23 10:00 GMT", "Tuesday, 03-Jan-23 10:00:00 GMT"} {
		if date(raw).IsZero() {
			t.Errorf("date rejected %q", raw)
		}
	}
	if !date("invalid").IsZero() {
		t.Fatal("invalid date invented")
	}
}
