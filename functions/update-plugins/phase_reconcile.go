package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type SpokeConfig struct {
	AgentName string
	Dir       string
}

type ReconcileStats struct {
	HealedCount int
	PrunedCount int
	HealedNames []string
	PrunedNames []string
}

// ReconcileSpokes synchronizes the canonical skills hub to each agent spoke directory
func ReconcileSpokes(canonicalHub string, spokes []SpokeConfig, dryRun bool) (ReconcileStats, error) {
	var stats ReconcileStats

	// 1. Scan canonical hub
	entries, err := os.ReadDir(canonicalHub)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, err
	}

	canonicalSkills := make(map[string]string)
	for _, entry := range entries {
		// Only consider directories or symlinks to directories
		fullPath := filepath.Join(canonicalHub, entry.Name())
		if fi, err := os.Stat(fullPath); err == nil && fi.IsDir() {
			canonicalSkills[entry.Name()] = fullPath
		}
	}

	// 2. Process each spoke
	for _, spoke := range spokes {
		// Ensure spoke directory exists
		if _, err := os.Stat(spoke.Dir); os.IsNotExist(err) {
			if !dryRun {
				if err := os.MkdirAll(spoke.Dir, 0755); err != nil {
					return stats, fmt.Errorf("failed to create spoke directory %s: %w", spoke.Dir, err)
				}
			}
		}

		spokeEntries, err := os.ReadDir(spoke.Dir)
		if err != nil {
			continue
		}

		// Step A: Prune broken / dangling symlinks in spoke
		for _, se := range spokeEntries {
			linkPath := filepath.Join(spoke.Dir, se.Name())
			lfi, err := os.Lstat(linkPath)
			if err != nil {
				continue
			}

			if lfi.Mode()&os.ModeSymlink != 0 {
				// Symlink: check if target is valid
				if _, err := os.Stat(linkPath); os.IsNotExist(err) {
					// Target does not exist -> dangling symlink
					if !dryRun {
						_ = os.Remove(linkPath)
					}
					stats.PrunedCount++
					stats.PrunedNames = append(stats.PrunedNames, fmt.Sprintf("%s:%s", spoke.AgentName, se.Name()))
				}
			}
		}

		// Step B: Heal missing skills in spoke
		var skillNames []string
		for name := range canonicalSkills {
			skillNames = append(skillNames, name)
		}
		sort.Strings(skillNames)

		for _, name := range skillNames {
			hubSkillPath := canonicalSkills[name]
			spokeSkillPath := filepath.Join(spoke.Dir, name)

			// Check if already exists in spoke
			if _, err := os.Lstat(spokeSkillPath); err == nil {
				// Already exists (either as directory copy like browser-use in claude, or valid symlink)
				continue
			}

			// Skill is missing in spoke -> heal by symlinking
			if !dryRun {
				relTarget, err := filepath.Rel(spoke.Dir, hubSkillPath)
				target := hubSkillPath
				if err == nil {
					target = relTarget
				}
				if err := os.Symlink(target, spokeSkillPath); err != nil {
					// Fallback to absolute path if relative fails
					_ = os.Symlink(hubSkillPath, spokeSkillPath)
				}
			}
			stats.HealedCount++
			stats.HealedNames = append(stats.HealedNames, fmt.Sprintf("%s:%s", spoke.AgentName, name))
		}
	}

	return stats, nil
}

// ReconcileOmpSkills ensures the directory symlink ~/.omp/agent/skills -> ~/.agents/skills exists
func ReconcileOmpSkills(homeDir string, dryRun bool) (bool, error) {
	hubPath := CanonicalSkillsHub(homeDir)
	ompSkills := OmpSkillsDir(homeDir)

	if fi, err := os.Stat(hubPath); err != nil || !fi.IsDir() {
		return false, nil
	}

	lfi, err := os.Lstat(ompSkills)
	if err == nil {
		if lfi.Mode()&os.ModeSymlink != 0 {
			// Check if target exists
			if _, statErr := os.Stat(ompSkills); statErr == nil {
				return false, nil
			}
			// Dangling symlink: remove and recreate
			if !dryRun {
				_ = os.Remove(ompSkills)
			}
		} else {
			// Already an existing file or directory
			return false, nil
		}
	}

	if !dryRun {
		_ = os.MkdirAll(filepath.Dir(ompSkills), 0755)
		relTarget, err := filepath.Rel(filepath.Dir(ompSkills), hubPath)
		target := hubPath
		if err == nil {
			target = relTarget
		}
		if err := os.Symlink(target, ompSkills); err != nil {
			_ = os.Symlink(hubPath, ompSkills)
		}
	}
	return true, nil
}

// runPhaseReconcile executes Phase 4: Hub-and-Spoke Reconciliation
func runPhaseReconcile(homeDir string, enabledAgents map[string]bool, dryRun bool) (pluginResult, ReconcileStats) {
	tStart := time.Now()
	var spokes []SpokeConfig

	if enabledAgents["claude"] {
		spokes = append(spokes, SpokeConfig{
			AgentName: "claude",
			Dir:       ClaudeSkillsDir(homeDir),
		})
	}
	if enabledAgents["pi"] {
		spokes = append(spokes, SpokeConfig{
			AgentName: "pi",
			Dir:       PiSkillsDir(homeDir),
		})
	}
	if enabledAgents["gemini"] {
		spokes = append(spokes, SpokeConfig{
			AgentName: "gemini",
			Dir:       GeminiSkillsDir(homeDir),
		})
	}

	hub := CanonicalSkillsHub(homeDir)
	stats, _ := ReconcileSpokes(hub, spokes, dryRun)

	if enabledAgents["omp"] {
		healedOmp, _ := ReconcileOmpSkills(homeDir, dryRun)
		if healedOmp {
			stats.HealedCount++
			stats.HealedNames = append(stats.HealedNames, "omp:skills")
		}
	}

	totalElapsed := int(time.Since(tStart).Seconds())
	status := statusUpToDate
	if stats.HealedCount > 0 || stats.PrunedCount > 0 {
		if dryRun {
			status = statusDryRun
		} else {
			status = statusUpdated
		}
	} else if dryRun {
		status = statusDryRun
	}

	return pluginResult{
		id:      "hub-and-spoke reconciliation",
		rc:      0,
		elapsed: totalElapsed,
		status:  status,
	}, stats
}
