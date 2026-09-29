package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMetricCycling(t *testing.T) {
	// Verify that cycling through all metrics returns to the start
	start := MetricCode
	curr := start
	seen := make(map[Metric]bool)

	for range len(MetricList) {
		seen[curr] = true
		curr = NextMetric(curr)
	}

	if curr != start {
		t.Errorf("expected cycle to return to %v, got %v", start, curr)
	}
	if len(seen) != len(MetricList) {
		t.Errorf("expected to visit %d metrics, visited %d", len(MetricList), len(seen))
	}

	// Verify PrevMetric works in reverse
	prev := PrevMetric(NextMetric(MetricCode))
	if prev != MetricCode {
		t.Errorf("expected PrevMetric(NextMetric(Code)) == Code, got %v", prev)
	}
}

func TestCalculateHorizontalSegments(t *testing.T) {
	snap := &CommitSnapshot{
		Languages: map[string]LanguageStats{
			"Go":     {Name: "Go", Code: 600, Lines: 800, Files: 10},
			"Python": {Name: "Python", Code: 300, Lines: 400, Files: 5},
			"Shell":  {Name: "Shell", Code: 100, Lines: 150, Files: 2},
		},
		Total: LanguageStats{
			Name:  "Total",
			Code:  1000,
			Lines: 1350,
			Files: 17,
		},
	}

	totalVal, barWidth, segments := CalculateHorizontalSegments(snap, MetricCode, 40, 1000)

	if totalVal != 1000 {
		t.Errorf("expected totalVal=1000, got %d", totalVal)
	}
	if barWidth != 40 {
		t.Errorf("expected barWidth=40, got %d", barWidth)
	}

	allocatedChars := 0
	for _, seg := range segments {
		allocatedChars += seg.Chars
		if seg.Chars <= 0 {
			t.Errorf("segment for %s has non-positive chars: %d", seg.Language, seg.Chars)
		}
	}

	if allocatedChars != barWidth {
		t.Errorf("expected sum of segment chars == %d, got %d", barWidth, allocatedChars)
	}

	// Go should have the largest segment (60% of 40 = 24 chars)
	if segments[0].Language != "Go" || segments[0].Chars != 24 {
		t.Errorf("expected Go to have 24 chars, got %s with %d chars", segments[0].Language, segments[0].Chars)
	}
}

func TestCalculateHorizontalSegmentsZeroMax(t *testing.T) {
	snap := &CommitSnapshot{
		Languages: map[string]LanguageStats{},
		Total:     LanguageStats{},
	}
	totalVal, barWidth, segments := CalculateHorizontalSegments(snap, MetricCode, 40, 0)
	if totalVal != 0 || barWidth != 0 || len(segments) != 0 {
		t.Errorf("expected 0 for empty snapshot, got %d, %d, %d", totalVal, barWidth, len(segments))
	}
}

func TestCalculateVerticalColumn(t *testing.T) {
	snap := &CommitSnapshot{
		Languages: map[string]LanguageStats{
			"Go":   {Name: "Go", Code: 800},
			"Rust": {Name: "Rust", Code: 200},
		},
		Total: LanguageStats{
			Name: "Total",
			Code: 1000,
		},
	}

	chartHeight := 10
	col := CalculateVerticalColumn(snap, MetricCode, chartHeight, 1000)

	if len(col) != chartHeight {
		t.Fatalf("expected col length %d, got %d", chartHeight, len(col))
	}

	goColor := GetLanguageColor("Go")
	rustColor := GetLanguageColor("Rust")

	// Bottom 8 rows should be Go, top 2 rows Rust
	for i := range 8 {
		if col[i] != goColor {
			t.Errorf("expected row %d to be Go color %s, got %s", i, goColor, col[i])
		}
	}
	for i := 8; i < 10; i++ {
		if col[i] != rustColor {
			t.Errorf("expected row %d to be Rust color %s, got %s", i, rustColor, col[i])
		}
	}
}

func TestComputeDiffs(t *testing.T) {
	snap1 := &CommitSnapshot{
		Hash: "commit1",
		Languages: map[string]LanguageStats{
			"Go": {Name: "Go", Code: 100, Files: 2},
		},
		Total: LanguageStats{Code: 100, Files: 2},
	}

	snap2 := &CommitSnapshot{
		Hash: "commit2",
		Languages: map[string]LanguageStats{
			"Go":     {Name: "Go", Code: 150, Files: 3},
			"Python": {Name: "Python", Code: 50, Files: 1},
		},
		Total: LanguageStats{Code: 200, Files: 4},
	}

	snapshots := []*CommitSnapshot{snap1, snap2}
	ComputeDiffs(snapshots)

	// snap1 vs prev (no prev)
	if snap1.TotalDiffPrev.Code != 0 {
		t.Errorf("expected snap1 TotalDiffPrev.Code == 0, got %d", snap1.TotalDiffPrev.Code)
	}

	// snap1 vs latest (snap2 is latest, so 100 - 200 = -100)
	if snap1.TotalDiffLatest.Code != -100 {
		t.Errorf("expected snap1 TotalDiffLatest.Code == -100, got %d", snap1.TotalDiffLatest.Code)
	}

	// snap2 vs prev (200 - 100 = +100)
	if snap2.TotalDiffPrev.Code != 100 {
		t.Errorf("expected snap2 TotalDiffPrev.Code == 100, got %d", snap2.TotalDiffPrev.Code)
	}
	if snap2.DiffPrev["Go"].Code != 50 {
		t.Errorf("expected snap2 DiffPrev[Go].Code == 50, got %d", snap2.DiffPrev["Go"].Code)
	}
	if snap2.DiffPrev["Python"].Code != 50 {
		t.Errorf("expected snap2 DiffPrev[Python].Code == 50, got %d", snap2.DiffPrev["Python"].Code)
	}

	// snap2 vs latest (snap2 is latest, so 0)
	if snap2.TotalDiffLatest.Code != 0 {
		t.Errorf("expected snap2 TotalDiffLatest.Code == 0, got %d", snap2.TotalDiffLatest.Code)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{1234567, "1,234,567"},
		{-500, "-500"},
		{-1234, "-1,234"},
	}

	for _, tc := range tests {
		actual := formatNumber(tc.input)
		if actual != tc.expected {
			t.Errorf("formatNumber(%d): expected %q, got %q", tc.input, tc.expected, actual)
		}
	}
}

func TestFormatCompactNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{500, "500"},
		{1200, "1.2k"},
		{15400, "15k"},
		{2500000, "2.5M"},
	}

	for _, tc := range tests {
		actual := formatCompactNumber(tc.input)
		if actual != tc.expected {
			t.Errorf("formatCompactNumber(%d): expected %q, got %q", tc.input, tc.expected, actual)
		}
	}
}

func TestFormatDelta(t *testing.T) {
	if FormatDelta(15) != "+15" {
		t.Errorf("expected +15, got %s", FormatDelta(15))
	}
	if FormatDelta(-7) != "-7" {
		t.Errorf("expected -7, got %s", FormatDelta(-7))
	}
	if FormatDelta(0) != "0" {
		t.Errorf("expected 0, got %s", FormatDelta(0))
	}
}

func TestParseNumstat(t *testing.T) {
	raw := `15	5	main.go
-	-	image.png
3	0	README.md
`
	added, deleted, err := parseNumstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 18 {
		t.Errorf("expected 18 added, got %d", added)
	}
	if deleted != 5 {
		t.Errorf("expected 5 deleted, got %d", deleted)
	}
}

func TestLanguageColors(t *testing.T) {
	// Verify known language
	goCol := GetLanguageColor("Go")
	if !strings.HasPrefix(goCol, "#") {
		t.Errorf("expected valid hex color for Go, got %s", goCol)
	}

	// Verify unknown language produces deterministic color
	u1 := GetLanguageColor("MadeUpLanguageXYZ")
	u2 := GetLanguageColor("MadeUpLanguageXYZ")
	if u1 != u2 {
		t.Errorf("expected deterministic color for unknown language, got %s and %s", u1, u2)
	}
}

func TestModelViewHorizontalAndVertical(t *testing.T) {
	snap1 := &CommitSnapshot{
		Hash:         "aaa1111",
		ShortHash:    "aaa1111",
		Author:       "Alice",
		Date:         "2026-09-01",
		RelativeDate: "2d ago",
		Subject:      "Initial commit",
		Languages: map[string]LanguageStats{
			"Go": {Name: "Go", Code: 500, Lines: 600, Files: 5},
		},
		Total: LanguageStats{Code: 500, Lines: 600, Files: 5},
	}
	snap2 := &CommitSnapshot{
		Hash:         "bbb2222",
		ShortHash:    "bbb2222",
		Author:       "Bob",
		Date:         "2026-09-02",
		RelativeDate: "1d ago",
		Subject:      "Add python scripts",
		Languages: map[string]LanguageStats{
			"Go":     {Name: "Go", Code: 600, Lines: 700, Files: 6},
			"Python": {Name: "Python", Code: 200, Lines: 250, Files: 2},
		},
		Total: LanguageStats{Code: 800, Lines: 950, Files: 8},
	}
	snapshots := []*CommitSnapshot{snap1, snap2}
	ComputeDiffs(snapshots)

	m := NewModel(".", snapshots, 10)
	m.width = 120
	m.height = 35

	// Horizontal view check
	m.viewMode = ViewHorizontal
	viewH := m.View()
	if !strings.Contains(viewH, "TOKEI-TIME") {
		t.Errorf("horizontal view missing TOKEI-TIME: %s", viewH)
	}
	if !strings.Contains(viewH, "BREAKDOWN") {
		t.Errorf("horizontal view missing BREAKDOWN: %s", viewH)
	}
	if !strings.Contains(viewH, "Go") {
		t.Errorf("horizontal view missing Go: %s", viewH)
	}

	// Vertical view check
	m.viewMode = ViewVertical
	viewV := m.View()
	if !strings.Contains(viewV, "TOKEI-TIME") {
		t.Errorf("vertical view missing TOKEI-TIME: %s", viewV)
	}
	if !strings.Contains(viewV, "┴") {
		t.Errorf("vertical view missing axis: %s", viewV)
	}
}

func TestModelKeyNavigation(t *testing.T) {
	snap1 := &CommitSnapshot{Hash: "1", Total: LanguageStats{Code: 100}}
	snap2 := &CommitSnapshot{Hash: "2", Total: LanguageStats{Code: 200}}
	snapshots := []*CommitSnapshot{snap1, snap2}
	ComputeDiffs(snapshots)

	m := NewModel(".", snapshots, 10)
	m.width = 100
	m.height = 30

	// Tab switches metric
	origMetric := m.activeMetric
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mUpdated.(Model)
	if m.activeMetric == origMetric {
		t.Errorf("expected metric to change after Tab, remained %v", origMetric)
	}

	// View toggle
	if m.viewMode != ViewHorizontal {
		t.Errorf("expected initial viewMode ViewHorizontal")
	}
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = mUpdated.(Model)
	if m.viewMode != ViewVertical {
		t.Errorf("expected viewMode to be ViewVertical after 'v'")
	}

	// Diff mode toggle
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = mUpdated.(Model)
	if m.diffMode != DiffLatest {
		t.Errorf("expected diffMode to be DiffLatest after 'd'")
	}
}

func TestRealRepoSmoke(t *testing.T) {
	root, err := GetGitRoot(".")
	if err != nil {
		t.Skipf("skipping git test: %v", err)
	}
	if err := CheckTokeiInstalled(); err != nil {
		t.Skipf("skipping tokei test: %v", err)
	}

	snapshots, err := CollectHistory(root, 2, nil)
	if err != nil {
		t.Fatalf("CollectHistory failed: %v", err)
	}
	if len(snapshots) == 0 {
		t.Fatalf("expected at least 1 snapshot, got 0")
	}
	lastSnap := snapshots[len(snapshots)-1]
	if lastSnap.Total.Lines == 0 && lastSnap.Total.Files == 0 {
		t.Errorf("expected non-zero total lines/files in last snapshot")
	}
}
