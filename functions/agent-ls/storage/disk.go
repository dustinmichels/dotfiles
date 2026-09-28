package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type devIno struct {
	dev uint64
	ino uint64
}

// GetAllocatedSize calculates the physical disk space (in bytes) allocated to the given paths,
// using stat.Blocks * 512 without following symlinks and avoiding double-counting inodes.
func GetAllocatedSize(paths []string) int64 {
	var total int64
	seen := make(map[devIno]bool)

	for _, p := range paths {
		if p == "" {
			continue
		}
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
			key := devIno{dev: uint64(stat.Dev), ino: stat.Ino}
			if !seen[key] {
				seen[key] = true
				total += stat.Blocks * 512
			}
		}

		if fi.IsDir() {
			_ = filepath.Walk(p, func(subPath string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 {
					return nil
				}
				if stat, ok := info.Sys().(*syscall.Stat_t); ok {
					key := devIno{dev: uint64(stat.Dev), ino: stat.Ino}
					if !seen[key] {
						seen[key] = true
						total += stat.Blocks * 512
					}
				}
				return nil
			})
		}
	}

	return total
}

type TrashManifest struct {
	Tool      string    `json:"tool"`
	SessionID string    `json:"session_id"`
	TrashedAt time.Time `json:"trashed_at"`
	Files     []string  `json:"files"`
}

type renameRecord struct {
	src string
	dst string
}

// MoveToTrash moves files/directories to ~/.Trash/agent-ls/<timestamp>_<tool>_<id>/
// preserving their relative path under $HOME, so that different stores with identical
// basenames (e.g. antigravity and antigravity-ide) never collide.
// If any move fails, prior moves are rolled back, and the trash directory is preserved
// if any rollback rename fails.
func MoveToTrash(paths []string, tool, id string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sessionTrashDir := filepath.Join(home, ".Trash", "agent-ls",
		fmt.Sprintf("%s_%s_%s", time.Now().Format("20060102-150405"), tool, id))

	var moved []renameRecord
	var manifestFiles []string

	rollback := func(origErr error) error {
		var rollbackErrs []string
		allReverted := true
		for i := len(moved) - 1; i >= 0; i-- {
			rec := moved[i]
			if err := os.Rename(rec.dst, rec.src); err != nil {
				allReverted = false
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("failed reverting %s: %v", rec.dst, err))
			}
		}
		if allReverted {
			_ = os.RemoveAll(sessionTrashDir)
			return origErr
		}
		return fmt.Errorf("%w; rollback partially failed (files preserved in %s): %s",
			origErr, sessionTrashDir, strings.Join(rollbackErrs, "; "))
	}

	for _, p := range paths {
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			continue
		}

		// Calculate relative destination under trashDir
		relPath := p
		if strings.HasPrefix(p, home) {
			relPath = strings.TrimPrefix(p, home)
			relPath = strings.TrimPrefix(relPath, "/")
		} else {
			relPath = strings.TrimPrefix(relPath, "/")
		}

		dest := filepath.Join(sessionTrashDir, relPath)
		destDir := filepath.Dir(dest)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return rollback(fmt.Errorf("creating trash subdir %s: %w", destDir, err))
		}

		if err := os.Rename(p, dest); err != nil {
			return rollback(fmt.Errorf("moving %s to trash: %w", p, err))
		}

		moved = append(moved, renameRecord{src: p, dst: dest})
		manifestFiles = append(manifestFiles, p)
	}

	// Write manifest for full audit & restore capability
	manifest := TrashManifest{
		Tool:      tool,
		SessionID: id,
		TrashedAt: time.Now(),
		Files:     manifestFiles,
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionTrashDir, "manifest.json"), manifestBytes, 0644)

	return nil
}

// RestoreFromTrash restores a session folder from ~/.Trash/agent-ls/<sessionDir> back to original paths
func RestoreFromTrash(sessionTrashDir string) error {
	manifestPath := filepath.Join(sessionTrashDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading trash manifest: %w", err)
	}

	var manifest TrashManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parsing trash manifest: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	for _, origPath := range manifest.Files {
		relPath := origPath
		if strings.HasPrefix(origPath, home) {
			relPath = strings.TrimPrefix(origPath, home)
			relPath = strings.TrimPrefix(relPath, "/")
		} else {
			relPath = strings.TrimPrefix(relPath, "/")
		}

		trashFile := filepath.Join(sessionTrashDir, relPath)
		if _, err := os.Lstat(trashFile); err != nil {
			continue
		}

		origDir := filepath.Dir(origPath)
		_ = os.MkdirAll(origDir, 0755)

		if err := os.Rename(trashFile, origPath); err != nil {
			return fmt.Errorf("restoring %s: %w", origPath, err)
		}
	}

	_ = os.Remove(manifestPath)
	_ = os.RemoveAll(sessionTrashDir)
	return nil
}

// FormatBytes formats byte count into human-readable string (B, KB, MB, GB).
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

// FormatDuration formats duration into human-readable age (e.g. "2h ago", "5d ago").
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	if days < 365 {
		return fmt.Sprintf("%dmo ago", days/30)
	}
	return fmt.Sprintf("%dy ago", days/365)
}
