// Package chapters parses, normalizes and queries episode chapter markers from
// Podcasting 2.0 JSON, Podlove Simple Chapters XML, and mpv container metadata.
package chapters

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Chapter struct {
	Index    int     `json:"index"`
	Start    float64 `json:"start"`              // seconds from start of episode
	Duration float64 `json:"duration,omitempty"` // seconds, optional or computed
	Title    string  `json:"title"`
	URL      string  `json:"url,omitempty"`
	Image    string  `json:"image,omitempty"`
}

// ParseTimestamp converts "HH:MM:SS[.mmm]", "MM:SS[.mmm]", or seconds "123.45"
// to floating-point seconds.
func ParseTimestamp(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("empty timestamp")
	}

	parts := strings.Split(s, ":")
	switch len(parts) {
	case 1:
		// Raw seconds e.g. "120" or "120.5"
		sec, err := strconv.ParseFloat(parts[0], 64)
		if err != nil || sec < 0 || math.IsNaN(sec) || math.IsInf(sec, 0) {
			return 0, fmt.Errorf("invalid seconds timestamp %q", raw)
		}
		return sec, nil
	case 2:
		// "MM:SS[.mmm]"
		min, err := strconv.ParseFloat(parts[0], 64)
		if err != nil || min < 0 || math.IsNaN(min) || math.IsInf(min, 0) {
			return 0, fmt.Errorf("invalid minutes in timestamp %q", raw)
		}
		sec, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || sec < 0 || sec >= 60 || math.IsNaN(sec) || math.IsInf(sec, 0) {
			return 0, fmt.Errorf("invalid seconds in timestamp %q", raw)
		}
		return min*60 + sec, nil
	case 3:
		// "HH:MM:SS[.mmm]"
		hr, err := strconv.ParseFloat(parts[0], 64)
		if err != nil || hr < 0 || math.IsNaN(hr) || math.IsInf(hr, 0) {
			return 0, fmt.Errorf("invalid hours in timestamp %q", raw)
		}
		min, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || min < 0 || min >= 60 || math.IsNaN(min) || math.IsInf(min, 0) {
			return 0, fmt.Errorf("invalid minutes in timestamp %q", raw)
		}
		sec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil || sec < 0 || sec >= 60 || math.IsNaN(sec) || math.IsInf(sec, 0) {
			return 0, fmt.Errorf("invalid seconds in timestamp %q", raw)
		}
		return hr*3600 + min*60 + sec, nil
	default:
		return 0, fmt.Errorf("malformed timestamp format %q", raw)
	}
}

// ParseJSON parses Podcasting 2.0 JSON chapters specification format.
func ParseJSON(data []byte) ([]Chapter, error) {
	if len(data) == 0 {
		return nil, nil
	}
	// Try standard container format: {"version": "...", "chapters": [...]}
	var container struct {
		Chapters []struct {
			StartTime json.RawMessage `json:"startTime"`
			Title     string          `json:"title"`
			Img       string          `json:"img"`
			URL       string          `json:"url"`
			Toc       *bool           `json:"toc"`
		} `json:"chapters"`
	}

	if err := json.Unmarshal(data, &container); err != nil {
		// Fallback: check if root is directly a slice of chapters
		var rawList []struct {
			StartTime json.RawMessage `json:"startTime"`
			Title     string          `json:"title"`
			Img       string          `json:"img"`
			URL       string          `json:"url"`
			Toc       *bool           `json:"toc"`
		}
		if errList := json.Unmarshal(data, &rawList); errList != nil {
			return nil, fmt.Errorf("invalid chapters JSON: %w", err)
		}
		container.Chapters = rawList
	}

	var list []Chapter
	for _, raw := range container.Chapters {
		if raw.Toc != nil && !*raw.Toc {
			continue // Podcasting 2.0 spec allows excluding chapter from TOC
		}
		start, err := parseJSONTime(raw.StartTime)
		if err != nil {
			continue
		}
		list = append(list, Chapter{
			Start: start,
			Title: strings.TrimSpace(raw.Title),
			URL:   strings.TrimSpace(raw.URL),
			Image: strings.TrimSpace(raw.Img),
		})
	}

	return normalize(list), nil
}

func parseJSONTime(raw json.RawMessage) (float64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("missing start time")
	}
	// Number format e.g. 120 or 120.5
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		if num < 0 || math.IsNaN(num) || math.IsInf(num, 0) {
			return 0, fmt.Errorf("invalid numeric start time")
		}
		return num, nil
	}
	// String format e.g. "00:02:00.500" or "120.5"
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return ParseTimestamp(str)
	}
	return 0, fmt.Errorf("unrecognized time format")
}

// ParsePodloveXML parses inline Podlove Simple Chapters XML (<psc:chapters> or <chapters>).
func ParsePodloveXML(data []byte) ([]Chapter, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var doc struct {
		XMLName  xml.Name
		Chapters []struct {
			Start string `xml:"start,attr"`
			Title string `xml:"title,attr"`
			HRef  string `xml:"href,attr"`
			Image string `xml:"image,attr"`
		} `xml:"chapter"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid Podlove XML: %w", err)
	}

	var list []Chapter
	for _, c := range doc.Chapters {
		start, err := ParseTimestamp(c.Start)
		if err != nil {
			continue
		}
		list = append(list, Chapter{
			Start: start,
			Title: strings.TrimSpace(c.Title),
			URL:   strings.TrimSpace(c.HRef),
			Image: strings.TrimSpace(c.Image),
		})
	}
	return normalize(list), nil
}

// FromMPV normalizes chapters extracted from mpv's "chapter-list" property.
func FromMPV(data []byte) ([]Chapter, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var rawList []struct {
		Title string  `json:"title"`
		Time  float64 `json:"time"`
	}
	if err := json.Unmarshal(data, &rawList); err != nil {
		return nil, fmt.Errorf("invalid mpv chapter-list JSON: %w", err)
	}

	var list []Chapter
	for _, r := range rawList {
		if r.Time < 0 || math.IsNaN(r.Time) || math.IsInf(r.Time, 0) {
			continue
		}
		list = append(list, Chapter{
			Start: r.Time,
			Title: strings.TrimSpace(r.Title),
		})
	}
	return normalize(list), nil
}

// normalize sorts chapters by start time, deduplicates timestamps, populates
// fallback titles, assigns sequential indices, and computes durations between chapters.
func normalize(list []Chapter) []Chapter {
	if len(list) == 0 {
		return nil
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Start < list[j].Start
	})

	// Deduplicate identical start times (retain first occurrence)
	var dedup []Chapter
	for _, c := range list {
		if len(dedup) > 0 && math.Abs(dedup[len(dedup)-1].Start-c.Start) < 0.001 {
			continue
		}
		dedup = append(dedup, c)
	}

	for i := range dedup {
		dedup[i].Index = i + 1
		if dedup[i].Title == "" {
			dedup[i].Title = fmt.Sprintf("Chapter %d", i+1)
		}
		if i < len(dedup)-1 {
			dedup[i].Duration = dedup[i+1].Start - dedup[i].Start
		}
	}
	return dedup
}

// Current returns the chapter covering the given playback position in seconds,
// its 0-based slice index, and true if chapters exist.
func Current(chapters []Chapter, position float64) (Chapter, int, bool) {
	if len(chapters) == 0 {
		return Chapter{}, 0, false
	}
	if position <= chapters[0].Start {
		return chapters[0], 0, true
	}
	for i := len(chapters) - 1; i >= 0; i-- {
		if position >= chapters[i].Start {
			return chapters[i], i, true
		}
	}
	return chapters[0], 0, true
}

// Serialize encodes normalized chapters into a compact JSON string for database caching.
func Serialize(chapters []Chapter) (string, error) {
	if len(chapters) == 0 {
		return "", nil
	}
	b, err := json.Marshal(chapters)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Deserialize decodes cached JSON chapters into normalized slice.
func Deserialize(jsonStr string) ([]Chapter, error) {
	s := strings.TrimSpace(jsonStr)
	if s == "" {
		return nil, nil
	}
	var list []Chapter
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return nil, err
	}
	return list, nil
}
