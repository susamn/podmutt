package opml

import (
	"strings"
	"testing"
)

func TestParseNestedAndDeduplicated(t *testing.T) {
	feeds, err := Parse(strings.NewReader(`<?xml version="1.0"?>
<opml version="2.0"><head><title>x</title></head><body>
  <outline text="Folder"><outline type="rss" text="A &amp; B" xmlUrl="https://a.test/feed"/>
    <outline type="rss" title="Cee" text="c" xmlUrl=" https://c.test/rss "/></outline>
  <outline type="rss" text="dup" xmlUrl="https://a.test/feed"/>
  <outline text="no feed here"/>
</body></opml>`))
	if err != nil {
		t.Fatal(err)
	}
	want := []Feed{{"A & B", "https://a.test/feed"}, {"Cee", "https://c.test/rss"}}
	if len(feeds) != len(want) {
		t.Fatalf("got %+v", feeds)
	}
	for i := range want {
		if feeds[i] != want[i] {
			t.Fatalf("feed %d: got %+v want %+v", i, feeds[i], want[i])
		}
	}
}

func TestParseRejectsNonOPML(t *testing.T) {
	if _, err := Parse(strings.NewReader("not xml")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Parse(strings.NewReader("<rss/>")); err == nil {
		t.Fatal("expected error for non-opml root")
	}
}
