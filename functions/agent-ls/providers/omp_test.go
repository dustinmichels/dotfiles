package providers

import (
	"testing"
	"time"
)

func TestParseOmpArchiveTimestamp(t *testing.T) {
	filename := "2026-06-23T15-39-51-066Z_019ef523-499a-7000-a405-d1bb440a2714.jsonl.gz"
	ts, ok := ParseOmpArchiveTimestamp(filename)
	if !ok {
		t.Fatalf("expected ParseOmpArchiveTimestamp to succeed for %s", filename)
	}

	expected := time.Date(2026, time.June, 23, 15, 39, 51, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, ts)
	}
}
