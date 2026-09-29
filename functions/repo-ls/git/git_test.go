package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"github.com/dustinmichels/repo-ls/models"
)

func TestParseConfigRemotes(t *testing.T) {
	configContent := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
	ignorecase = true
	precomposeunicode = true
[remote "origin"]
	url = git@github.com:dustinmichels/dotfiles.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
	merge = refs/heads/main
`
	name, url := parseConfigRemotes(configContent)
	if name != "origin" {
		t.Errorf("expected remote name 'origin', got %q", name)
	}
	if url != "git@github.com:dustinmichels/dotfiles.git" {
		t.Errorf("expected url 'git@github.com:dustinmichels/dotfiles.git', got %q", url)
	}
}

func TestParseConfigRemotesMultiple(t *testing.T) {
	configContent := `[remote "upstream"]
	url = https://github.com/upstream/repo.git
	fetch = +refs/heads/*:refs/remotes/upstream/*
[remote "origin"]
	url = https://github.com/dustinmichels/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`
	name, url := parseConfigRemotes(configContent)
	// Origin should be preferred if present
	if name != "origin" {
		t.Errorf("expected remote name 'origin', got %q", name)
	}
	if url != "https://github.com/dustinmichels/repo.git" {
		t.Errorf("expected url 'https://github.com/dustinmichels/repo.git', got %q", url)
	}
}

func TestParseRemoteURLs(t *testing.T) {
	tests := []struct {
		url          string
		wantIsGitHub bool
		wantSlug     string
		wantWebURL   string
	}{
		{
			url:          "git@github.com:dustinmichels/repo-ls.git",
			wantIsGitHub: true,
			wantSlug:     "dustinmichels/repo-ls",
			wantWebURL:   "https://github.com/dustinmichels/repo-ls",
		},
		{
			url:          "https://github.com/dustinmichels/agent-ls",
			wantIsGitHub: true,
			wantSlug:     "dustinmichels/agent-ls",
			wantWebURL:   "https://github.com/dustinmichels/agent-ls",
		},
		{
			url:          "https://gitlab.com/someone/myproject.git",
			wantIsGitHub: false,
			wantSlug:     "",
			wantWebURL:   "",
		},
		{
			url:          "",
			wantIsGitHub: false,
			wantSlug:     "",
			wantWebURL:   "",
		},
	}

	for _, tt := range tests {
		tmpDir, err := os.MkdirTemp("", "test-git-remote-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatal(err)
		}

		if tt.url != "" {
			cfg := "[remote \"origin\"]\n\turl = " + tt.url + "\n"
			if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0644); err != nil {
				t.Fatal(err)
			}
		}

		_, _, isGH, slug, webURL := ParseRemote(tmpDir, gitDir)
		if isGH != tt.wantIsGitHub {
			t.Errorf("url %q: expected isGitHub=%v, got %v", tt.url, tt.wantIsGitHub, isGH)
		}
		if slug != tt.wantSlug {
			t.Errorf("url %q: expected slug=%q, got %q", tt.url, tt.wantSlug, slug)
		}
		if webURL != tt.wantWebURL {
			t.Errorf("url %q: expected webURL=%q, got %q", tt.url, tt.wantWebURL, webURL)
		}
	}
}

func TestResolveGitDir(t *testing.T) {
	// 1. Regular directory
	tmpDir, err := os.MkdirTemp("", "test-resolve-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	gitDir := filepath.Join(tmpDir, ".git")
	_ = os.MkdirAll(gitDir, 0755)

	resolved := ResolveGitDir(tmpDir)
	if resolved != gitDir {
		t.Errorf("expected %q, got %q", gitDir, resolved)
	}

	// 2. Submodule or worktree file
	worktreeDir, err := os.MkdirTemp("", "test-worktree-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(worktreeDir)

	realGitDir := filepath.Join(tmpDir, ".git", "modules", "sub")
	_ = os.MkdirAll(realGitDir, 0755)

	gitFile := filepath.Join(worktreeDir, ".git")
	_ = os.WriteFile(gitFile, []byte("gitdir: "+realGitDir+"\n"), 0644)

	resolvedWorktree := ResolveGitDir(worktreeDir)
	if resolvedWorktree != realGitDir {
		t.Errorf("expected %q, got %q", realGitDir, resolvedWorktree)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	if got := models.RelativeTime(time.Time{}); got != "never" {
		t.Errorf("zero time expected 'never', got %q", got)
	}

	if got := models.RelativeTime(now.Add(-10 * time.Second)); got != "just now" {
		t.Errorf("10s ago expected 'just now', got %q", got)
	}

	if got := models.RelativeTime(now.Add(-5 * time.Minute)); got != "5m ago" {
		t.Errorf("5m ago expected '5m ago', got %q", got)
	}

	if got := models.RelativeTime(now.Add(-3 * time.Hour)); got != "3h ago" {
		t.Errorf("3h ago expected '3h ago', got %q", got)
	}

	if got := models.RelativeTime(now.Add(-48 * time.Hour)); got != "2d ago" {
		t.Errorf("48h ago expected '2d ago', got %q", got)
	}

	if got := models.RelativeTime(now.Add(-70 * 24 * time.Hour)); got != "2mo ago" {
		t.Errorf("70d ago expected '2mo ago', got %q", got)
	}
}
func TestGetDateCreatedAndModified(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-git-dates-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Empty repo (no commits) should fallback to .git directory mod time
	run := func(env []string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\noutput: %s", args, err, string(out))
		}
	}

	run(nil, "init")
	run(nil, "config", "user.name", "Tester")
	run(nil, "config", "user.email", "tester@example.com")

	emptyCreated := GetDateCreated(tmpDir)
	if emptyCreated.IsZero() {
		t.Errorf("expected non-zero creation time for empty repo")
	}

	// 2. Commit 1 in 2020
	filePath := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(filePath, []byte("version 1"), 0644); err != nil {
		t.Fatal(err)
	}
	run(nil, "add", "file1.txt")
	run([]string{
		"GIT_AUTHOR_DATE=2020-06-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-06-01T00:00:00Z",
	}, "commit", "-m", "first commit in 2020")

	// Commit 2 in 2021
	if err := os.WriteFile(filePath, []byte("version 2"), 0644); err != nil {
		t.Fatal(err)
	}
	run(nil, "add", "file1.txt")
	run([]string{
		"GIT_AUTHOR_DATE=2021-06-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2021-06-01T00:00:00Z",
	}, "commit", "-m", "second commit in 2021")

	created := GetDateCreated(tmpDir)
	if created.UTC().Year() != 2020 {
		t.Errorf("expected creation year 2020, got %d (date: %v)", created.UTC().Year(), created)
	}

	lastCommit := GetLastCommit(tmpDir)
	if lastCommit == nil {
		t.Fatalf("expected last commit to be found")
	}
	if lastCommit.Date.Year() != 2021 {
		t.Errorf("expected last commit year 2021, got %d", lastCommit.Date.Year())
	}

	// Clean repo modified time should match last commit time (2021)
	cleanMod := GetLastModified(tmpDir, lastCommit, false, nil)
	if cleanMod.Year() != 2021 {
		t.Errorf("expected clean mod time year 2021, got %d", cleanMod.Year())
	}

	// 3. Add uncommitted change (now)
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filePath, []byte("uncommitted change"), 0644); err != nil {
		t.Fatal(err)
	}
	isDirty, _, details := GetStatus(tmpDir)
	if !isDirty {
		t.Fatalf("expected repo to be dirty")
	}

	dirtyMod := GetLastModified(tmpDir, lastCommit, isDirty, details)
	if !dirtyMod.After(lastCommit.Date) {
		t.Errorf("expected dirty mod time %v to be after last commit date %v", dirtyMod, lastCommit.Date)
	}
	if time.Since(dirtyMod) > 5*time.Second {
		t.Errorf("expected dirty mod time to be recent, got %v (since: %v)", dirtyMod, time.Since(dirtyMod))
	}
}
