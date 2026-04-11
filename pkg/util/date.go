package util

import "time"

// ParseDateOnly extracts YYYY-MM-DD from various date formats (ISO datetime, RFC3339, etc.)
func ParseDateOnly(s string) string {
	if s == "" {
		return ""
	}
	// Try RFC3339: 2026-03-24T16:29:24Z or 2026-03-24T16:29:24+07:00
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	// Try ISO with millis: 2026-03-24T16:29:24.019Z
	if t, err := time.Parse("2006-01-02T15:04:05.000Z", s); err == nil {
		return t.Format("2006-01-02")
	}
	// Try plain datetime: 2026-03-24T16:29:24
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.Format("2006-01-02")
	}
	// Already YYYY-MM-DD or other, truncate at 10 chars
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
