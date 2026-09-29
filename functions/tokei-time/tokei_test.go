package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	m := NewModel(".", snapshots, 10, GroupCommit)
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

	m := NewModel(".", snapshots, 10, GroupCommit)
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

	// Summary mode toggle ('s')
	if m.activeGroup != GroupCommit {
		t.Errorf("expected initial activeGroup GroupCommit")
	}
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = mUpdated.(Model)
	if m.activeGroup != GroupDay {
		t.Errorf("expected activeGroup to be GroupDay after 's', got %v", m.activeGroup)
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

func TestGroupModeCycling(t *testing.T) {
	start := GroupCommit
	curr := start
	for range len(GroupModeList) {
		curr = NextGroupMode(curr)
	}
	if curr != start {
		t.Errorf("expected full cycle to return to %v, got %v", start, curr)
	}

	// Test PrevGroupMode
	prev := PrevGroupMode(GroupCommit)
	if prev != GroupYear {
		t.Errorf("expected PrevGroupMode(GroupCommit) to be GroupYear, got %v", prev)
	}

	// Test ParseGroupMode
	modes := []string{"commit", "day", "week", "month", "year"}
	for _, modeStr := range modes {
		gm, err := ParseGroupMode(modeStr)
		if err != nil {
			t.Errorf("failed to parse group mode %q: %v", modeStr, err)
		}
		if strings.ToLower(GroupModeNames[gm]) != modeStr {
			t.Errorf("mismatch in parsed group mode name: got %s, want %s", GroupModeNames[gm], modeStr)
		}
	}
}

func TestAggregateSnapshots(t *testing.T) {
	// Create 3 commits on day 1 and 2 commits on day 2
	t1 := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 9, 21, 14, 0, 0, 0, time.UTC)

	s1 := &CommitSnapshot{
		Hash:      "c1",
		Timestamp: t1,
		Languages: map[string]LanguageStats{
			"Go": {Name: "Go", Code: 100, Lines: 120, Files: 2},
		},
		Total: LanguageStats{Code: 100, Lines: 120, Files: 2},
	}
	s2 := &CommitSnapshot{
		Hash:      "c2",
		Timestamp: t2,
		Languages: map[string]LanguageStats{
			"Go": {Name: "Go", Code: 200, Lines: 240, Files: 4},
		},
		Total: LanguageStats{Code: 200, Lines: 240, Files: 4},
	}
	s3 := &CommitSnapshot{
		Hash:      "c3",
		Timestamp: t3,
		Languages: map[string]LanguageStats{
			"Go": {Name: "Go", Code: 300, Lines: 350, Files: 5},
		},
		Total: LanguageStats{Code: 300, Lines: 350, Files: 5},
	}
	s4 := &CommitSnapshot{
		Hash:      "c4",
		Timestamp: t4,
		Languages: map[string]LanguageStats{
			"Go": {Name: "Go", Code: 500, Lines: 600, Files: 7},
		},
		Total: LanguageStats{Code: 500, Lines: 600, Files: 7},
	}

	raw := []*CommitSnapshot{s1, s2, s3, s4}
	ComputeDiffs(raw)

	// Aggregate by Day
	daySummaries := AggregateSnapshots(raw, GroupDay)
	if len(daySummaries) != 2 {
		t.Fatalf("expected 2 day summaries, got %d", len(daySummaries))
	}

	// Day 1 average: (100 + 200)/2 = 150 Code
	day1 := daySummaries[0]
	if day1.CommitCount != 2 {
		t.Errorf("expected 2 commits in day 1, got %d", day1.CommitCount)
	}
	if day1.Total.Code != 150 {
		t.Errorf("expected average code 150 on day 1, got %d", day1.Total.Code)
	}
	if day1.Total.Lines != 180 {
		t.Errorf("expected average lines 180 on day 1, got %d", day1.Total.Lines)
	}
	if day1.Total.Files != 3 {
		t.Errorf("expected average files 3 on day 1, got %d", day1.Total.Files)
	}

	// Day 2 average: (300 + 500)/2 = 400 Code
	day2 := daySummaries[1]
	if day2.CommitCount != 2 {
		t.Errorf("expected 2 commits in day 2, got %d", day2.CommitCount)
	}
	if day2.Total.Code != 400 {
		t.Errorf("expected average code 400 on day 2, got %d", day2.Total.Code)
	}

	// Diff between day 2 and day 1: 400 - 150 = +250
	if day2.TotalDiffPrev.Code != 250 {
		t.Errorf("expected day 2 delta vs day 1 to be +250, got %d", day2.TotalDiffPrev.Code)
	}

	// Aggregate by Month (both in September 2026)
	monthSummaries := AggregateSnapshots(raw, GroupMonth)
	if len(monthSummaries) != 1 {
		t.Fatalf("expected 1 month summary, got %d", len(monthSummaries))
	}
	mo1 := monthSummaries[0]
	if mo1.CommitCount != 4 {
		t.Errorf("expected 4 commits in month, got %d", mo1.CommitCount)
	}
	// Average: (100 + 200 + 300 + 500)/4 = 1100/4 = 275 Code
	if mo1.Total.Code != 275 {
		t.Errorf("expected average code 275 in month summary, got %d", mo1.Total.Code)
	}
}

func TestGraphSizeInvarianceAcrossScrollAndPeriodSwitch(t *testing.T) {
	var snaps []*CommitSnapshot
	for i := range 20 {
		ts := time.Date(2026, 9, 1+i, 12, 0, 0, 0, time.UTC)
		s := &CommitSnapshot{
			Hash:      fmt.Sprintf("hash-%d", i),
			ShortHash: fmt.Sprintf("h%d", i),
			Timestamp: ts,
			Languages: map[string]LanguageStats{
				"Go": {Name: "Go", Code: 100 * (i + 1), Lines: 120 * (i + 1), Files: 2},
			},
			Total: LanguageStats{Code: 100 * (i + 1), Lines: 120 * (i + 1), Files: 2},
		}
		if i%3 == 0 {
			s.Languages["Python"] = LanguageStats{Name: "Python", Code: 50, Lines: 60, Files: 1}
			s.Total.Code += 50
			s.Total.Lines += 60
			s.Total.Files++
		}
		snaps = append(snaps, s)
	}
	ComputeDiffs(snaps)

	m := NewModel(".", snaps, 20, GroupCommit)
	m.width = 120
	m.height = 35

	// 1. Invariance across scroll
	m.selectedIndex = len(snaps) - 1
	m.ensureSelectionVisible()
	hBottom := m.renderHorizontalView()
	linesBottom := strings.Count(hBottom, "\n")

	// Scroll up to top
	m.selectedIndex = 0
	m.ensureSelectionVisible()
	hTop := m.renderHorizontalView()
	linesTop := strings.Count(hTop, "\n")

	if linesBottom != linesTop {
		t.Errorf("horizontal graph height changed while scrolling: bottom=%d lines, top=%d lines", linesBottom, linesTop)
	}

	// 2. Invariance across summary period switches
	modes := []GroupMode{GroupCommit, GroupDay, GroupWeek, GroupMonth, GroupYear}
	for _, mode := range modes {
		m.activeGroup = mode
		m.rebuildDisplay()
		hView := m.renderHorizontalView()
		linesView := strings.Count(hView, "\n")
		if linesView != linesBottom {
			t.Errorf("horizontal graph height changed in mode %s: got %d lines, want %d lines",
				GroupModeNames[mode], linesView, linesBottom)
		}
	}

	// 3. Detail card height invariance between 1-language and 2-language commits
	m.activeGroup = GroupCommit
	m.rebuildDisplay()
	m.selectedIndex = 0 // has Go + Python
	card0 := m.renderDetailCard()
	linesCard0 := strings.Count(card0, "\n")

	m.selectedIndex = 1 // has only Go
	card1 := m.renderDetailCard()
	linesCard1 := strings.Count(card1, "\n")

	if linesCard0 != linesCard1 {
		t.Errorf("detail card height changed between commits: card0=%d lines, card1=%d lines", linesCard0, linesCard1)
	}
}

func TestNoHeaderJitterOrOverflow(t *testing.T) {
	// Test that for various terminal dimensions and commits with very long subjects or many languages:
	// 1. Total rendered lines NEVER exceeds m.height
	// 2. No line ever wraps beyond m.width
	// 3. The first line ALWAYS contains "TOKEI-TIME"
	var snaps []*CommitSnapshot
	for i := range 15 {
		subj := fmt.Sprintf("Commit #%d: extremely long subject line with lots of words designed to test line wrapping inside terminal windows and prevent jitter", i)
		if i%2 == 0 {
			subj = fmt.Sprintf("Commit #%d: short", i)
		}
		s := &CommitSnapshot{
			Hash:         fmt.Sprintf("hash-%d-1234567890abcdef", i),
			ShortHash:    fmt.Sprintf("h%d", i),
			Author:       "Test Author With A Reasonably Long Name",
			Subject:      subj,
			Date:         "2026-09-20",
			RelativeDate: "2 days ago",
			Timestamp:    time.Date(2026, 9, 1+i, 12, 0, 0, 0, time.UTC),
			Languages: map[string]LanguageStats{
				"Go":         {Name: "Go", Code: 100 * (i + 1), Lines: 120 * (i + 1), Files: 2},
				"Python":     {Name: "Python", Code: 50 * (i + 1), Lines: 60 * (i + 1), Files: 1},
				"Rust":       {Name: "Rust", Code: 30 * (i + 1), Lines: 40 * (i + 1), Files: 1},
				"TypeScript": {Name: "TypeScript", Code: 20 * (i + 1), Lines: 25 * (i + 1), Files: 1},
				"JavaScript": {Name: "JavaScript", Code: 10 * (i + 1), Lines: 15 * (i + 1), Files: 1},
				"HTML":       {Name: "HTML", Code: 5 * (i + 1), Lines: 8 * (i + 1), Files: 1},
				"CSS":        {Name: "CSS", Code: 5 * (i + 1), Lines: 7 * (i + 1), Files: 1},
				"Markdown":   {Name: "Markdown", Code: 0, Lines: 50, Files: 2},
			},
			Total: LanguageStats{Code: 220 * (i + 1), Lines: 325 * (i + 1), Files: 10},
		}
		snaps = append(snaps, s)
	}
	ComputeDiffs(snaps)

	sizes := []struct {
		w, h int
	}{
		{70, 24},
		{80, 28},
		{100, 35},
		{120, 40},
		{160, 50},
	}

	for _, sz := range sizes {
		m := NewModel(".", snaps, len(snaps), GroupCommit)
		m.width = sz.w
		m.height = sz.h
		m.ensureSelectionVisible()

		// Test every commit index as selected (scrolling from 0 to len-1)
		for idx := range snaps {
			m.selectedIndex = idx
			m.ensureSelectionVisible()

			view := m.View()
			lines := strings.Split(view, "\n")

			if len(lines) > sz.h {
				t.Errorf("size %dx%d idx=%d: rendered %d lines, exceeded terminal height %d",
					sz.w, sz.h, idx, len(lines), sz.h)
			}

			for lineIdx, line := range lines {
				w := lipgloss.Width(line)
				if w > sz.w {
					t.Errorf("size %dx%d idx=%d line %d: visual width %d exceeded terminal width %d:\n%q",
						sz.w, sz.h, idx, lineIdx, w, sz.w, line)
				}
			}

			if len(lines) > 0 && !strings.Contains(lines[0], "TOKEI-TIME") {
				t.Errorf("size %dx%d idx=%d: first line missing TOKEI-TIME:\n%q", sz.w, sz.h, idx, lines[0])
			}
		}
	}
}

func TestLazyLoadingHistory(t *testing.T) {
	// Create 10 mock commits
	var allInfos []CommitInfo
	for i := range 10 {
		allInfos = append(allInfos, CommitInfo{
			Hash:         fmt.Sprintf("hash-%d", i),
			ShortHash:    fmt.Sprintf("h%d", i),
			Author:       "Author",
			Subject:      fmt.Sprintf("Commit %d", i),
			Date:         "2026-09-01",
			RelativeDate: fmt.Sprintf("%d days ago", 10-i),
			Timestamp:    time.Date(2026, 9, 1+i, 0, 0, 0, 0, time.UTC),
		})
	}

	// Initial batch: last 4 commits (indices 6..9)
	var initialSnaps []*CommitSnapshot
	for i := 6; i < 10; i++ {
		initialSnaps = append(initialSnaps, &CommitSnapshot{
			Hash:      allInfos[i].Hash,
			ShortHash: allInfos[i].ShortHash,
			Author:    allInfos[i].Author,
			Subject:   allInfos[i].Subject,
			Timestamp: allInfos[i].Timestamp,
			Languages: map[string]LanguageStats{"Go": {Name: "Go", Code: 100 * (i + 1)}},
			Total:     LanguageStats{Code: 100 * (i + 1)},
		})
	}
	pending := allInfos[:6] // older commits

	initResult := &HistoryInitResult{
		InitialSnapshots: initialSnaps,
		PendingInfos:     pending,
		TotalCommits:     10,
	}

	m := NewModelWithLazyLoading(".", initResult, 0, GroupCommit)
	if !m.isLazyLoading {
		t.Errorf("expected isLazyLoading to be true")
	}
	if len(m.pendingInfos) != 6 {
		t.Errorf("expected 6 pending infos, got %d", len(m.pendingInfos))
	}
	if len(m.rawSnapshots) != 4 {
		t.Errorf("expected 4 raw snapshots, got %d", len(m.rawSnapshots))
	}

	// Simulate chunk loaded: 3 older commits (indices 3..5)
	var chunk1 []*CommitSnapshot
	for i := 3; i < 6; i++ {
		chunk1 = append(chunk1, &CommitSnapshot{
			Hash:      allInfos[i].Hash,
			ShortHash: allInfos[i].ShortHash,
			Author:    allInfos[i].Author,
			Subject:   allInfos[i].Subject,
			Timestamp: allInfos[i].Timestamp,
			Languages: map[string]LanguageStats{"Go": {Name: "Go", Code: 100 * (i + 1)}},
			Total:     LanguageStats{Code: 100 * (i + 1)},
		})
	}

	updatedModel, cmd := m.Update(chunkLoadedMsg{
		snapshots:        chunk1,
		remainingPending: pending[:3],
		err:              nil,
	})
	m = updatedModel.(Model)

	if cmd == nil {
		t.Errorf("expected next chunk command to be dispatched, got nil")
	}
	if len(m.rawSnapshots) != 7 {
		t.Errorf("expected 7 raw snapshots after chunk 1, got %d", len(m.rawSnapshots))
	}
	// Verify chronological order: oldest to newest
	if m.rawSnapshots[0].Hash != "hash-3" {
		t.Errorf("expected oldest loaded commit hash-3 at index 0, got %s", m.rawSnapshots[0].Hash)
	}
	if m.rawSnapshots[6].Hash != "hash-9" {
		t.Errorf("expected newest loaded commit hash-9 at index 6, got %s", m.rawSnapshots[6].Hash)
	}

	// Simulate final chunk: remaining 3 older commits (indices 0..2)
	var chunk2 []*CommitSnapshot
	for i := range 3 {
		chunk2 = append(chunk2, &CommitSnapshot{
			Hash:      allInfos[i].Hash,
			ShortHash: allInfos[i].ShortHash,
			Author:    allInfos[i].Author,
			Subject:   allInfos[i].Subject,
			Timestamp: allInfos[i].Timestamp,
			Languages: map[string]LanguageStats{"Go": {Name: "Go", Code: 100 * (i + 1)}},
			Total:     LanguageStats{Code: 100 * (i + 1)},
		})
	}

	updatedModel, cmdFinal := m.Update(chunkLoadedMsg{
		snapshots:        chunk2,
		remainingPending: nil,
		err:              nil,
	})
	m = updatedModel.(Model)

	if cmdFinal != nil {
		t.Errorf("expected nil cmd after all commits loaded, got %v", cmdFinal)
	}
	if m.isLazyLoading {
		t.Errorf("expected isLazyLoading to be false when all commits loaded")
	}
	if len(m.rawSnapshots) != 10 {
		t.Errorf("expected 10 raw snapshots, got %d", len(m.rawSnapshots))
	}
	if m.rawSnapshots[0].Hash != "hash-0" {
		t.Errorf("expected hash-0 at index 0, got %s", m.rawSnapshots[0].Hash)
	}
	if m.rawSnapshots[9].Hash != "hash-9" {
		t.Errorf("expected hash-9 at index 9, got %s", m.rawSnapshots[9].Hash)
	}
	if !strings.Contains(m.statusMsg, "All 10 commits loaded") {
		t.Errorf("expected status to say 'All 10 commits loaded', got %q", m.statusMsg)
	}
}
