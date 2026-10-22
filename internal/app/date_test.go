package app

import (
	"testing"
	"time"
)

func TestFormatPublishedDateRelativeBoundaries(t *testing.T) {
	loc := time.FixedZone("EST", -5*3600)
	now := time.Date(2026, 10, 22, 14, 30, 0, 0, loc)

	cases := []struct {
		name string
		pub  time.Time
		want string
	}{
		{
			name: "missing zero timestamp",
			pub:  time.Time{},
			want: "",
		},
		{
			name: "unix zero timestamp",
			pub:  time.Unix(0, 0),
			want: "",
		},
		{
			name: "negative unix timestamp",
			pub:  time.Unix(-3600, 0),
			want: "",
		},
		{
			name: "today earlier same day",
			pub:  time.Date(2026, 10, 22, 8, 0, 0, 0, loc),
			want: "Today",
		},
		{
			name: "today midnight",
			pub:  time.Date(2026, 10, 22, 0, 1, 0, 0, loc),
			want: "Today",
		},
		{
			name: "yesterday late evening (short elapsed hours, 1 calendar day)",
			pub:  time.Date(2026, 10, 21, 23, 59, 0, 0, loc),
			want: "Yesterday",
		},
		{
			name: "yesterday early morning",
			pub:  time.Date(2026, 10, 21, 0, 1, 0, 0, loc),
			want: "Yesterday",
		},
		{
			name: "2 days back",
			pub:  time.Date(2026, 10, 20, 12, 0, 0, 0, loc),
			want: "2 days back",
		},
		{
			name: "3 days back",
			pub:  time.Date(2026, 10, 19, 12, 0, 0, 0, loc),
			want: "3 days back",
		},
		{
			name: "4 days back",
			pub:  time.Date(2026, 10, 18, 12, 0, 0, 0, loc),
			want: "4 days back",
		},
		{
			name: "5 days back",
			pub:  time.Date(2026, 10, 17, 12, 0, 0, 0, loc),
			want: "5 days back",
		},
		{
			name: "6 days back",
			pub:  time.Date(2026, 10, 16, 12, 0, 0, 0, loc),
			want: "6 days back",
		},
		{
			name: "exactly 7 days back",
			pub:  time.Date(2026, 10, 15, 12, 0, 0, 0, loc),
			want: "A week back",
		},
		{
			name: "8 days back (falls back to YYYY-MM-DD)",
			pub:  time.Date(2026, 10, 14, 12, 0, 0, 0, loc),
			want: "2026-10-14",
		},
		{
			name: "30 days back",
			pub:  time.Date(2026, 9, 22, 12, 0, 0, 0, loc),
			want: "2026-09-22",
		},
		{
			name: "one year back",
			pub:  time.Date(2025, 10, 22, 12, 0, 0, 0, loc),
			want: "2025-10-22",
		},
		{
			name: "future date tomorrow",
			pub:  time.Date(2026, 10, 23, 10, 0, 0, 0, loc),
			want: "2026-10-23",
		},
		{
			name: "future date next month",
			pub:  time.Date(2026, 11, 20, 10, 0, 0, 0, loc),
			want: "2026-11-20",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatPublishedDate(tc.pub, now)
			if got != tc.want {
				t.Fatalf("FormatPublishedDate(%v, %v) = %q, want %q", tc.pub, now, got, tc.want)
			}
		})
	}
}

func TestFormatPublishedDateMonthYearAndLeapBoundaries(t *testing.T) {
	loc := time.UTC

	// Month boundary: Nov 1 looking back at late October
	nowNov1 := time.Date(2026, 11, 1, 10, 0, 0, 0, loc)
	if got := FormatPublishedDate(time.Date(2026, 10, 31, 22, 0, 0, 0, loc), nowNov1); got != "Yesterday" {
		t.Fatalf("month boundary yesterday = %q, want Yesterday", got)
	}
	if got := FormatPublishedDate(time.Date(2026, 10, 25, 10, 0, 0, 0, loc), nowNov1); got != "A week back" {
		t.Fatalf("month boundary 7 days = %q, want 'A week back'", got)
	}
	if got := FormatPublishedDate(time.Date(2026, 10, 24, 10, 0, 0, 0, loc), nowNov1); got != "2026-10-24" {
		t.Fatalf("month boundary 8 days = %q, want 2026-10-24", got)
	}

	// Year boundary: Jan 1 looking back at late December
	nowJan1 := time.Date(2027, 1, 1, 1, 0, 0, 0, loc)
	if got := FormatPublishedDate(time.Date(2026, 12, 31, 23, 0, 0, 0, loc), nowJan1); got != "Yesterday" {
		t.Fatalf("year boundary yesterday = %q, want Yesterday", got)
	}
	if got := FormatPublishedDate(time.Date(2026, 12, 25, 10, 0, 0, 0, loc), nowJan1); got != "A week back" {
		t.Fatalf("year boundary 7 days = %q, want 'A week back'", got)
	}
	if got := FormatPublishedDate(time.Date(2026, 12, 24, 10, 0, 0, 0, loc), nowJan1); got != "2026-12-24" {
		t.Fatalf("year boundary 8 days = %q, want 2026-12-24", got)
	}

	// Leap year: 2024 is a leap year (Feb 29 exists)
	nowLeap := time.Date(2024, 3, 1, 12, 0, 0, 0, loc)
	if got := FormatPublishedDate(time.Date(2024, 2, 29, 12, 0, 0, 0, loc), nowLeap); got != "Yesterday" {
		t.Fatalf("leap day yesterday = %q, want Yesterday", got)
	}
	// 7 days before Mar 1 in leap year: Feb 23 (29, 28, 27, 26, 25, 24, 23)
	if got := FormatPublishedDate(time.Date(2024, 2, 23, 12, 0, 0, 0, loc), nowLeap); got != "A week back" {
		t.Fatalf("leap year 7 days = %q, want 'A week back'", got)
	}
	if got := FormatPublishedDate(time.Date(2024, 2, 22, 12, 0, 0, 0, loc), nowLeap); got != "2024-02-22" {
		t.Fatalf("leap year 8 days = %q, want 2024-02-22", got)
	}
}

func TestFormatPublishedDateDaylightSavingTime(t *testing.T) {
	// America/New_York undergoes DST changes.
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("America/New_York timezone not available")
	}

	// Fall back transition: Sunday, Nov 1, 2026 (25-hour day)
	// Published on Saturday Oct 31, checked on Monday Nov 2 -> 2 calendar days
	nowFallBack := time.Date(2026, 11, 2, 12, 0, 0, 0, ny)
	pubBeforeFallBack := time.Date(2026, 10, 31, 12, 0, 0, 0, ny)
	if got := FormatPublishedDate(pubBeforeFallBack, nowFallBack); got != "2 days back" {
		t.Fatalf("fall back 2 calendar days = %q, want '2 days back'", got)
	}

	// Spring forward transition: Sunday, Mar 8, 2026 (23-hour day)
	// Published on Saturday Mar 7, checked on Monday Mar 9 -> 2 calendar days
	nowSpringForward := time.Date(2026, 3, 9, 12, 0, 0, 0, ny)
	pubBeforeSpring := time.Date(2026, 3, 7, 12, 0, 0, 0, ny)
	if got := FormatPublishedDate(pubBeforeSpring, nowSpringForward); got != "2 days back" {
		t.Fatalf("spring forward 2 calendar days = %q, want '2 days back'", got)
	}
}

func TestFormatPublishedDateTimezoneConversion(t *testing.T) {
	// Publication timestamp in UTC, user viewing in Tokyo (UTC+9)
	tokyo := time.FixedZone("JST", 9*3600)
	// 2026-10-22 23:00 UTC is 2026-10-23 08:00 JST
	pubUTC := time.Date(2026, 10, 22, 23, 0, 0, 0, time.UTC)

	// User in Tokyo on 2026-10-23 10:00 JST sees it as Today
	nowTokyo := time.Date(2026, 10, 23, 10, 0, 0, 0, tokyo)
	if got := FormatPublishedDate(pubUTC, nowTokyo); got != "Today" {
		t.Fatalf("tokyo user today = %q, want Today", got)
	}

	// User in New York (UTC-4) at the same moment: 2026-10-22 21:00 EDT sees it as Today
	ny := time.FixedZone("EDT", -4*3600)
	nowNY := time.Date(2026, 10, 22, 21, 0, 0, 0, ny)
	if got := FormatPublishedDate(pubUTC, nowNY); got != "Today" {
		t.Fatalf("ny user today = %q, want Today", got)
	}
}

func TestFormatPublishedDateDefaultClock(t *testing.T) {
	// Verify package-level helper calls default currentTime
	old := currentTime
	defer func() { currentTime = old }()

	currentTime = func() time.Time {
		return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	}

	pub := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	if got := formatPublishedDate(pub); got != "Yesterday" {
		t.Fatalf("default clock helper got %q, want Yesterday", got)
	}
}
