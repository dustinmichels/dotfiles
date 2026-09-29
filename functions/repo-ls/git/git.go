package git

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustinmichels/repo-ls/models"
)

var (
	githubURLRegex = regexp.MustCompile(`(?i)(?:git@github\.com:|https?://github\.com/)([\w.-]+)/([\w.-]+?)(?:\.git)?$`)
)

// ResolveGitDir locates the actual .git directory (handling worktrees and submodules).
func ResolveGitDir(repoPath string) string {
	gitPath := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return gitPath
	}
	// It's a file (e.g. worktree or submodule pointing to gitdir: ...)
	content, err := os.ReadFile(gitPath)
	if err != nil {
		return gitPath
	}
	line := strings.TrimSpace(string(content))
	if strings.HasPrefix(line, "gitdir:") {
		dir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(repoPath, dir)
		}
		return filepath.Clean(dir)
	}
	return gitPath
}

// GetBranch retrieves the current branch name or HEAD state.
func GetBranch(repoPath string, gitDir string) string {
	if gitDir == "" {
		gitDir = ResolveGitDir(repoPath)
	}
	if gitDir != "" {
		headFile := filepath.Join(gitDir, "HEAD")
		if data, err := os.ReadFile(headFile); err == nil {
			head := strings.TrimSpace(string(data))
			if strings.HasPrefix(head, "ref: refs/heads/") {
				return strings.TrimPrefix(head, "ref: refs/heads/")
			}
			if len(head) >= 7 {
				return head[:7] + " (detached)"
			}
		}
	}

	// Fallback to git CLI
	cmd := exec.Command("git", "-C", repoPath, "branch", "--show-current")
	out, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" {
			return branch
		}
	}
	return "unknown"
}

// ParseRemote extracts the remote name, URL, GitHub status, slug, and web URL.
func ParseRemote(repoPath string, gitDir string) (name, rawURL string, isGitHub bool, slug, webURL string) {
	if gitDir == "" {
		gitDir = ResolveGitDir(repoPath)
	}

	// Try reading .git/config first
	if gitDir != "" {
		configFile := filepath.Join(gitDir, "config")
		if data, err := os.ReadFile(configFile); err == nil {
			name, rawURL = parseConfigRemotes(string(data))
		}
	}

	// Fallback to git CLI if not found
	if rawURL == "" {
		cmd := exec.Command("git", "-C", repoPath, "remote", "-v")
		if out, err := cmd.Output(); err == nil {
			name, rawURL = parseRemoteVOutput(string(out))
		}
	}

	if rawURL == "" {
		return "", "", false, "", ""
	}

	// Check if GitHub
	if matches := githubURLRegex.FindStringSubmatch(rawURL); len(matches) >= 3 {
		owner := matches[1]
		repo := matches[2]
		slug = owner + "/" + repo
		isGitHub = true
		webURL = "https://github.com/" + slug
	} else if strings.Contains(strings.ToLower(rawURL), "github.com") {
		isGitHub = true
		// Try best effort slug
		clean := strings.TrimSuffix(rawURL, ".git")
		parts := strings.Split(clean, "github.com")
		if len(parts) > 1 {
			sub := strings.TrimPrefix(parts[1], "/")
			sub = strings.TrimPrefix(sub, ":")
			slug = sub
			webURL = "https://github.com/" + sub
		}
	}

	return name, rawURL, isGitHub, slug, webURL
}

func parseConfigRemotes(content string) (string, string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	currentRemote := ""
	originURL := ""
	firstRemote := ""
	firstURL := ""

	remoteHeaderRegex := regexp.MustCompile(`^\[remote\s+"([^"]+)"\]`)
	urlRegex := regexp.MustCompile(`^\s*url\s*=\s*(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := remoteHeaderRegex.FindStringSubmatch(line); len(m) > 1 {
			currentRemote = m[1]
			continue
		}
		if currentRemote != "" {
			if m := urlRegex.FindStringSubmatch(line); len(m) > 1 {
				u := strings.TrimSpace(m[1])
				if currentRemote == "origin" {
					return "origin", u
				}
				if firstRemote == "" {
					firstRemote = currentRemote
					firstURL = u
				}
			}
		}
	}
	if originURL != "" {
		return "origin", originURL
	}
	return firstRemote, firstURL
}

func parseRemoteVOutput(out string) (string, string) {
	lines := strings.Split(out, "\n")
	firstRemote := ""
	firstURL := ""

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			remoteName := fields[0]
			url := fields[1]
			if remoteName == "origin" {
				return "origin", url
			}
			if firstRemote == "" {
				firstRemote = remoteName
				firstURL = url
			}
		}
	}
	return firstRemote, firstURL
}

// GetStatus checks git status --porcelain for uncommitted changes.
func GetStatus(repoPath string) (isDirty bool, count int, details []string) {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain=v1")
	out, err := cmd.Output()
	if err != nil {
		return false, 0, nil
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return false, 0, nil
	}

	lines := strings.Split(trimmed, "\n")
	details = make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			details = append(details, l)
		}
	}

	return len(details) > 0, len(details), details
}

// GetLastCommit returns details about the most recent commit.
func GetLastCommit(repoPath string) *models.CommitInfo {
	cmd := exec.Command("git", "-C", repoPath, "log", "-1", "--format=%h\t%an\t%at\t%s")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) < 4 {
		return nil
	}

	sec, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil
	}
	t := time.Unix(sec, 0)

	return &models.CommitInfo{
		Hash:     parts[0],
		Author:   parts[1],
		Date:     t,
		Relative: models.RelativeTime(t),
		Message:  parts[3],
	}
}

// GetCommitCount returns the total number of commits on HEAD.
func GetCommitCount(repoPath string) int {
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return count
}

// GetDateCreated returns the repository creation time by finding the root commit,
// falling back to the .git directory modification time.
func GetDateCreated(repoPath string) time.Time {
	cmd := exec.Command("git", "-C", repoPath, "log", "--max-parents=0", "--format=%at", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var minSec int64
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			sec, err := strconv.ParseInt(l, 10, 64)
			if err == nil {
				if minSec == 0 || sec < minSec {
					minSec = sec
				}
			}
		}
		if minSec > 0 {
			return time.Unix(minSec, 0)
		}
	}

	gitDir := filepath.Join(repoPath, ".git")
	if fi, err := os.Stat(gitDir); err == nil {
		return fi.ModTime()
	}
	if fi, err := os.Stat(repoPath); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// GetLastModified returns the time of the latest modification.
// If uncommitted changes exist, it checks the modification times of those files.
// Otherwise it falls back to the last commit time, or .git directory mod time.
func GetLastModified(repoPath string, lastCommit *models.CommitInfo, hasUncommitted bool, uncommittedDetails []string) time.Time {
	var latest time.Time
	if lastCommit != nil {
		latest = lastCommit.Date
	}

	if hasUncommitted {
		for _, detail := range uncommittedDetails {
			parts := strings.Fields(detail)
			if len(parts) >= 2 {
				filePath := filepath.Join(repoPath, parts[len(parts)-1])
				if fi, err := os.Stat(filePath); err == nil {
					if fi.ModTime().After(latest) {
						latest = fi.ModTime()
					}
				}
			}
		}
	}

	if latest.IsZero() {
		gitDir := filepath.Join(repoPath, ".git")
		if fi, err := os.Stat(gitDir); err == nil {
			latest = fi.ModTime()
		} else if fi, err := os.Stat(repoPath); err == nil {
			latest = fi.ModTime()
		}
	}

	return latest
}

// EnrichRepo fills in Git metadata for a repo.
func EnrichRepo(repo *models.Repo) {
	gitDir := ResolveGitDir(repo.Path)
	repo.Branch = GetBranch(repo.Path, gitDir)
	remoteName, remoteURL, isGitHub, slug, webURL := ParseRemote(repo.Path, gitDir)
	repo.RemoteName = remoteName
	repo.RemoteURL = remoteURL
	repo.IsGitHub = isGitHub
	repo.GitHubSlug = slug
	repo.GitHubWebURL = webURL

	if !isGitHub && remoteURL == "" {
		repo.GitHubVisibility = models.VisibilityLocal
	} else if !isGitHub {
		repo.GitHubVisibility = models.VisibilityUnknown
	}

	isDirty, count, details := GetStatus(repo.Path)
	repo.HasUncommitted = isDirty
	repo.UncommittedCount = count
	repo.UncommittedDetails = details

	repo.LastCommit = GetLastCommit(repo.Path)
	repo.CommitCount = GetCommitCount(repo.Path)
	repo.DateCreated = GetDateCreated(repo.Path)
	repo.LastModified = GetLastModified(repo.Path, repo.LastCommit, repo.HasUncommitted, repo.UncommittedDetails)
	if !repo.LastModified.IsZero() && repo.DateCreated.After(repo.LastModified) {
		repo.DateCreated = repo.LastModified
	}
}

// FormatCommitSummary produces a short string summary of last commit.
func FormatCommitSummary(c *models.CommitInfo) string {
	if c == nil {
		return "No commits yet"
	}
	var b bytes.Buffer
	b.WriteString(c.Hash)
	b.WriteString(" (")
	b.WriteString(c.Relative)
	b.WriteString(" by ")
	b.WriteString(c.Author)
	b.WriteString("): ")
	b.WriteString(c.Message)
	return b.String()
}
