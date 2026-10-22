package feedback

import "testing"

func TestSourceAttributionOmitsCredentialsAndTokens(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"https://user:private-secret@example.test/rss?token=private-secret#private-secret", "https://example.test/rss"},
		{"https://podcastindex.org/podcast/123", "https://podcastindex.org/podcast/123"},
		{"https://example.test/rss?", "https://example.test/rss"},
		{"not-a-url", "Source URL unavailable"},
		{"https://%", "Source URL unavailable"},
	} {
		if got := SourceURL(test.input); got != test.want {
			t.Fatal(got, test.want)
		}
	}
}
