package main

import (
	"fmt"
	"strings"
	"time"
)

// Metric represents the code metric being visualized.
type Metric int

const (
	MetricCode Metric = iota
	MetricLines
	MetricFiles
	MetricComments
	MetricBlanks
)

var MetricList = []Metric{
	MetricCode,
	MetricLines,
	MetricFiles,
	MetricComments,
	MetricBlanks,
}

var MetricNames = map[Metric]string{
	MetricCode:     "Code",
	MetricLines:    "Lines",
	MetricFiles:    "Files",
	MetricComments: "Comments",
	MetricBlanks:   "Blanks",
}

// NextMetric returns the next metric in the tab cycle.
func NextMetric(current Metric) Metric {
	for i, m := range MetricList {
		if m == current {
			return MetricList[(i+1)%len(MetricList)]
		}
	}
	return MetricCode
}

// PrevMetric returns the previous metric in the tab cycle.
func PrevMetric(current Metric) Metric {
	for i, m := range MetricList {
		if m == current {
			return MetricList[(i-1+len(MetricList))%len(MetricList)]
		}
	}
	return MetricCode
}

// GroupMode represents the aggregation level (individual commits or summary periods).
type GroupMode int

const (
	GroupCommit GroupMode = iota
	GroupDay
	GroupWeek
	GroupMonth
	GroupYear
)

var GroupModeList = []GroupMode{
	GroupCommit,
	GroupDay,
	GroupWeek,
	GroupMonth,
	GroupYear,
}

var GroupModeNames = map[GroupMode]string{
	GroupCommit: "Commit",
	GroupDay:    "Day",
	GroupWeek:   "Week",
	GroupMonth:  "Month",
	GroupYear:   "Year",
}

// NextGroupMode returns the next grouping mode in the cycle.
func NextGroupMode(current GroupMode) GroupMode {
	for i, g := range GroupModeList {
		if g == current {
			return GroupModeList[(i+1)%len(GroupModeList)]
		}
	}
	return GroupCommit
}

// PrevGroupMode returns the previous grouping mode in the cycle.
func PrevGroupMode(current GroupMode) GroupMode {
	for i, g := range GroupModeList {
		if g == current {
			return GroupModeList[(i-1+len(GroupModeList))%len(GroupModeList)]
		}
	}
	return GroupCommit
}

// ParseGroupMode parses a string into a GroupMode.
func ParseGroupMode(s string) (GroupMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "commit", "commits", "c":
		return GroupCommit, nil
	case "day", "days", "d", "daily":
		return GroupDay, nil
	case "week", "weeks", "w", "weekly":
		return GroupWeek, nil
	case "month", "months", "m", "monthly":
		return GroupMonth, nil
	case "year", "years", "y", "yearly":
		return GroupYear, nil
	default:
		return GroupCommit, fmt.Errorf("unknown group mode %q (expected: commit, day, week, month, year)", s)
	}
}

// LanguageStats holds the metrics for a single programming language.
type LanguageStats struct {
	Name     string
	Files    int
	Lines    int
	Code     int
	Comments int
	Blanks   int
}

// Value returns the count for the specified metric.
func (s LanguageStats) Value(m Metric) int {
	switch m {
	case MetricCode:
		return s.Code
	case MetricLines:
		return s.Lines
	case MetricFiles:
		return s.Files
	case MetricComments:
		return s.Comments
	case MetricBlanks:
		return s.Blanks
	default:
		return s.Code
	}
}

// CommitInfo holds basic metadata from git log.
type CommitInfo struct {
	Hash         string
	ShortHash    string
	Author       string
	Date         string
	RelativeDate string
	Subject      string
	Timestamp    time.Time
}

// HistoryInitResult holds initial snapshots and pending commits for lazy loading.
type HistoryInitResult struct {
	InitialSnapshots []*CommitSnapshot
	PendingInfos     []CommitInfo
	TotalCommits     int
	IsDirty          bool
}

// CommitSnapshot represents the complete state of a codebase at a specific commit
// or in the uncommitted working tree.
type CommitSnapshot struct {
	Hash          string
	ShortHash     string
	Author        string
	Date          string
	RelativeDate  string
	Subject       string
	Timestamp     time.Time
	IsWorkingTree bool

	// Summary aggregation metadata
	IsSummary   bool
	CommitCount int
	PeriodLabel string
	PeriodKey   string
	SubCommits  []string
	// Tokei stats
	Languages map[string]LanguageStats
	Total     LanguageStats

	// Diff compared to the immediately preceding commit (chronological parent)
	DiffPrev      map[string]LanguageStats
	TotalDiffPrev LanguageStats

	// Diff compared to the latest commit / working tree
	DiffLatest      map[string]LanguageStats
	TotalDiffLatest LanguageStats

	// Git numstat lines added and deleted (if available)
	LinesAdded   int
	LinesDeleted int
}

// FormatDelta formats an integer difference with an explicit sign and optional zero formatting.
func FormatDelta(delta int) string {
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	} else if delta < 0 {
		return fmt.Sprintf("%d", delta)
	}
	return "0"
}
