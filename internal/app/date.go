package app

import (
	"fmt"
	"time"
)

var currentTime = time.Now

// FormatPublishedDate formats an episode publication timestamp.
// For episodes published within the past seven calendar days (in the user's
// local timezone), it returns human-readable relative text:
//   - Today
//   - Yesterday
//   - 2–6 days back
//   - A week back (exactly 7 calendar days back)
//
// Episodes older than 7 calendar days, future dates, or fallback cases return
// YYYY-MM-DD. Missing/zero timestamps return an empty string.
func FormatPublishedDate(t time.Time, now time.Time) string {
	if t.IsZero() || t.Unix() <= 0 {
		return ""
	}
	loc := now.Location()
	tLocal := t.In(loc)

	// Measure whole calendar days using timezone-neutral UTC midnights of the
	// local Year, Month, and Day. This avoids daylight-saving distortion (23h or
	// 25h days) and calendar rollover issues across months/years.
	todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	pubUTC := time.Date(tLocal.Year(), tLocal.Month(), tLocal.Day(), 0, 0, 0, 0, time.UTC)
	days := int(todayUTC.Sub(pubUTC) / (24 * time.Hour))

	switch {
	case days < 0:
		// Future publication date: preserve standard absolute date.
		return tLocal.Format("2006-01-02")
	case days == 0:
		return "Today"
	case days == 1:
		return "Yesterday"
	case days >= 2 && days <= 6:
		return fmt.Sprintf("%d days back", days)
	case days == 7:
		return "A week back"
	default:
		return tLocal.Format("2006-01-02")
	}
}

func formatPublishedDate(t time.Time) string {
	return FormatPublishedDate(t, currentTime())
}
