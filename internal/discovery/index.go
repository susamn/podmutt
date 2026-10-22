package discovery

import (
	"context"
	"crypto/sha1" // Podcast Index's documented authentication protocol, not password hashing.
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"podmutt/internal/activity"
	"podmutt/internal/feedback"
)

// Searcher is the small shared provider contract. Charts remain Apple-specific.
type Searcher interface {
	Search(context.Context, string) ([]Show, error)
}

const IndexKeyEnv = "PODMUTT_PODCAST_INDEX_KEY"
const IndexSecretEnv = "PODMUTT_PODCAST_INDEX_SECRET"

var ErrIndexCredentials = feedback.New(feedback.Credentials, "Podcast Index requires your own PODMUTT_PODCAST_INDEX_KEY and PODMUTT_PODCAST_INDEX_SECRET; Apple and RSS need no key")

// Index keeps no API response cache or durable directory records. The only
// persistent content comes from separately fetching a user-selected public RSS.
type Index struct {
	transport             *Apple
	key, secret, endpoint string
	now                   func() time.Time
}

func NewIndex(key, secret string, client *http.Client) *Index {
	transport := NewApple(client)
	// Never forward signed credentials, even to a same-host redirect target.
	transport.client.CheckRedirect = func(*http.Request, []*http.Request) error { return ErrUnavailable }
	return &Index{transport: transport, key: key, secret: secret, endpoint: "https://api.podcastindex.org/api/1.0/search/byterm", now: time.Now}
}
func (i *Index) String() string   { return "Podcast Index client (credentials redacted)" }
func (i *Index) GoString() string { return i.String() }
func (i *Index) Search(ctx context.Context, query string) ([]Show, error) {
	ctx, finish := activity.Begin(ctx, "Searching Podcast Index…", activity.Foreground)
	defer finish()
	query = strings.Join(strings.Fields(query), " ")
	if size := len([]rune(query)); size == 0 || size > 128 {
		return nil, ErrQuery
	}
	if i.key == "" || i.secret == "" {
		return nil, ErrIndexCredentials
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	select {
	case i.transport.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-i.transport.gate }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(i.endpoint)
	if err != nil {
		return nil, ErrUnavailable
	}
	params := endpoint.Query()
	params.Set("q", query)
	params.Set("max", strconv.Itoa(maxResults))
	endpoint.RawQuery = params.Encode()
	stamp := strconv.FormatInt(i.now().Unix(), 10)
	sum := sha1.Sum([]byte(i.key + i.secret + stamp))
	headers := http.Header{"X-Auth-Key": {i.key}, "X-Auth-Date": {stamp}, "Authorization": {hex.EncodeToString(sum[:])}, "User-Agent": {"podmutt/dev (+https://gitlab.com/susamn/podmutt)"}}
	var response struct {
		Status json.RawMessage
		Feeds  []struct {
			ID         int64
			Title, URL string
		}
	}
	if err := i.transport.readHeaders(ctx, endpoint.String(), &response, headers); err != nil {
		return nil, err
	}
	if string(response.Status) != "true" && string(response.Status) != `"true"` {
		return nil, feedback.New(feedback.Authentication, "Podcast Index rejected search; check your API access and system clock")
	}
	records := make([]appleRecord, 0, min(len(response.Feeds), maxResults))
	for _, record := range response.Feeds {
		source := "https://podcastindex.org"
		if record.ID > 0 {
			source += "/podcast/" + strconv.FormatInt(record.ID, 10)
		}
		records = append(records, appleRecord{CollectionName: record.Title, FeedURL: record.URL, CollectionViewURL: source})
	}
	return normalize(records, maxResults), nil
}
