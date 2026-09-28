package providers

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/storage"
	_ "modernc.org/sqlite"
)

type OmpProvider struct {
	baseDir string
}

func NewOmpProvider() *OmpProvider {
	home, _ := os.UserHomeDir()
	return &OmpProvider{
		baseDir: filepath.Join(home, ".omp", "agent"),
	}
}

func (p *OmpProvider) Name() string {
	return "omp"
}

func (p *OmpProvider) loadTitles() map[string]string {
	titles := make(map[string]string)
	dbPath := filepath.Join(p.baseDir, "history.db")
	if _, err := os.Stat(dbPath); err != nil {
		return titles
	}

	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return titles
	}
	defer db.Close()

	rows, err := db.Query("SELECT session_id, title FROM session_titles")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title string
			if err := rows.Scan(&id, &title); err == nil && id != "" {
				titles[id] = title
			}
		}
	}
	return titles
}

func (p *OmpProvider) loadLivePaths() map[string]bool {
	live := make(map[string]bool)
	sessDir := filepath.Join(p.baseDir, "sessions")
	out, err := exec.Command("lsof", "+D", sessDir).Output()
	if err != nil && len(out) == 0 {
		return live
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) > 8 {
			path := fields[len(fields)-1]
			live[path] = true
		}
	}
	return live
}

func cleanProjectSlug(slug string) string {
	s := strings.TrimPrefix(slug, "-")
	home, _ := os.UserHomeDir()
	homePrefix := strings.ReplaceAll(strings.TrimPrefix(home, "/"), "/", "-") + "-"
	
	prefixes := []string{
		homePrefix + "GitRepos-",
		homePrefix + "Desktop-",
		homePrefix,
		"GitRepos-",
		"Desktop-",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}
	return s
}

func ParseOmpArchiveTimestamp(fname string) (time.Time, bool) {
	// e.g. 2026-06-23T15-39-51-066Z_019ef523-499a-7000-a405-d1bb440a2714.jsonl.gz
	parts := strings.Split(fname, "_")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	tStr := parts[0]
	if len(tStr) < 19 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02T15-04-05", tStr[:19], time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (p *OmpProvider) Scan() ([]models.Session, error) {
	var sessions []models.Session
	titles := p.loadTitles()
	livePaths := p.loadLivePaths()

	// 1. Active sessions
	activeDir := filepath.Join(p.baseDir, "sessions")
	if entries, err := os.ReadDir(activeDir); err == nil {
		for _, projEntry := range entries {
			if !projEntry.IsDir() {
				continue
			}
			projSlug := projEntry.Name()
			projPath := filepath.Join(activeDir, projSlug)
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

				base := strings.TrimSuffix(name, ".jsonl")
				jsonlPath := filepath.Join(projPath, name)
				sidecarDir := filepath.Join(projPath, base)
				lockPath := filepath.Join(projPath, fmt.Sprintf(".%s.lock.os", name))

				fi, err := os.Stat(jsonlPath)
				if err != nil {
					continue
				}

				lastActive := fi.ModTime()

				paths := []string{jsonlPath}
				if _, err := os.Stat(sidecarDir); err == nil {
					paths = append(paths, sidecarDir)
				}
				if _, err := os.Stat(lockPath); err == nil {
					paths = append(paths, lockPath)
				}

				isLive := false
				for _, p := range paths {
					if livePaths[p] {
						isLive = true
						break
					}
				}

				allocated := storage.GetAllocatedSize(paths)

				// Extract UUID if present: 2026-09-27T15-48-07-107Z_<uuid>
				uuid := base
				if idx := strings.Index(base, "_"); idx != -1 && idx+1 < len(base) {
					uuid = base[idx+1:]
				}

				title := titles[uuid]
				if title == "" {
					title = titles[base]
				}
				if title == "" {
					title = base
				}

				sessions = append(sessions, models.Session{
					ID:             base,
					Tool:           "omp",
					Project:        projName,
					Title:          title,
					LastActive:     lastActive,
					AllocatedBytes: allocated,
					IsArchived:     false,
					IsLive:         isLive,
					Paths:          paths,
				})
			}
		}
	}

	// 2. Archived sessions
	archiveDir := filepath.Join(p.baseDir, "archive", "sessions")
	if entries, err := os.ReadDir(archiveDir); err == nil {
		for _, projEntry := range entries {
			if !projEntry.IsDir() {
				continue
			}
			projSlug := projEntry.Name()
			projPath := filepath.Join(archiveDir, projSlug)
			projName := cleanProjectSlug(projSlug)

			files, err := os.ReadDir(projPath)
			if err != nil {
				continue
			}

			for _, f := range files {
				name := f.Name()
				if !strings.HasSuffix(name, ".jsonl.gz") || strings.HasPrefix(name, ".") {
					continue
				}

				base := strings.TrimSuffix(name, ".jsonl.gz")
				gzPath := filepath.Join(projPath, name)
				sidecarDir := filepath.Join(projPath, base)

				lastActive, ok := ParseOmpArchiveTimestamp(name)
				if !ok {
					fi, _ := os.Stat(gzPath)
					if fi != nil {
						lastActive = fi.ModTime()
					}
				}

				paths := []string{gzPath}
				if _, err := os.Stat(sidecarDir); err == nil {
					paths = append(paths, sidecarDir)
				}

				allocated := storage.GetAllocatedSize(paths)

				uuid := base
				if idx := strings.Index(base, "_"); idx != -1 && idx+1 < len(base) {
					uuid = base[idx+1:]
				}

				title := titles[uuid]
				if title == "" {
					title = titles[base]
				}
				if title == "" {
					title = base
				}

				sessions = append(sessions, models.Session{
					ID:             base,
					Tool:           "omp",
					Project:        projName,
					Title:          title,
					LastActive:     lastActive,
					AllocatedBytes: allocated,
					IsArchived:     true,
					IsLive:         false,
					Paths:          paths,
				})
			}
		}
	}

	return sessions, nil
}
