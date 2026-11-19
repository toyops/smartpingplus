package http

import (
	"testing"
	"time"
)

func TestTimeParseInLocation(t *testing.T) {
	input := "2026-11-19 14:30"
	parsed, err := time.ParseInLocation("2006-01-02 15:04", input, time.Local)
	if err != nil {
		t.Fatalf("unexpected error parsing time: %v", err)
	}

	formatted := parsed.Format("2006-01-02 15:04")
	if formatted != input {
		t.Errorf("expected %s, got %s", input, formatted)
	}

	// Format from Unix timestamp must match input exactly in time.Local
	fromUnix := time.Unix(parsed.Unix(), 0).Format("2006-01-02 15:04")
	if fromUnix != input {
		t.Errorf("Unix roundtrip mismatch: expected %s, got %s", input, fromUnix)
	}
}
