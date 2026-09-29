package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// CheckTokeiInstalled verifies that tokei is available in PATH.
func CheckTokeiInstalled() error {
	_, err := exec.LookPath("tokei")
	if err != nil {
		return fmt.Errorf("tokei is not installed or not in PATH.\nInstall with: brew install tokei  or  cargo install tokei")
	}
	return nil
}

type tokeiReportItem struct {
	Stats struct {
		Blanks   int `json:"blanks"`
		Code     int `json:"code"`
		Comments int `json:"comments"`
	} `json:"stats"`
	Name string `json:"name"`
}

type tokeiLangEntry struct {
	Blanks     int               `json:"blanks"`
	Code       int               `json:"code"`
	Comments   int               `json:"comments"`
	Reports    []tokeiReportItem `json:"reports"`
	Inaccurate bool              `json:"inaccurate"`
}

// RunTokei executes tokei -o json on the specified directory and parses the results.
func RunTokei(dir string) (map[string]LanguageStats, LanguageStats, error) {
	cmd := exec.Command("tokei", "-o", "json", dir)
	out, err := cmd.Output()
	if err != nil {
		return nil, LanguageStats{}, fmt.Errorf("tokei failed: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, LanguageStats{}, fmt.Errorf("failed to parse tokei output: %w", err)
	}

	languages := make(map[string]LanguageStats)
	total := LanguageStats{Name: "Total"}

	for langName, rawMsg := range raw {
		if langName == "Total" {
			continue
		}

		var entry tokeiLangEntry
		if err := json.Unmarshal(rawMsg, &entry); err != nil {
			continue
		}

		stats := LanguageStats{
			Name:     langName,
			Files:    len(entry.Reports),
			Code:     entry.Code,
			Comments: entry.Comments,
			Blanks:   entry.Blanks,
			Lines:    entry.Code + entry.Comments + entry.Blanks,
		}

		languages[langName] = stats
		total.Files += stats.Files
		total.Code += stats.Code
		total.Comments += stats.Comments
		total.Blanks += stats.Blanks
		total.Lines += stats.Lines
	}

	return languages, total, nil
}

// AnalyzeWorkingTree runs tokei on the current uncommitted state in repoRoot.
func AnalyzeWorkingTree(repoRoot string) (*CommitSnapshot, error) {
	languages, total, err := RunTokei(repoRoot)
	if err != nil {
		return nil, err
	}

	added, deleted, _ := GetWorkingTreeDiffNumstat(repoRoot)

	return &CommitSnapshot{
		Hash:          "working-tree",
		ShortHash:     "Working Tree",
		Author:        "You",
		Date:          "now",
		RelativeDate:  "uncommitted",
		Subject:       "Uncommitted changes in working tree",
		IsWorkingTree: true,
		Languages:     languages,
		Total:         total,
		LinesAdded:    added,
		LinesDeleted:  deleted,
	}, nil
}

// AnalyzeCommit extracts a commit into a temp directory and runs tokei on it.
func AnalyzeCommit(repoRoot string, info CommitInfo) (*CommitSnapshot, error) {
	tmpDir, err := ExtractCommitToTempDir(repoRoot, info.Hash)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	languages, total, err := RunTokei(tmpDir)
	if err != nil {
		return nil, err
	}

	added, deleted, _ := GetCommitDiffNumstat(repoRoot, info.Hash)

	return &CommitSnapshot{
		Hash:          info.Hash,
		ShortHash:     info.ShortHash,
		Author:        info.Author,
		Date:          info.Date,
		RelativeDate:  info.RelativeDate,
		Subject:       info.Subject,
		IsWorkingTree: false,
		Languages:     languages,
		Total:         total,
		LinesAdded:    added,
		LinesDeleted:  deleted,
	}, nil
}

// CollectHistory gathers tokei snapshots across recent commits and uncommitted changes.
func CollectHistory(repoRoot string, limit int, progressFn func(done, total int)) ([]*CommitSnapshot, error) {
	commitInfos, err := GetRecentCommits(repoRoot, limit)
	if err != nil {
		return nil, err
	}

	isDirty, err := IsWorkingTreeDirty(repoRoot)
	if err != nil {
		isDirty = false
	}

	totalJobs := len(commitInfos)
	if isDirty {
		totalJobs++
	}

	var (
		mu        sync.Mutex
		doneCount int
		snapshots = make([]*CommitSnapshot, len(commitInfos))
	)

	// Determine worker concurrency
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	if numWorkers < 2 {
		numWorkers = 2
	}

	type job struct {
		index int
		info  CommitInfo
	}

	jobs := make(chan job, len(commitInfos))
	for i, info := range commitInfos {
		jobs <- job{index: i, info: info}
	}
	close(jobs)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				snap, err := AnalyzeCommit(repoRoot, j.info)
				if err != nil {
					// Fallback snapshot with empty stats on error
					snap = &CommitSnapshot{
						Hash:         j.info.Hash,
						ShortHash:    j.info.ShortHash,
						Author:       j.info.Author,
						Date:         j.info.Date,
						RelativeDate: j.info.RelativeDate,
						Subject:      j.info.Subject,
						Languages:    make(map[string]LanguageStats),
					}
				}
				mu.Lock()
				snapshots[j.index] = snap
				doneCount++
				if progressFn != nil {
					progressFn(doneCount, totalJobs)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// If dirty, analyze working tree as the latest entry
	if isDirty {
		wtSnap, err := AnalyzeWorkingTree(repoRoot)
		if err == nil {
			snapshots = append(snapshots, wtSnap)
		}
		doneCount++
		if progressFn != nil {
			progressFn(doneCount, totalJobs)
		}
	}

	// Filter out any nil snapshots
	var validSnapshots []*CommitSnapshot
	for _, s := range snapshots {
		if s != nil {
			validSnapshots = append(validSnapshots, s)
		}
	}

	ComputeDiffs(validSnapshots)
	return validSnapshots, nil
}

// ComputeDiffs calculates differences between adjacent commits and vs the latest commit.
func ComputeDiffs(snapshots []*CommitSnapshot) {
	n := len(snapshots)
	if n == 0 {
		return
	}

	latest := snapshots[n-1]

	for i, curr := range snapshots {
		// Diffs vs previous commit
		curr.DiffPrev = make(map[string]LanguageStats)
		if i > 0 {
			prev := snapshots[i-1]
			curr.TotalDiffPrev = subtractStats(curr.Total, prev.Total)

			// Collect all languages from curr and prev
			allLangs := make(map[string]bool)
			for l := range curr.Languages {
				allLangs[l] = true
			}
			for l := range prev.Languages {
				allLangs[l] = true
			}
			for l := range allLangs {
				currL := curr.Languages[l]
				prevL := prev.Languages[l]
				curr.DiffPrev[l] = subtractStats(currL, prevL)
			}
		}

		// Diffs vs latest commit
		curr.DiffLatest = make(map[string]LanguageStats)
		curr.TotalDiffLatest = subtractStats(curr.Total, latest.Total)

		allLangs := make(map[string]bool)
		for l := range curr.Languages {
			allLangs[l] = true
		}
		for l := range latest.Languages {
			allLangs[l] = true
		}
		for l := range allLangs {
			currL := curr.Languages[l]
			latestL := latest.Languages[l]
			curr.DiffLatest[l] = subtractStats(currL, latestL)
		}
	}
}

func subtractStats(a, b LanguageStats) LanguageStats {
	return LanguageStats{
		Name:     a.Name,
		Files:    a.Files - b.Files,
		Lines:    a.Lines - b.Lines,
		Code:     a.Code - b.Code,
		Comments: a.Comments - b.Comments,
		Blanks:   a.Blanks - b.Blanks,
	}
}
