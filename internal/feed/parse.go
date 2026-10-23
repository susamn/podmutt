// Package feed normalizes public podcast feeds and refreshes cached metadata.
package feed

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"podmutt/internal/chapters"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

type chapterTag struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}
type podloveTag struct {
	XMLName xml.Name
	Items   []struct {
		Start string `xml:"start,attr"`
		Title string `xml:"title,attr"`
		HRef  string `xml:"href,attr"`
		Image string `xml:"image,attr"`
	} `xml:"chapter"`
}

const MaxBytes = 8 << 20
const maxEpisodes = 10000

type Document struct {
	Title, ArtworkURL string
	Episodes          []store.Episode
}

type text struct {
	Value string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

func (t text) value() string {
	if strings.TrimSpace(t.Value) != "" {
		return strings.TrimSpace(t.Value)
	}
	return strings.TrimSpace(t.Inner)
}

type image struct {
	URL  string `xml:"url"`
	Href string `xml:"href,attr"`
}
type link struct {
	URL  string `xml:"url,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
	Rel  string `xml:"rel,attr"`
}
type entry struct {
	Base        string     `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	GUID        string     `xml:"guid"`
	ID          string     `xml:"id"`
	Title       text       `xml:"title"`
	Description text       `xml:"description"`
	Summary     text       `xml:"summary"`
	Content     text       `xml:"content"`
	Published   string     `xml:"published"`
	Updated     string     `xml:"updated"`
	PubDate     string     `xml:"pubDate"`
	Date        string     `xml:"date"`
	Duration    string     `xml:"duration"`
	Images      []image    `xml:"image"`
	Enclosures  []link     `xml:"enclosure"`
	Links       []link     `xml:"link"`
	ChaptersTag chapterTag `xml:"http://podcastindex.org/namespace/1.0 chapters"`
	PodloveXML  podloveTag `xml:"http://podlove.org/simple-chapters chapters"`
}
type channel struct {
	Base   string  `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	Title  text    `xml:"title"`
	Images []image `xml:"image"`
	Items  []entry `xml:"item"`
}
type wireDocument struct {
	XMLName xml.Name
	Base    string   `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	Title   text     `xml:"title"`
	Logo    string   `xml:"logo"`
	Icon    string   `xml:"icon"`
	Channel *channel `xml:"channel"`
	Items   []entry  `xml:"item"`
	Entries []entry  `xml:"entry"`
}

// Parse supports UTF-8/ASCII RSS 2, RSS 1/RDF, and Atom, including common iTunes
// artwork/duration fields. Missing/bad optional metadata does not drop audio.
func Parse(data []byte, sourceURL string) (Document, error) {
	if len(data) > MaxBytes {
		return Document{}, fmt.Errorf("feed exceeds %d bytes", MaxBytes)
	}
	base, err := httpurl.Parse(sourceURL)
	if err != nil {
		return Document{}, err
	}
	var wire wireDocument
	if err := xml.Unmarshal(data, &wire); err != nil {
		return Document{}, fmt.Errorf("invalid feed XML: %w", err)
	}
	base, err = xmlBase(base, wire.Base)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	var entries []entry
	switch strings.ToLower(wire.XMLName.Local) {
	case "rss", "rdf":
		if wire.Channel == nil {
			return Document{}, fmt.Errorf("RSS channel missing")
		}
		documentBase := base
		base, err = xmlBase(base, wire.Channel.Base)
		if err != nil {
			return Document{}, err
		}
		doc.Title = wire.Channel.Title.value()
		doc.ArtworkURL = artwork(base, wire.Channel.Images)
		entries = wire.Channel.Items
		for _, item := range wire.Items {
			rootBase, baseErr := xmlBase(documentBase, item.Base)
			if baseErr != nil {
				continue
			}
			item.Base = rootBase.String()
			entries = append(entries, item)
		}
	case "feed":
		doc.Title = wire.Title.value()
		doc.ArtworkURL, _ = httpurl.Resolve(base, first(wire.Logo, wire.Icon))
		entries = wire.Entries
	default:
		return Document{}, fmt.Errorf("unsupported feed format")
	}
	if len(entries) > maxEpisodes {
		return Document{}, fmt.Errorf("feed exceeds %d entries", maxEpisodes)
	}
	seen := make(map[string]bool)
	for _, item := range entries {
		itemBase, err := xmlBase(base, item.Base)
		if err != nil {
			continue
		}
		var audio string
		candidates := append([]link(nil), item.Enclosures...)
		for _, candidate := range item.Links {
			if candidate.Rel == "enclosure" {
				candidates = append(candidates, candidate)
			}
		}
		for _, candidate := range candidates {
			kind := strings.ToLower(strings.TrimSpace(strings.SplitN(candidate.Type, ";", 2)[0]))
			if kind != "" && !strings.HasPrefix(kind, "audio/") && kind != "application/ogg" && kind != "application/octet-stream" {
				continue
			}
			if resolved, err := httpurl.Resolve(itemBase, first(candidate.URL, candidate.Href)); err == nil {
				audio = resolved
				break
			}
		}
		if audio == "" {
			continue
		}
		guid := first(item.GUID, item.ID)
		identity := "url:" + audio
		if guid != "" {
			identity = "guid:" + guid
		}
		if seen[identity] {
			continue
		}
		seen[identity] = true

		chaptersURL := ""
		rawChapURL := item.ChaptersTag.URL
		if rawChapURL != "" {
			if resolved, err := httpurl.Resolve(itemBase, rawChapURL); err == nil {
				chaptersURL = resolved
			}
		}

		chaptersJSON := ""
		podlove := item.PodloveXML
		if len(podlove.Items) > 0 {
			if raw, err := xml.Marshal(podlove); err == nil {
				if parsed, err := chapters.ParsePodloveXML(raw); err == nil && len(parsed) > 0 {
					chaptersJSON, _ = chapters.Serialize(parsed)
				}
			}
		}

		doc.Episodes = append(doc.Episodes, store.Episode{
			GUID: guid, Title: item.Title.value(), Description: first(item.Description.value(), item.Summary.value(), item.Content.value()),
			AudioURL: audio, ArtworkURL: artwork(itemBase, item.Images),
			ChaptersURL: chaptersURL, ChaptersJSON: chaptersJSON,
			PublishedAt: date(first(item.PubDate, item.Published, item.Date, item.Updated)), DurationSeconds: duration(item.Duration),
		})
	}
	return doc, nil
}

func first(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
func xmlBase(base *url.URL, reference string) (*url.URL, error) {
	if reference == "" {
		return base, nil
	}
	resolved, err := httpurl.Resolve(base, reference)
	if err != nil {
		return nil, err
	}
	return httpurl.Parse(resolved)
}
func artwork(base *url.URL, images []image) string {
	for _, image := range images {
		if resolved, err := httpurl.Resolve(base, first(image.Href, image.URL)); err == nil {
			return resolved
		}
	}
	return ""
}
func date(raw string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", time.RFC822Z, time.RFC822, time.RFC850} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
func duration(raw string) int64 {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) > 3 {
		return 0
	}
	var seconds int64
	for _, part := range parts {
		value, err := strconv.ParseInt(part, 10, 64)
		if err != nil || value < 0 || value > 1_000_000 || seconds > 1_000_000 {
			return 0
		}
		seconds = seconds*60 + value
	}
	if seconds > 1_000_000 {
		return 0
	}
	return seconds
}
