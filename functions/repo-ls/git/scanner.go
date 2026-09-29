package git

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dustinmichels/repo-ls/models"
)

var ignoredDirNames = map[string]bool{
	"node_modules": true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".cache":       true,
	".next":        true,
	".venv":        true,
	"venv":         true,
	"env":          true,
	"__pycache__":  true,
	"Pods":         true,
	"DerivedData":  true,
	".cargo":       true,
	".rustup":      true,
	".npm":         true,
	".yarn":        true,
	".idea":        true,
	".vscode":      true,
	".git":         true,
}

// ScanRepositories recursively finds all git repositories under rootDir.
func ScanRepositories(rootDir string, maxDepth int) ([]*models.Repo, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	var repoPaths []string
	var walk func(current string, depth int)

	walk = func(current string, depth int) {
		if depth > maxDepth {
			return
		}

		entries, err := os.ReadDir(current)
		if err != nil {
			return
		}

		isRepo := false
		for _, e := range entries {
			if e.Name() == ".git" {
				isRepo = true
				break
			}
		}

		if isRepo {
			repoPaths = append(repoPaths, current)
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if ignoredDirNames[name] {
				continue
			}
			// Skip hidden directories (starting with '.') unless it is the root scan dir
			if strings.HasPrefix(name, ".") {
				continue
			}

			subPath := filepath.Join(current, name)
			walk(subPath, depth+1)
		}
	}

	walk(absRoot, 0)

	// Create repo objects
	repos := make([]*models.Repo, len(repoPaths))
	for i, p := range repoPaths {
		rel, err := filepath.Rel(absRoot, p)
		if err != nil || rel == "." {
			rel = filepath.Base(p)
		}
		repos[i] = &models.Repo{
			Path:             p,
			RelPath:          rel,
			Name:             filepath.Base(p),
			GitHubVisibility: models.VisibilityUnknown,
		}
	}

	// Concurrently enrich Git metadata
	EnrichAllRepos(repos)

	return repos, nil
}

// EnrichAllRepos populates git metadata for a list of repos in parallel.
func EnrichAllRepos(repos []*models.Repo) {
	numWorkers := runtime.NumCPU() * 2
	if numWorkers < 4 {
		numWorkers = 4
	}
	if numWorkers > 32 {
		numWorkers = 32
	}

	ch := make(chan *models.Repo, len(repos))
	for _, r := range repos {
		ch <- r
	}
	close(ch)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range ch {
				EnrichRepo(r)
			}
		}()
	}
	wg.Wait()
}
