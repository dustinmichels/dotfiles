package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CheckGitInstalled verifies that git is available in PATH.
func CheckGitInstalled() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is not installed or not in PATH")
	}
	return nil
}

// GetGitRoot returns the root directory of the git repository containing dir.
func GetGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	return strings.TrimSpace(string(out)), nil
}

// IsWorkingTreeDirty checks if there are uncommitted changes (staged or unstaged).
func IsWorkingTreeDirty(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

// GetRecentCommits returns up to limit recent commits in chronological order (oldest first).
func GetRecentCommits(repoRoot string, limit int) ([]CommitInfo, error) {
	// Format: %H (full hash) NUL %h (short hash) NUL %an (author) NUL %ad (date) NUL %ar (relative date) NUL %s (subject) NUL %aI (ISO strict)
	format := "%H%x00%h%x00%an%x00%ad%x00%ar%x00%s%x00%aI"
	args := []string{"log", "--date=short", fmt.Sprintf("--format=%s", format)}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil, nil
	}

	// git log returns newest first; reverse so oldest is first (timeline order)
	var commits []CommitInfo
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 6 {
			continue
		}
		var ts time.Time
		if len(parts) >= 7 {
			parsed, err := time.Parse(time.RFC3339, parts[6])
			if err == nil {
				ts = parsed
			}
		}
		if ts.IsZero() {
			parsed, err := time.Parse("2006-01-02", parts[3])
			if err == nil {
				ts = parsed
			} else {
				ts = time.Now()
			}
		}

		commits = append(commits, CommitInfo{
			Hash:         parts[0],
			ShortHash:    parts[1],
			Author:       parts[2],
			Date:         parts[3],
			RelativeDate: parts[4],
			Subject:      parts[5],
			Timestamp:    ts,
		})
	}
	return commits, nil
}

// GetCommitDiffNumstat gets lines added and deleted in a commit.
func GetCommitDiffNumstat(repoRoot string, commit string) (int, int, error) {
	cmd := exec.Command("git", "show", "--numstat", "--format=", commit)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	return parseNumstat(string(out))
}

// GetWorkingTreeDiffNumstat gets lines added and deleted in the uncommitted working tree vs HEAD.
func GetWorkingTreeDiffNumstat(repoRoot string) (int, int, error) {
	cmd := exec.Command("git", "diff", "--numstat", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	return parseNumstat(string(out))
}

func parseNumstat(output string) (int, int, error) {
	added := 0
	deleted := 0
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Binary files show "-" instead of numbers
		if fields[0] != "-" {
			if a, err := strconv.Atoi(fields[0]); err == nil {
				added += a
			}
		}
		if fields[1] != "-" {
			if d, err := strconv.Atoi(fields[1]); err == nil {
				deleted += d
			}
		}
	}
	return added, deleted, nil
}

// ExtractCommitToTempDir extracts the files of a commit into a new temporary directory.
// The caller is responsible for deleting the directory with os.RemoveAll.
func ExtractCommitToTempDir(repoRoot string, commit string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "tokei-time-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	cmd := exec.Command("git", "archive", commit)
	cmd.Dir = repoRoot
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to pipe git archive: %w", err)
	}

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to start git archive: %w", err)
	}

	tr := tar.NewReader(stdout)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		target := filepath.Join(tmpDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err == nil {
				io.Copy(f, tr)
				f.Close()
			}
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Symlink(hdr.Linkname, target)
		}
	}

	cmd.Wait()
	return tmpDir, nil
}
