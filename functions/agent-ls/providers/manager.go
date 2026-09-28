package providers

import (
	"fmt"
	"os/exec"
	"sort"
	"time"

	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/storage"
)

type Provider interface {
	Name() string
	Scan() ([]models.Session, error)
}

type Manager struct {
	providers []Provider
}

func NewManager() *Manager {
	return &Manager{
		providers: []Provider{
			NewOmpProvider(),
			NewAntigravityProvider(),
			NewClaudeProvider(),
			NewCodexProvider(),
		},
	}
}

func (m *Manager) ScanAll(toolFilter string) ([]models.Session, error) {
	var all []models.Session
	for _, p := range m.providers {
		if toolFilter != "" && toolFilter != "all" && p.Name() != toolFilter {
			continue
		}
		sessions, err := p.Scan()
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}

	// Sort newest first by default
	sort.Slice(all, func(i, j int) bool {
		return all[i].LastActive.After(all[j].LastActive)
	})

	return all, nil
}

func (m *Manager) Summaries(sessions []models.Session, cutoff time.Time) []models.AgentSummary {
	tools := []string{"omp", "antigravity", "claude", "codex"}
	summaries := make(map[string]*models.AgentSummary)
	for _, t := range tools {
		summaries[t] = &models.AgentSummary{Tool: t}
	}

	for _, s := range sessions {
		sum, ok := summaries[s.Tool]
		if !ok {
			sum = &models.AgentSummary{Tool: s.Tool}
			summaries[s.Tool] = sum
			tools = append(tools, s.Tool)
		}
		sum.TotalCount++
		sum.TotalBytes += s.AllocatedBytes

		if s.IsOlderThan(cutoff) {
			sum.OldCount++
			sum.OldBytes += s.AllocatedBytes
		}
	}

	var res []models.AgentSummary
	for _, t := range tools {
		if s, ok := summaries[t]; ok && s.TotalCount > 0 {
			res = append(res, *s)
		}
	}
	return res
}

func (m *Manager) DeleteSession(s models.Session) error {
	if s.Tool == "claude" || s.Tool == "codex" {
		return fmt.Errorf("deletion is not supported for %s (Claude self-prunes transcripts at 30d, Codex manages its own lifecycle)", s.Tool)
	}
	if s.IsLive {
		return fmt.Errorf("session %s is currently live/locked; skipping", s.ID)
	}
	// 1. Move files/dirs to ~/.Trash/agent-ls/...
	if err := storage.MoveToTrash(s.Paths, s.Tool, s.ID); err != nil {
		return err
	}

	// 2. Call DB deletion hook if exists
	if s.DeleteDB != nil {
		if err := s.DeleteDB(); err != nil {
			// Don't fail the whole operation if DB delete fails, but record error
			return fmt.Errorf("files trashed, but DB cleanup failed: %w", err)
		}
	}

	return nil
}

// RunOmpGC runs omp gc --blobs --wal to clean unreferenced blobs and flush WALs
func RunOmpGC() error {
	if _, err := exec.LookPath("omp"); err != nil {
		return nil
	}
	cmd := exec.Command("omp", "gc", "--apply", "--blobs", "--wal")
	return cmd.Run()
}
