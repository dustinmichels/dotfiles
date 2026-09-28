package providers

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/storage"
)

type CodexProvider struct {
	baseDir string
}

func NewCodexProvider() *CodexProvider {
	home, _ := os.UserHomeDir()
	return &CodexProvider{
		baseDir: filepath.Join(home, ".codex"),
	}
}

func (p *CodexProvider) Name() string {
	return "codex"
}

type codexIndexItem struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

func (p *CodexProvider) loadIndex() map[string]codexIndexItem {
	items := make(map[string]codexIndexItem)
	indexPath := filepath.Join(p.baseDir, "session_index.jsonl")
	f, err := os.Open(indexPath)
	if err != nil {
		return items
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var item codexIndexItem
		if err := json.Unmarshal([]byte(line), &item); err == nil && item.ID != "" {
			items[item.ID] = item
		}
	}
	return items
}

func (p *CodexProvider) Scan() ([]models.Session, error) {
	var sessions []models.Session
	index := p.loadIndex()
	sessRoot := filepath.Join(p.baseDir, "sessions")

	if _, err := os.Stat(sessRoot); os.IsNotExist(err) {
		return sessions, nil
	}

	_ = filepath.Walk(sessRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			return nil
		}

		// rollout-2026-06-22T21-36-12-019ef21e-e878-7193-a69a-d883cc0cf787.jsonl
		base := strings.TrimSuffix(name, ".jsonl")
		uuid := base
		parts := strings.Split(base, "-")
		if len(parts) >= 5 {
			uuid = strings.Join(parts[len(parts)-5:], "-")
		}

		title := ""
		lastActive := info.ModTime()

		if item, ok := index[uuid]; ok {
			if item.ThreadName != "" {
				title = item.ThreadName
			}
			if t, err := time.Parse(time.RFC3339, item.UpdatedAt); err == nil {
				lastActive = t
			}
		}

		if title == "" {
			title = base
		}

		paths := []string{path}
		allocated := storage.GetAllocatedSize(paths)

		sessions = append(sessions, models.Session{
			ID:             uuid,
			Tool:           "codex",
			Project:        "workspace",
			Title:          title,
			LastActive:     lastActive,
			AllocatedBytes: allocated,
			IsArchived:     false,
			IsLive:         false,
			Paths:          paths,
		})
		return nil
	})

	return sessions, nil
}
