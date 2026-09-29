package git

import (
	"testing"
)

func TestRunTokei(t *testing.T) {
	if !IsTokeiAvailable() {
		t.Skip("tokei not installed; skipping test")
	}

	stats, err := RunTokei(".")
	if err != nil {
		t.Fatalf("unexpected error running tokei: %v", err)
	}

	if !stats.Loaded {
		t.Errorf("expected stats.Loaded to be true")
	}

	if stats.Total.Code <= 0 {
		t.Errorf("expected Total.Code > 0, got %d", stats.Total.Code)
	}

	if len(stats.Languages) == 0 {
		t.Errorf("expected at least 1 language parsed, got 0")
	}

	// Verify Go is in languages
	foundGo := false
	for _, l := range stats.Languages {
		if l.Name == "Go" {
			foundGo = true
			if l.Code <= 0 {
				t.Errorf("expected Go code lines > 0, got %d", l.Code)
			}
		}
	}

	if !foundGo {
		t.Errorf("expected Go language in tokei stats")
	}
}
