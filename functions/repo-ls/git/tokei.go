package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"

	"github.com/dustinmichels/repo-ls/models"
)

type tokeiRawLang struct {
	Blanks   int           `json:"blanks"`
	Code     int           `json:"code"`
	Comments int           `json:"comments"`
	Reports  []interface{} `json:"reports"`
}

// IsTokeiAvailable checks if tokei executable exists in PATH.
func IsTokeiAvailable() bool {
	_, err := exec.LookPath("tokei")
	return err == nil
}

// RunTokei executes tokei -o json on the given directories and returns parsed statistics.
func RunTokei(repoPaths ...string) (*models.TokeiStats, error) {
	if !IsTokeiAvailable() {
		return &models.TokeiStats{
			Loaded: true,
			Err:    "tokei command not found in PATH",
		}, fmt.Errorf("tokei not found in PATH")
	}
	if len(repoPaths) == 0 {
		return &models.TokeiStats{
			Loaded: true,
		}, nil
	}

	args := append([]string{"-o", "json"}, repoPaths...)
	cmd := exec.Command("tokei", args...)
	out, err := cmd.Output()
	if err != nil {
		return &models.TokeiStats{
			Loaded: true,
			Err:    fmt.Sprintf("tokei failed: %v", err),
		}, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return &models.TokeiStats{
			Loaded: true,
			Err:    "failed to parse tokei JSON output",
		}, err
	}

	var languages []models.LanguageStats
	total := models.LanguageStats{Name: "Total"}

	for langName, rawVal := range raw {
		if langName == "Total" {
			continue
		}

		var item tokeiRawLang
		if err := json.Unmarshal(rawVal, &item); err != nil {
			continue
		}

		// Skip languages with 0 lines
		lineCount := item.Code + item.Comments + item.Blanks
		if lineCount == 0 && len(item.Reports) == 0 {
			continue
		}

		stat := models.LanguageStats{
			Name:     langName,
			Files:    len(item.Reports),
			Code:     item.Code,
			Comments: item.Comments,
			Blanks:   item.Blanks,
			Lines:    lineCount,
		}

		languages = append(languages, stat)
		total.Files += stat.Files
		total.Code += stat.Code
		total.Comments += stat.Comments
		total.Blanks += stat.Blanks
		total.Lines += stat.Lines
	}

	// Sort languages by Code count descending
	sort.Slice(languages, func(i, j int) bool {
		if languages[i].Code == languages[j].Code {
			return languages[i].Lines > languages[j].Lines
		}
		return languages[i].Code > languages[j].Code
	})

	return &models.TokeiStats{
		Loaded:    true,
		Total:     total,
		Languages: languages,
	}, nil
}
