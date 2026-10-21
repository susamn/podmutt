package feed

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"mime"
	"strings"

	"podmutt/internal/activity"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

type Resolved struct {
	URL     string
	Fetched Fetched
}

// Resolve reads a public feed or a website's explicit RSS/Atom alternate links.
// It never persists, follows website links only one level, tries at most three,
// and shares Fetch's body/redirect/slot bounds and one total 15-second deadline.
func (r *Refresher) Resolve(ctx context.Context, raw string) (Resolved, error) {
	ctx, finish := activity.Begin(ctx, "Loading feed preview…", activity.Foreground)
	defer finish()
	address, err := httpurl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Resolved{}, err
	}
	address.Fragment = ""
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	result := Resolved{URL: address.String()}
	var links []string
	err = r.withFeed(ctx, store.Feed{URL: result.URL}, func(ctx context.Context) error {
		var err error
		result.Fetched, err = r.fetchWith(ctx, store.Feed{URL: result.URL}, func(data []byte, finalURL string) (Document, error) {
			document, err := Parse(data, finalURL)
			if err == nil {
				return document, nil
			}
			links = alternateFeeds(data, finalURL)
			if len(links) == 0 {
				return Document{}, fmt.Errorf("URL is not a supported feed or website with RSS/Atom links")
			}
			return Document{}, nil
		})
		return err
	})
	if err != nil {
		return Resolved{}, err
	}
	if len(links) == 0 {
		return result, nil
	}
	// Release the website's slot before fetching candidates, avoiding nested-slot
	// deadlocks when several previews resolve websites concurrently.
	for _, link := range links {
		fetched, err := r.Fetch(ctx, store.Feed{URL: link})
		if err == nil && !fetched.NotModified {
			return Resolved{URL: link, Fetched: fetched}, nil
		}
		if ctx.Err() != nil {
			return Resolved{}, ctx.Err()
		}
	}
	return Resolved{}, fmt.Errorf("website RSS/Atom links could not be loaded")
}
func alternateFeeds(data []byte, base string) []string {
	baseURL, err := httpurl.Parse(base)
	if err != nil {
		return nil
	}
	// HTML discovery is deliberately best-effort, not a new browser/parser stack.
	decoder := xml.NewDecoder(bytes.NewReader(data[:min(len(data), 1<<20)]))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	var links []string
	seen := make(map[string]bool)
	baseSeen := false
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		name := strings.ToLower(start.Name.Local)
		if name == "body" {
			break
		}
		if name == "script" || name == "style" {
			if decoder.Skip() != nil {
				break
			}
			continue
		}
		attributes := make(map[string]string)
		for _, attr := range start.Attr {
			attributes[strings.ToLower(attr.Name.Local)] = attr.Value
		}
		if name == "base" && !baseSeen {
			baseSeen = true
			if u, err := httpurl.Resolve(baseURL, attributes["href"]); err == nil {
				baseURL, _ = httpurl.Parse(u)
			}
			continue
		}
		if name != "link" {
			continue
		}
		alternate := false
		for _, rel := range strings.Fields(strings.ToLower(attributes["rel"])) {
			alternate = alternate || rel == "alternate"
		}
		kind, _, _ := mime.ParseMediaType(strings.ToLower(attributes["type"]))
		if !alternate || (kind != "application/rss+xml" && kind != "application/atom+xml" && kind != "application/rdf+xml") {
			continue
		}
		address, err := httpurl.Resolve(baseURL, attributes["href"])
		if err != nil || attributes["href"] == "" || seen[address] {
			continue
		}
		seen[address] = true
		links = append(links, address)
		if len(links) == 3 {
			break
		}
	}
	return links
}
