package httpurl

import "testing"

func TestParseAndResolve(t *testing.T) {
	base, err := Parse("https://example.test/feeds/show.xml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(base, "../audio.mp3")
	if err != nil || got != "https://example.test/audio.mp3" {
		t.Fatalf("resolve: %q, %v", got, err)
	}
	for _, raw := range []string{"", "file:///tmp/feed", "https://", "http://[", "https://user:secret@example.test/feed"} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"", "http://[", "file:///tmp/a"} {
		if _, err := Resolve(base, raw); err == nil {
			t.Errorf("resolved %q", raw)
		}
	}
}
