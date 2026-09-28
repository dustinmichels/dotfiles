package providers

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/storage"
	_ "modernc.org/sqlite"
)

type AntigravityProvider struct {
	baseDir    string
	ideDir     string
	brainDir   string
	convDir    string
	ideConvDir string
}

func NewAntigravityProvider() *AntigravityProvider {
	home, _ := os.UserHomeDir()
	gemini := filepath.Join(home, ".gemini")
	return &AntigravityProvider{
		baseDir:    filepath.Join(gemini, "antigravity"),
		ideDir:     filepath.Join(gemini, "antigravity-ide"),
		brainDir:   filepath.Join(gemini, "antigravity", "brain"),
		convDir:    filepath.Join(gemini, "antigravity", "conversations"),
		ideConvDir: filepath.Join(gemini, "antigravity-ide", "conversations"),
	}
}

func (p *AntigravityProvider) Name() string {
	return "antigravity"
}

func isAntigravityRunning() bool {
	out, err := exec.Command("pgrep", "-i", "antigravity").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
	}
	out, err = exec.Command("pgrep", "-i", "agentapi").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
	}
	return false
}

func parseWorkspaceProject(workspaceURIs string) string {
	var uris []string
	if err := json.Unmarshal([]byte(workspaceURIs), &uris); err == nil && len(uris) > 0 {
		raw := uris[0]
		if u, err := url.Parse(raw); err == nil {
			return filepath.Base(u.Path)
		}
		return filepath.Base(raw)
	}
	return "workspace"
}

func (p *AntigravityProvider) readBrainTitle(convoID string) string {
	walkthrough := filepath.Join(p.brainDir, convoID, "walkthrough.md")
	if f, err := os.Open(walkthrough); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "# ") {
				title := strings.TrimPrefix(line, "# ")
				title = strings.TrimPrefix(title, "Walkthrough: ")
				return strings.TrimSpace(title)
			}
		}
	}
	return ""
}

type convoMeta struct {
	id         string
	title      string
	project    string
	lastActive time.Time
}

func (p *AntigravityProvider) loadSummaries() map[string]convoMeta {
	metas := make(map[string]convoMeta)
	dbPath := filepath.Join(p.baseDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		return metas
	}

	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return metas
	}
	defer db.Close()

	rows, err := db.Query("SELECT conversation_id, title, workspace_uris, datetime(last_modified_time) FROM conversation_summaries")
	if err != nil {
		return metas
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, uris, dtStr string
		if err := rows.Scan(&id, &title, &uris, &dtStr); err == nil && id != "" {
			t, _ := time.Parse("2006-01-02 15:04:05", dtStr)
			metas[id] = convoMeta{
				id:         id,
				title:      title,
				project:    parseWorkspaceProject(uris),
				lastActive: t,
			}
		}
	}
	return metas
}

func (p *AntigravityProvider) Scan() ([]models.Session, error) {
	var sessions []models.Session
	metas := p.loadSummaries()
	appRunning := isAntigravityRunning()

	entries, err := os.ReadDir(p.convDir)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".db") || strings.HasPrefix(name, ".") {
			continue
		}

		convoID := strings.TrimSuffix(name, ".db")
		meta, hasMeta := metas[convoID]

		title := meta.title
		if title == "" {
			title = p.readBrainTitle(convoID)
		}
		if title == "" {
			title = convoID
		}

		project := meta.project
		if project == "" {
			project = "workspace"
		}

		lastActive := meta.lastActive
		dbFile := filepath.Join(p.convDir, name)
		if lastActive.IsZero() {
			if fi, err := os.Stat(dbFile); err == nil {
				lastActive = fi.ModTime()
			}
		}

		var paths []string
		paths = append(paths, dbFile)
		for _, sidecar := range []string{convoID + ".db-wal", convoID + ".db-shm"} {
			sp := filepath.Join(p.convDir, sidecar)
			if _, err := os.Stat(sp); err == nil {
				paths = append(paths, sp)
			}
		}

		brainPath := filepath.Join(p.brainDir, convoID)
		if _, err := os.Stat(brainPath); err == nil {
			paths = append(paths, brainPath)
		}

		// Include corresponding IDE duplicates so both are kept clean
		ideDb := filepath.Join(p.ideConvDir, name)
		if _, err := os.Stat(ideDb); err == nil {
			paths = append(paths, ideDb)
			for _, sidecar := range []string{convoID + ".db-wal", convoID + ".db-shm"} {
				sp := filepath.Join(p.ideConvDir, sidecar)
				if _, err := os.Stat(sp); err == nil {
					paths = append(paths, sp)
				}
			}
		}

		allocated := storage.GetAllocatedSize(paths)

		deleteDBHook := func() error {
			if isAntigravityRunning() {
				return fmt.Errorf("cannot remove database index while Antigravity is running")
			}
			for _, base := range []string{p.baseDir, p.ideDir} {
				sumDb := filepath.Join(base, "conversation_summaries.db")
				if _, err := os.Stat(sumDb); err == nil {
					db, err := sql.Open("sqlite", sumDb)
					if err == nil {
						_, _ = db.Exec("DELETE FROM conversation_summaries WHERE conversation_id = ?", convoID)
						db.Close()
					}
				}
			}
			return nil
		}

		_ = hasMeta
		sessions = append(sessions, models.Session{
			ID:             convoID,
			Tool:           "antigravity",
			Project:        project,
			Title:          title,
			LastActive:     lastActive,
			AllocatedBytes: allocated,
			IsArchived:     false,
			IsLive:         appRunning,
			Paths:          paths,
			DeleteDB:       deleteDBHook,
		})
	}

	return sessions, nil
}
