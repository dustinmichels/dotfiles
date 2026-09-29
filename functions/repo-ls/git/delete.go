package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustinmichels/repo-ls/models"
)

// DeleteLocalRepo removes the repository directory from disk with safety checks.
func DeleteLocalRepo(repoPath string) error {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Safety guards: do not allow deleting root, home directory, or empty path
	home, _ := os.UserHomeDir()
	if absPath == "/" || absPath == home || absPath == filepath.Dir(home) {
		return fmt.Errorf("refusing to delete protected system/home path: %s", absPath)
	}

	// Verify that the path contains .git
	gitPath := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		return fmt.Errorf("refusing to delete non-git directory: %s", absPath)
	}

	return os.RemoveAll(absPath)
}

// DeleteGitHubRepo deletes the repository from GitHub using gh CLI.
func DeleteGitHubRepo(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" || !strings.Contains(slug, "/") {
		return fmt.Errorf("invalid GitHub repository slug: %q", slug)
	}

	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found in PATH")
	}

	cmd := exec.Command("gh", "repo", "delete", slug, "--yes")
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "delete_repo") || strings.Contains(outStr, "scope") {
			return fmt.Errorf("missing 'delete_repo' scope. Run 'gh auth refresh -s delete_repo' in terminal (%s)", outStr)
		}
		if outStr != "" {
			return fmt.Errorf("gh repo delete failed: %s", outStr)
		}
		return fmt.Errorf("gh repo delete failed: %w", err)
	}

	return nil
}

// DeleteRepo coordinates local and/or GitHub deletion for a repo.
func DeleteRepo(repo *models.Repo, deleteLocal bool, deleteGitHub bool) (localErr error, githubErr error) {
	if deleteGitHub && repo.IsGitHub && repo.GitHubSlug != "" {
		githubErr = DeleteGitHubRepo(repo.GitHubSlug)
	}

	if deleteLocal {
		localErr = DeleteLocalRepo(repo.Path)
	}

	return localErr, githubErr
}
