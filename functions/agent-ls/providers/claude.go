package providers

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/storage"
)

type ClaudeProvider struct {
	baseDir string
}

func NewClaudeProvider() *ClaudeProvider {
	home, _ := os.UserHomeDir()
	return &ClaudeProvider{
		baseDir: filepath.Join(home, ".claude", "projects"),
	}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func isClaudeRunning() bool {
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func parseClaudeTitle(jsonlPath, sidecarDir string) string {
	customTitlePath := filepath.Join(sidecarDir, "custom-title.json")
	if data, err := os.ReadFile(customTitlePath); err == nil {
		var payload struct {
			CustomTitle string `json:"customTitle"`
		}
		if err := json.Unmarshal(data, &payload); err == nil && payload.CustomTitle != "" {
			return payload.CustomTitle
		}
	}

	// Try reading first line with text
	f, err := os.Open(jsonlPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var item struct {
			Type    string `json:"type"`
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &item); err == nil {
			if str, ok := item.Message.Content.(string); ok && str != "" {
				text := strings.TrimSpace(str)
				if len(text) > 60 {
					return text[:57] + "..."
				}
				return text
			}
			if slice, ok := item.Message.Content.([]any); ok && len(slice) > 0 {
				if first, ok := slice[0].(map[string]any); ok {
					if t, ok := first["text"].(string); ok && t != "" {
						text := strings.TrimSpace(t)
						if len(text) > 60 {
							return text[:57] + "..."
						}
						return text
					}
				}
			}
		}
	}
	return ""
}

func (p *ClaudeProvider) Scan() ([]models.Session, error) {
	var sessions []models.Session
	entries, err := os.ReadDir(p.baseDir)
	if err != nil {
		return nil, nil
	}

	claudeRunning := isClaudeRunning()

	for _, projEntry := range entries {
		if !projEntry.IsDir() {
			continue
		}
		projSlug := projEntry.Name()
		projPath := filepath.Join(p.baseDir, projSlug)
		projName := cleanProjectSlug(projSlug)

		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			name := f.Name()
			if !strings.HasSuffix(name, ".jsonl") || strings.HasPrefix(name, ".") {
				continue
			}

			uuid := strings.TrimSuffix(name, ".jsonl")
			jsonlPath := filepath.Join(projPath, name)
			sidecarDir := filepath.Join(projPath, uuid)

			fi, err := os.Stat(jsonlPath)
			if err != nil {
				continue
			}

			paths := []string{jsonlPath}
			if _, err := os.Stat(sidecarDir); err == nil {
				paths = append(paths, sidecarDir)
			}

			title := parseClaudeTitle(jsonlPath, sidecarDir)
			if title == "" {
				title = uuid
			}

			allocated := storage.GetAllocatedSize(paths)

			sessions = append(sessions, models.Session{
				ID:             uuid,
				Tool:           "claude",
				Project:        projName,
				Title:          title,
				LastActive:     fi.ModTime(),
				AllocatedBytes: allocated,
				IsArchived:     false,
				IsLive:         claudeRunning,
				Paths:          paths,
			})
		}
	}

	return sessions, nil
}
