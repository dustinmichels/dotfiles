package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type installedPluginsFile struct {
	Plugins map[string]json.RawMessage `json:"plugins"`
}

type pluginListItem struct {
	ID string `json:"id"`
}

type pluginScopeEntry struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha"`
}

func parsePluginScopeEntries(raw json.RawMessage) []pluginScopeEntry {
	var list []pluginScopeEntry
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var single pluginScopeEntry
	if err := json.Unmarshal(raw, &single); err == nil {
		return []pluginScopeEntry{single}
	}
	return nil
}

func getPluginFingerprint(homeDir string, id string) string {
	pluginFile := filepath.Join(ClaudePluginsDir(homeDir), "installed_plugins.json")
	data, err := os.ReadFile(pluginFile)
	if err != nil {
		return ""
	}
	var parsed installedPluginsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ""
	}
	raw, ok := parsed.Plugins[id]
	if !ok {
		return ""
	}
	entries := parsePluginScopeEntries(raw)
	var parts []string
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s", e.Scope, e.Version, e.GitCommitSha, e.InstallPath))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

func getInstalledPluginIDs(homeDir string) []string {
	var pluginIDs []string

	// 1. Try ~/.claude/plugins/installed_plugins.json
	pluginFile := filepath.Join(ClaudePluginsDir(homeDir), "installed_plugins.json")
	if data, err := os.ReadFile(pluginFile); err == nil {
		var parsed installedPluginsFile
		if err := json.Unmarshal(data, &parsed); err == nil && len(parsed.Plugins) > 0 {
			for id := range parsed.Plugins {
				pluginIDs = append(pluginIDs, id)
			}
			sort.Strings(pluginIDs)
			return pluginIDs
		}
	}

	// 2. Fallback to `claude plugin list --json`
	claudePath := findClaude(homeDir)
	if claudePath != "" {
		cmd := exec.Command(claudePath, "plugin", "list", "--json")
		cmd.Stdin = strings.NewReader("")
		out, err := cmd.Output()
		if err == nil {
			var list []pluginListItem
			if err := json.Unmarshal(out, &list); err == nil {
				for _, item := range list {
					if item.ID != "" {
						pluginIDs = append(pluginIDs, item.ID)
					}
				}
				sort.Strings(pluginIDs)
				return pluginIDs
			}
		}
	}

	return pluginIDs
}

// updateClaudePlugin updates an individual Claude plugin
func updateClaudePlugin(homeDir, claudePath, id string, yesFlag, dryRunFlag bool) pluginResult {
	tStart := time.Now()
	beforeFP := ""
	if !dryRunFlag {
		beforeFP = getPluginFingerprint(homeDir, id)
	}

	args := []string{"plugin", "update", id}
	if yesFlag {
		args = append(args, "-y")
	}

	rc, tElapsed := runCmd(claudePath, args, nil, dryRunFlag)
	totalElapsed := int(time.Since(tStart).Seconds())

	if dryRunFlag {
		return pluginResult{
			id:      id,
			rc:      0,
			elapsed: tElapsed,
			status:  statusDryRun,
		}
	}

	if rc != 0 {
		return pluginResult{
			id:      id,
			rc:      rc,
			elapsed: totalElapsed,
			status:  statusFailed,
		}
	}

	afterFP := getPluginFingerprint(homeDir, id)
	isUpToDate := (beforeFP == afterFP && beforeFP != "")
	if isUpToDate {
		return pluginResult{
			id:      id,
			rc:      0,
			elapsed: totalElapsed,
			status:  statusUpToDate,
		}
	}

	return pluginResult{
		id:      id,
		rc:      0,
		elapsed: totalElapsed,
		status:  statusUpdated,
	}
}

// updateOmpPlugins runs `omp plugin upgrade`
func updateOmpPlugins(homeDir string, dryRunFlag bool) (pluginResult, bool) {
	ompPath := findOmp(homeDir)
	if ompPath == "" {
		return pluginResult{
			id:     "omp plugins",
			status: statusSkipped,
		}, false
	}

	tStart := time.Now()
	args := []string{"plugin", "upgrade"}
	if dryRunFlag {
		args = append(args, "--dry-run")
	}

	rc, elapsed := runCmd(ompPath, args, nil, dryRunFlag)
	totalElapsed := int(time.Since(tStart).Seconds()) + elapsed

	status := statusUpdated
	if dryRunFlag {
		status = statusDryRun
	} else if rc != 0 {
		status = statusFailed
	} else if rc == 0 {
		status = statusUpToDate
	}

	return pluginResult{
		id:      "omp plugins",
		rc:      rc,
		elapsed: totalElapsed,
		status:  status,
	}, true
}

// validateCodexPlugins checks configured plugins against marketplaces
func validateCodexPlugins(homeDir string, dryRunFlag bool) (pluginResult, bool) {
	codexConfig := CodexConfigFile(homeDir)
	if _, err := os.Stat(codexConfig); os.IsNotExist(err) {
		return pluginResult{
			id:     "codex plugins",
			status: statusSkipped,
		}, false
	}

	tStart := time.Now()
	codexPath := findCodex(homeDir)
	var rc int
	var tElapsed int
	if codexPath != "" {
		rc, tElapsed = runCmd(codexPath, []string{"plugin", "list", "--json"}, nil, dryRunFlag)
	}

	totalElapsed := int(time.Since(tStart).Seconds()) + tElapsed
	status := statusUpToDate
	if dryRunFlag {
		status = statusDryRun
	} else if rc != 0 {
		status = statusFailed
	}

	return pluginResult{
		id:      "codex plugins",
		rc:      rc,
		elapsed: totalElapsed,
		status:  status,
	}, true
}

// runPhasePlugins executes Phase 6: Installed Plugins for all enabled agents
func runPhasePlugins(homeDir string, enabledAgents map[string]bool, yesFlag, dryRunFlag bool) []pluginResult {
	var results []pluginResult

	// Claude plugins
	if enabledAgents["claude"] {
		claudePath := findClaude(homeDir)
		if claudePath != "" {
			pluginIDs := getInstalledPluginIDs(homeDir)
			for _, id := range pluginIDs {
				res := updateClaudePlugin(homeDir, claudePath, id, yesFlag, dryRunFlag)
				results = append(results, res)
			}
		}
	}

	// Codex plugins
	if enabledAgents["codex"] {
		if res, ok := validateCodexPlugins(homeDir, dryRunFlag); ok {
			results = append(results, res)
		}
	}

	// OMP plugins
	if enabledAgents["omp"] {
		if res, ok := updateOmpPlugins(homeDir, dryRunFlag); ok {
			results = append(results, res)
		}
	}

	return results
}
