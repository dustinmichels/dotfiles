package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteLocalRepoSafety(t *testing.T) {
	// 1. Refuse deleting non-git directory
	tmpDir, err := os.MkdirTemp("", "test-del-nongit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	err = DeleteLocalRepo(tmpDir)
	if err == nil {
		t.Errorf("expected error deleting non-git directory, got nil")
	}

	// 2. Refuse deleting root or home
	home, _ := os.UserHomeDir()
	if err := DeleteLocalRepo(home); err == nil {
		t.Errorf("expected error refusing to delete home directory")
	}
	if err := DeleteLocalRepo("/"); err == nil {
		t.Errorf("expected error refusing to delete root directory")
	}
}

func TestDeleteLocalRepoSuccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-del-repo-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create fake git repo
	repoDir := filepath.Join(tmpDir, "sample-repo")
	_ = os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0644)

	err = DeleteLocalRepo(repoDir)
	if err != nil {
		t.Fatalf("unexpected error deleting repo: %v", err)
	}

	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Errorf("expected repo directory to be removed from disk")
	}
}

func TestDeleteGitHubRepoValidation(t *testing.T) {
	// Invalid slug
	err := DeleteGitHubRepo("invalid-slug-no-slash")
	if err == nil {
		t.Errorf("expected error for invalid slug without slash")
	}

	err = DeleteGitHubRepo("")
	if err == nil {
		t.Errorf("expected error for empty slug")
	}
}
