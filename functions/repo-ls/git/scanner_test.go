package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRepositories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-scanner-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create valid repo 1: /repo1/.git
	repo1 := filepath.Join(tmpDir, "repo1")
	_ = os.MkdirAll(filepath.Join(repo1, ".git"), 0755)

	// Create valid nested repo 2: /folder/sub/repo2/.git
	repo2 := filepath.Join(tmpDir, "folder", "sub", "repo2")
	_ = os.MkdirAll(filepath.Join(repo2, ".git"), 0755)

	// Create repo inside node_modules (should be ignored)
	ignoredNode := filepath.Join(tmpDir, "repo1", "node_modules", "nested-pkg")
	_ = os.MkdirAll(filepath.Join(ignoredNode, ".git"), 0755)

	// Create repo inside target (should be ignored)
	ignoredTarget := filepath.Join(tmpDir, "repo1", "target", "build-repo")
	_ = os.MkdirAll(filepath.Join(ignoredTarget, ".git"), 0755)

	// Create repo inside hidden directory (should be ignored)
	ignoredHidden := filepath.Join(tmpDir, ".hidden", "hidden-repo")
	_ = os.MkdirAll(filepath.Join(ignoredHidden, ".git"), 0755)

	// Scan
	repos, err := ScanRepositories(tmpDir, 5)
	if err != nil {
		t.Fatalf("unexpected error scanning: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(repos))
	}

	foundPaths := make(map[string]bool)
	for _, r := range repos {
		foundPaths[r.Path] = true
	}

	if !foundPaths[repo1] {
		t.Errorf("expected to find repo1 at %s", repo1)
	}
	if !foundPaths[repo2] {
		t.Errorf("expected to find repo2 at %s", repo2)
	}
	if foundPaths[ignoredNode] {
		t.Errorf("expected node_modules repo to be ignored")
	}
	if foundPaths[ignoredTarget] {
		t.Errorf("expected target repo to be ignored")
	}
	if foundPaths[ignoredHidden] {
		t.Errorf("expected hidden repo to be ignored")
	}
}
