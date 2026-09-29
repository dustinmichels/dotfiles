package models

import (
	"fmt"
	"time"
)

// Visibility indicates GitHub repository visibility.
type Visibility string

const (
	VisibilityPublic  Visibility = "PUBLIC"
	VisibilityPrivate Visibility = "PRIVATE"
	VisibilityLocal   Visibility = "LOCAL"
	VisibilityUnknown Visibility = "UNKNOWN"
)

// LanguageStats holds code metric figures for a single programming language.
type LanguageStats struct {
	Name     string `json:"name"`
	Files    int    `json:"files"`
	Code     int    `json:"code"`
	Comments int    `json:"comments"`
	Blanks   int    `json:"blanks"`
	Lines    int    `json:"lines"`
}

// TokeiStats holds aggregate and per-language metrics for a repository.
type TokeiStats struct {
	Loaded    bool            `json:"loaded"`
	Loading   bool            `json:"loading"`
	Err       string          `json:"error,omitempty"`
	Total     LanguageStats   `json:"total"`
	Languages []LanguageStats `json:"languages"`
}

// CommitInfo holds details about the latest git commit.
type CommitInfo struct {
	Hash     string    `json:"hash"`
	Author   string    `json:"author"`
	Date     time.Time `json:"date"`
	Relative string    `json:"relative"`
	Message  string    `json:"message"`
}

// Repo represents a git repository discovered on disk.
type Repo struct {
	Path        string `json:"path"`         // Full absolute path
	RelPath     string `json:"rel_path"`     // Relative to root scan directory
	Name        string `json:"name"`         // Base directory name
	Branch      string `json:"branch"`       // Current branch or HEAD state
	CommitCount int    `json:"commit_count"` // Total commit count on current branch

	// Git status
	HasUncommitted     bool     `json:"has_uncommitted"`
	UncommittedCount   int      `json:"uncommitted_count"`
	UncommittedDetails []string `json:"uncommitted_details"` // e.g. ["M main.go", "?? foo.txt"]

	// Remote / GitHub
	RemoteName       string     `json:"remote_name"`       // e.g. "origin"
	RemoteURL        string     `json:"remote_url"`        // e.g. "git@github.com:dustinmichels/repo-ls.git"
	IsGitHub         bool       `json:"is_github"`         // true if remote is github.com
	GitHubSlug       string     `json:"github_slug"`       // "owner/repo"
	GitHubVisibility Visibility `json:"github_visibility"` // PUBLIC, PRIVATE, LOCAL, UNKNOWN
	GitHubWebURL     string     `json:"github_web_url"`    // https://github.com/owner/repo

	// Last commit
	LastCommit *CommitInfo `json:"last_commit,omitempty"`

	// Creation and modification times
	DateCreated  time.Time `json:"date_created"`
	LastModified time.Time `json:"last_modified"`

	// Tokei stats (loaded on selection / on demand)
	Tokei *TokeiStats `json:"tokei,omitempty"`
}

// RelativeTime formats a time.Time into human-friendly relative string.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins <= 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	case d < 24*time.Hour:
		hrs := int(d.Hours())
		if hrs <= 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hrs)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days <= 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / (24 * 30))
		if months <= 1 {
			return "1mo ago"
		}
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := int(d.Hours() / (24 * 365))
		if years <= 1 {
			return "1y ago"
		}
		return fmt.Sprintf("%dy ago", years)
	}
}

// FilterMode defines the current table filter.
type FilterMode int

const (
	FilterAll FilterMode = iota
	FilterUncommitted
	FilterClean
	FilterGitHub
	FilterLocal
	FilterPublic
	FilterPrivate
)

func (f FilterMode) String() string {
	switch f {
	case FilterUncommitted:
		return "Dirty"
	case FilterClean:
		return "Clean"
	case FilterGitHub:
		return "GitHub"
	case FilterLocal:
		return "Local Only"
	case FilterPublic:
		return "Public"
	case FilterPrivate:
		return "Private"
	default:
		return "All"
	}
}

// SortMode defines the table sort order.
type SortMode int

const (
	SortPath SortMode = iota
	SortName
	SortModified
	SortCreated
	SortDirtyFirst
	SortRecent = SortModified
)

func (s SortMode) String() string {
	switch s {
	case SortName:
		return "Name"
	case SortModified:
		return "Last Modified"
	case SortCreated:
		return "Date Created"
	case SortDirtyFirst:
		return "Dirty First"
	default:
		return "Path"
	}
}
