package chapters

import (
	"math"
	"reflect"
	"testing"
)

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		input string
		want  float64
		err   bool
	}{
		{"0", 0, false},
		{"00:00:00", 0, false},
		{"00:00", 0, false},
		{"120.5", 120.5, false},
		{"02:00", 120, false},
		{"02:00.5", 120.5, false},
		{"01:02:03", 3723, false},
		{"01:02:03.500", 3723.5, false},
		{"  00:15:30  ", 930, false},
		{"", 0, true},
		{"-10", 0, true},
		{"abc", 0, true},
		{"01:60", 0, true},
		{"01:02:60", 0, true},
		{"01:02:03:04", 0, true},
		{"-01:30", 0, true},
		{"01:-30", 0, true},
		{"-01:02:03", 0, true},
		{"01:-02:03", 0, true},
		{"01:02:-03", 0, true},
	}

	for _, tc := range cases {
		got, err := ParseTimestamp(tc.input)
		if (err != nil) != tc.err {
			t.Fatalf("ParseTimestamp(%q) err=%v, wantErr=%v", tc.input, err, tc.err)
		}
		if !tc.err && math.Abs(got-tc.want) > 0.001 {
			t.Fatalf("ParseTimestamp(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseJSONPodcasting20(t *testing.T) {
	sample := []byte(`{
		"version": "1.2.0",
		"chapters": [
			{"startTime": 0, "title": "Intro"},
			{"startTime": "00:05:00", "title": "Main Interview", "url": "https://example.com"},
			{"startTime": 600, "toc": false},
			{"startTime": 300, "title": ""},
			{"startTime": -10, "title": "Bad negative"},
			{"startTime": true, "title": "Bad type"}
		]
	}`)

	list, err := ParseJSON(sample)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Start: 0 ("Intro")
	// 2. Start: 300 (Title defaulted to "Chapter 2")
	// 3. Start: 300 was duplicate? Deduplicated.
	// 4. 600 was toc: false, so skipped.
	// 5. Negative and bad type were skipped.
	if len(list) != 2 {
		t.Fatalf("expected 2 chapters, got %d: %+v", len(list), list)
	}
	if list[0].Title != "Intro" || list[0].Start != 0 || list[0].Duration != 300 {
		t.Fatalf("unexpected chapter 1: %+v", list[0])
	}
	if list[1].Title != "Main Interview" || list[1].Start != 300 || list[1].URL != "https://example.com" {
		t.Fatalf("unexpected chapter 2: %+v", list[1])
	}

	// Fallback format: root array
	sliceSample := []byte(`[{"startTime": 0, "title": "Chapter A"}, {"startTime": 100, "title": "Chapter B"}]`)
	sliceList, err := ParseJSON(sliceSample)
	if err != nil || len(sliceList) != 2 || sliceList[1].Start != 100 {
		t.Fatalf("slice JSON failed: %v, %+v", err, sliceList)
	}

	// Empty and invalid
	if l, err := ParseJSON(nil); err != nil || l != nil {
		t.Fatal(l, err)
	}
	if _, err := ParseJSON([]byte(`invalid json`)); err == nil {
		t.Fatal("expected error on invalid JSON")
	}

	// parseJSONTime empty check
	if _, err := parseJSONTime(nil); err == nil {
		t.Fatal("expected error on empty raw json time")
	}
}

func TestParsePodloveXML(t *testing.T) {
	xmlData := []byte(`
	<psc:chapters xmlns:psc="http://podlove.org/simple-chapters" version="1.2">
		<psc:chapter start="00:00:00.000" title="Welcome" />
		<psc:chapter start="00:02:30.000" title="Segment 1" href="https://podlove.org" image="https://podlove.org/img.png"/>
		<psc:chapter start="00:10:00" title="" />
		<psc:chapter start="invalid-time" title="Skipped" />
	</psc:chapters>`)

	list, err := ParsePodloveXML(xmlData)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(list))
	}
	if list[0].Title != "Welcome" || list[0].Duration != 150 {
		t.Fatalf("chapter 0 mismatch: %+v", list[0])
	}
	if list[1].Title != "Segment 1" || list[1].URL != "https://podlove.org" || list[1].Image != "https://podlove.org/img.png" {
		t.Fatalf("chapter 1 mismatch: %+v", list[1])
	}
	if list[2].Title != "Chapter 3" || list[2].Start != 600 {
		t.Fatalf("chapter 2 fallback title mismatch: %+v", list[2])
	}

	// Empty and invalid
	if l, err := ParsePodloveXML(nil); err != nil || l != nil {
		t.Fatal(l, err)
	}
	if _, err := ParsePodloveXML([]byte(`<bad xml`)); err == nil {
		t.Fatal("expected error on malformed XML")
	}
}

func TestFromMPV(t *testing.T) {
	mpvData := []byte(`[
		{"title": "Intro", "time": 0.0},
		{"title": "Discussion", "time": 180.5},
		{"title": "Skipped Negative", "time": -5.0},
		{"title": "Outro", "time": 600.0}
	]`)

	list, err := FromMPV(mpvData)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 chapters from mpv, got %d", len(list))
	}
	if list[1].Title != "Discussion" || list[1].Start != 180.5 || list[1].Duration != 419.5 {
		t.Fatalf("mpv chapter mismatch: %+v", list[1])
	}

	// Null and empty
	if l, err := FromMPV([]byte("null")); err != nil || l != nil {
		t.Fatal(l, err)
	}
	if l, err := FromMPV(nil); err != nil || l != nil {
		t.Fatal(l, err)
	}
	if _, err := FromMPV([]byte("not json")); err == nil {
		t.Fatal("expected error on invalid mpv json")
	}
}

func TestCurrentChapterLookup(t *testing.T) {
	chapters := []Chapter{
		{Index: 1, Start: 0, Duration: 60, Title: "Ch 1"},
		{Index: 2, Start: 60, Duration: 120, Title: "Ch 2"},
		{Index: 3, Start: 180, Duration: 60, Title: "Ch 3"},
	}

	// Empty list
	if _, _, ok := Current(nil, 10); ok {
		t.Fatal("expected false for empty chapters")
	}

	// Before first chapter
	c, idx, ok := Current(chapters, -5)
	if !ok || idx != 0 || c.Title != "Ch 1" {
		t.Fatalf("before start got idx=%d, ch=%+v", idx, c)
	}

	// In chapter 1
	c, idx, ok = Current(chapters, 30)
	if !ok || idx != 0 || c.Title != "Ch 1" {
		t.Fatalf("ch 1 got idx=%d, ch=%+v", idx, c)
	}

	// Exact boundary chapter 2
	c, idx, ok = Current(chapters, 60)
	if !ok || idx != 1 || c.Title != "Ch 2" {
		t.Fatalf("ch 2 boundary got idx=%d, ch=%+v", idx, c)
	}

	// In chapter 2
	c, idx, ok = Current(chapters, 120)
	if !ok || idx != 1 || c.Title != "Ch 2" {
		t.Fatalf("ch 2 got idx=%d, ch=%+v", idx, c)
	}

	// Past last chapter
	c, idx, ok = Current(chapters, 500)
	if !ok || idx != 2 || c.Title != "Ch 3" {
		t.Fatalf("past end got idx=%d, ch=%+v", idx, c)
	}
}

func TestNormalizeEmpty(t *testing.T) {
	if got := normalize(nil); got != nil {
		t.Fatal(got)
	}
}

func TestSerializeAndDeserializeRoundTrip(t *testing.T) {
	original := []Chapter{
		{Index: 1, Start: 0, Duration: 100, Title: "Intro"},
		{Index: 2, Start: 100, Duration: 200, Title: "Topic", URL: "https://example.com", Image: "https://example.com/art.png"},
	}

	serialized, err := Serialize(original)
	if err != nil {
		t.Fatal(err)
	}
	if serialized == "" {
		t.Fatal("empty serialized string")
	}

	deserialized, err := Deserialize(serialized)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, deserialized) {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", deserialized, original)
	}

	// Empty cases
	sEmpty, err := Serialize(nil)
	if err != nil || sEmpty != "" {
		t.Fatal(sEmpty, err)
	}
	dEmpty, err := Deserialize("")
	if err != nil || dEmpty != nil {
		t.Fatal(dEmpty, err)
	}
	if _, err := Deserialize("invalid json"); err == nil {
		t.Fatal("expected error on invalid json")
	}
}
