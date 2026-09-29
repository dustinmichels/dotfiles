package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

var (
	commitCacheMu sync.RWMutex
	commitCache   = make(map[string]*CommitSnapshot)
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
		Timestamp:     time.Now(),
		IsWorkingTree: true,
		Languages:     languages,
		Total:         total,
		LinesAdded:    added,
		LinesDeleted:  deleted,
	}, nil
}

// AnalyzeCommit extracts a commit into a temp directory and runs tokei on it.
func AnalyzeCommit(repoRoot string, info CommitInfo) (*CommitSnapshot, error) {
	commitCacheMu.RLock()
	if cached, ok := commitCache[info.Hash]; ok {
		commitCacheMu.RUnlock()
		cp := *cached
		cp.RelativeDate = info.RelativeDate
		cp.Timestamp = info.Timestamp
		return &cp, nil
	}
	commitCacheMu.RUnlock()

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

	snap := &CommitSnapshot{
		Hash:          info.Hash,
		ShortHash:     info.ShortHash,
		Author:        info.Author,
		Date:          info.Date,
		RelativeDate:  info.RelativeDate,
		Subject:       info.Subject,
		Timestamp:     info.Timestamp,
		IsWorkingTree: false,
		Languages:     languages,
		Total:         total,
		LinesAdded:    added,
		LinesDeleted:  deleted,
	}

	commitCacheMu.Lock()
	commitCache[info.Hash] = snap
	commitCacheMu.Unlock()

	return snap, nil
}

// AnalyzeCommitBatch analyzes a slice of commit infos concurrently using worker goroutines.
func AnalyzeCommitBatch(repoRoot string, commitInfos []CommitInfo, progressFn func(done, total int)) ([]*CommitSnapshot, error) {
	if len(commitInfos) == 0 {
		return nil, nil
	}

	totalJobs := len(commitInfos)
	var (
		mu        sync.Mutex
		doneCount int
		snapshots = make([]*CommitSnapshot, len(commitInfos))
	)

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
					snap = &CommitSnapshot{
						Hash:         j.info.Hash,
						ShortHash:    j.info.ShortHash,
						Author:       j.info.Author,
						Date:         j.info.Date,
						RelativeDate: j.info.RelativeDate,
						Subject:      j.info.Subject,
						Timestamp:    j.info.Timestamp,
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

	var validSnapshots []*CommitSnapshot
	for _, s := range snapshots {
		if s != nil {
			validSnapshots = append(validSnapshots, s)
		}
	}
	return validSnapshots, nil
}

// CollectInitialHistory gathers the most recent commits (up to initialBatchSize) for instant UI startup,
// returning initial snapshots, remaining older commit infos for background lazy loading, and total commits.
// If limit <= 0, all commits in git history are considered.
func CollectInitialHistory(repoRoot string, limit int, initialBatchSize int, progressFn func(done, total int)) (*HistoryInitResult, error) {
	allInfos, err := GetRecentCommits(repoRoot, limit)
	if err != nil {
		return nil, err
	}

	isDirty, err := IsWorkingTreeDirty(repoRoot)
	if err != nil {
		isDirty = false
	}

	total := len(allInfos)
	if total == 0 {
		var initialSnapshots []*CommitSnapshot
		if isDirty {
			wtSnap, err := AnalyzeWorkingTree(repoRoot)
			if err == nil {
				initialSnapshots = append(initialSnapshots, wtSnap)
			}
		}
		return &HistoryInitResult{
			InitialSnapshots: initialSnapshots,
			PendingInfos:     nil,
			TotalCommits:     0,
			IsDirty:          isDirty,
		}, nil
	}

	if initialBatchSize <= 0 || total <= initialBatchSize {
		snaps, err := AnalyzeCommitBatch(repoRoot, allInfos, progressFn)
		if err != nil {
			return nil, err
		}
		if isDirty {
			wtSnap, err := AnalyzeWorkingTree(repoRoot)
			if err == nil {
				snaps = append(snaps, wtSnap)
			}
		}
		ComputeDiffs(snaps)
		return &HistoryInitResult{
			InitialSnapshots: snaps,
			PendingInfos:     nil,
			TotalCommits:     total,
			IsDirty:          isDirty,
		}, nil
	}

	// allInfos is ordered oldest-first.
	// The most recent commits are at the end: allInfos[cutoff:]
	cutoff := total - initialBatchSize
	pendingInfos := allInfos[:cutoff]
	recentInfos := allInfos[cutoff:]

	snaps, err := AnalyzeCommitBatch(repoRoot, recentInfos, progressFn)
	if err != nil {
		return nil, err
	}
	if isDirty {
		wtSnap, err := AnalyzeWorkingTree(repoRoot)
		if err == nil {
			snaps = append(snaps, wtSnap)
		}
	}
	ComputeDiffs(snaps)

	return &HistoryInitResult{
		InitialSnapshots: snaps,
		PendingInfos:     pendingInfos,
		TotalCommits:     total,
		IsDirty:          isDirty,
	}, nil
}

// CollectHistory gathers tokei snapshots across commits and uncommitted changes.
func CollectHistory(repoRoot string, limit int, progressFn func(done, total int)) ([]*CommitSnapshot, error) {
	res, err := CollectInitialHistory(repoRoot, limit, 0, progressFn)
	if err != nil {
		return nil, err
	}
	return res.InitialSnapshots, nil
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

func periodKeyAndLabel(ts time.Time, mode GroupMode) (string, string) {
	switch mode {
	case GroupDay:
		return ts.Format("2006-01-02"), ts.Format("2006-01-02")
	case GroupWeek:
		year, week := ts.ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", year, week)
		label := fmt.Sprintf("%04d W%02d", year, week)
		return key, label
	case GroupMonth:
		return ts.Format("2006-01"), ts.Format("Jan 2006")
	case GroupYear:
		return ts.Format("2006"), ts.Format("2006")
	default:
		return ts.Format("2006-01-02"), ts.Format("2006-01-02")
	}
}

// AggregateSnapshots groups a slice of commit snapshots into periods (Day, Week, Month, Year)
// and calculates the average stats for all commits within each period.
func AggregateSnapshots(snapshots []*CommitSnapshot, mode GroupMode) []*CommitSnapshot {
	if mode == GroupCommit || len(snapshots) == 0 {
		return snapshots
	}

	type periodGroup struct {
		key     string
		label   string
		commits []*CommitSnapshot
		minTime time.Time
		maxTime time.Time
		hasWT   bool
	}

	var groups []*periodGroup
	groupMap := make(map[string]*periodGroup)

	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		ts := snap.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		key, label := periodKeyAndLabel(ts, mode)
		grp, exists := groupMap[key]
		if !exists {
			grp = &periodGroup{
				key:     key,
				label:   label,
				minTime: ts,
				maxTime: ts,
			}
			groupMap[key] = grp
			groups = append(groups, grp)
		}
		grp.commits = append(grp.commits, snap)
		if snap.IsWorkingTree {
			grp.hasWT = true
		}
		if ts.Before(grp.minTime) {
			grp.minTime = ts
		}
		if ts.After(grp.maxTime) {
			grp.maxTime = ts
		}
	}

	var summaries []*CommitSnapshot
	for _, grp := range groups {
		n := len(grp.commits)
		if n == 0 {
			continue
		}

		type accum struct {
			code     float64
			lines    float64
			files    float64
			comments float64
			blanks   float64
		}
		langMap := make(map[string]*accum)
		var totalAdded, totalDeleted int
		var subCommits []string

		for _, c := range grp.commits {
			totalAdded += c.LinesAdded
			totalDeleted += c.LinesDeleted

			var desc string
			if c.IsWorkingTree {
				desc = "Working Tree (Uncommitted)"
			} else {
				subj := c.Subject
				if len(subj) > 28 {
					subj = subj[:27] + "…"
				}
				desc = fmt.Sprintf("%s (%s)", c.ShortHash, subj)
			}
			subCommits = append(subCommits, desc)

			for lang, s := range c.Languages {
				ac, ok := langMap[lang]
				if !ok {
					ac = &accum{}
					langMap[lang] = ac
				}
				ac.code += float64(s.Code)
				ac.lines += float64(s.Lines)
				ac.files += float64(s.Files)
				ac.comments += float64(s.Comments)
				ac.blanks += float64(s.Blanks)
			}
		}

		// Compute average stats per language
		avgLanguages := make(map[string]LanguageStats)
		avgTotal := LanguageStats{Name: "Total"}

		for lang, ac := range langMap {
			s := LanguageStats{
				Name:     lang,
				Code:     int(math.Round(ac.code / float64(n))),
				Lines:    int(math.Round(ac.lines / float64(n))),
				Files:    int(math.Round(ac.files / float64(n))),
				Comments: int(math.Round(ac.comments / float64(n))),
				Blanks:   int(math.Round(ac.blanks / float64(n))),
			}
			if s.Code > 0 || s.Lines > 0 || s.Files > 0 {
				avgLanguages[lang] = s
				avgTotal.Code += s.Code
				avgTotal.Lines += s.Lines
				avgTotal.Files += s.Files
				avgTotal.Comments += s.Comments
				avgTotal.Blanks += s.Blanks
			}
		}

		var relativeSpan string
		if n == 1 {
			relativeSpan = grp.commits[0].RelativeDate
		} else {
			relativeSpan = fmt.Sprintf("%d commits", n)
		}

		subject := fmt.Sprintf("Average of %d commit(s) in %s", n, grp.label)
		if grp.hasWT {
			subject += " (incl. working tree)"
		}

		summarySnap := &CommitSnapshot{
			Hash:          fmt.Sprintf("summary-%s", grp.key),
			ShortHash:     grp.key,
			Author:        fmt.Sprintf("%d commits", n),
			Date:          grp.label,
			RelativeDate:  relativeSpan,
			Subject:       subject,
			Timestamp:     grp.maxTime,
			IsWorkingTree: grp.hasWT,
			IsSummary:     true,
			CommitCount:   n,
			PeriodLabel:   grp.label,
			PeriodKey:     grp.key,
			SubCommits:    subCommits,
			Languages:     avgLanguages,
			Total:         avgTotal,
			LinesAdded:    int(math.Round(float64(totalAdded) / float64(n))),
			LinesDeleted:  int(math.Round(float64(totalDeleted) / float64(n))),
		}
		summaries = append(summaries, summarySnap)
	}

	ComputeDiffs(summaries)
	return summaries
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
