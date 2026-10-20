// Package opml reads podcast subscription lists in the OPML format exported by
// most podcast clients.
package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Feed is one subscription found in an OPML document.
type Feed struct {
	Title string
	URL   string
}

type outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	Children []outline `xml:"outline"`
}

type document struct {
	XMLName xml.Name  `xml:"opml"`
	Body    []outline `xml:"body>outline"`
}

// Parse returns every outline carrying an xmlUrl, in document order. Outlines
// may be nested in folders. Duplicate URLs are reported once.
func Parse(r io.Reader) ([]Feed, error) {
	var doc document
	decoder := xml.NewDecoder(r)
	decoder.Strict = false
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("opml: %w", err)
	}
	var feeds []Feed
	seen := map[string]bool{}
	var walk func([]outline)
	walk = func(items []outline) {
		for _, item := range items {
			if url := strings.TrimSpace(item.XMLURL); url != "" && !seen[url] {
				seen[url] = true
				title := strings.TrimSpace(item.Title)
				if title == "" {
					title = strings.TrimSpace(item.Text)
				}
				feeds = append(feeds, Feed{Title: title, URL: url})
			}
			walk(item.Children)
		}
	}
	walk(doc.Body)
	return feeds, nil
}
