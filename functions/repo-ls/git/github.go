package git

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"
	"github.com/dustinmichels/repo-ls/models"
)

type ghRepoEntry struct {
	NameWithOwner string `json:"nameWithOwner"`
	Visibility    string `json:"visibility"`
	IsPrivate     bool   `json:"isPrivate"`
}

// FetchAllUserReposViaGH calls gh repo list to fetch all repos belonging to the user.
func FetchAllUserReposViaGH() (map[string]models.Visibility, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "repo", "list", "--limit", "1000", "--json", "nameWithOwner,visibility,isPrivate")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var entries []ghRepoEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, err
	}

	res := make(map[string]models.Visibility, len(entries))
	for _, e := range entries {
		slug := strings.ToLower(strings.TrimSpace(e.NameWithOwner))
		vis := models.VisibilityPublic
		if e.IsPrivate || strings.ToUpper(e.Visibility) == "PRIVATE" {
			vis = models.VisibilityPrivate
		}
		res[slug] = vis
	}

	return res, nil
}

// FetchRepoVisibilityViaGH fetches visibility for a single repository slug using gh CLI.
func FetchRepoVisibilityViaGH(slug string) (models.Visibility, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return models.VisibilityUnknown, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "repo", "view", slug, "--json", "visibility,isPrivate")
	out, err := cmd.Output()
	if err != nil {
		return models.VisibilityUnknown, err
	}

	var entry ghRepoEntry
	if err := json.Unmarshal(out, &entry); err != nil {
		return models.VisibilityUnknown, err
	}

	if entry.IsPrivate || strings.ToUpper(entry.Visibility) == "PRIVATE" {
		return models.VisibilityPrivate, nil
	}
	return models.VisibilityPublic, nil
}

// EnrichGitHubVisibility enriches the repositories with their GitHub visibility.
func EnrichGitHubVisibility(repos []*models.Repo) {
	ghMap, _ := FetchAllUserReposViaGH()
	if ghMap == nil {
		ghMap = make(map[string]models.Visibility)
	}

	var missingSlugs []*models.Repo
	for _, r := range repos {
		if !r.IsGitHub {
			if r.RemoteURL == "" {
				r.GitHubVisibility = models.VisibilityLocal
			} else {
				r.GitHubVisibility = models.VisibilityUnknown
			}
			continue
		}

		cleanSlug := strings.ToLower(strings.TrimSpace(r.GitHubSlug))
		if vis, ok := ghMap[cleanSlug]; ok {
			r.GitHubVisibility = vis
		} else {
			missingSlugs = append(missingSlugs, r)
		}
	}

	// For missing slugs (e.g. forks, org repos, external repos), query individually in parallel
	if len(missingSlugs) > 0 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for _, r := range missingSlugs {
			wg.Add(1)
			go func(repo *models.Repo) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				vis, err := FetchRepoVisibilityViaGH(repo.GitHubSlug)
				if err == nil && vis != models.VisibilityUnknown {
					repo.GitHubVisibility = vis
				} else {
					repo.GitHubVisibility = models.VisibilityUnknown
				}
			}(r)
		}
		wg.Wait()
	}
}
