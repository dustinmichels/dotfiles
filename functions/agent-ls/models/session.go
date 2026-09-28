package models

import "time"

type Session struct {
	ID             string       `json:"id"`
	Tool           string       `json:"tool"`
	Project        string       `json:"project"`
	Title          string       `json:"title"`
	LastActive     time.Time    `json:"last_active"`
	AllocatedBytes int64        `json:"allocated_bytes"`
	IsArchived     bool         `json:"is_archived"`
	IsLive         bool         `json:"is_live"`
	Paths          []string     `json:"paths"`
	DeleteDB       func() error `json:"-"`
}

func (s Session) Age(now time.Time) time.Duration {
	return now.Sub(s.LastActive)
}

func (s Session) IsOlderThan(cutoff time.Time) bool {
	return s.LastActive.Before(cutoff)
}

type AgentSummary struct {
	Tool       string `json:"tool"`
	TotalCount int    `json:"total_count"`
	TotalBytes int64  `json:"total_bytes"`
	OldCount   int    `json:"old_count"`
	OldBytes   int64  `json:"old_bytes"`
}
