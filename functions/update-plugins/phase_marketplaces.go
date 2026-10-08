package main

import (
	"time"
)

// runPhaseMarketplaces executes Phase 5: Remote Marketplaces
func runPhaseMarketplaces(homeDir string, enabledAgents map[string]bool, dryRun bool) []pluginResult {
	var results []pluginResult

	// Claude marketplace
	if enabledAgents["claude"] {
		claudePath := findClaude(homeDir)
		if claudePath != "" {
			tStart := time.Now()
			rc, elapsed := runCmd(claudePath, []string{"plugin", "marketplace", "update"}, nil, dryRun)
			status := statusUpdated
			if dryRun {
				status = statusDryRun
			} else if rc != 0 {
				status = statusFailed
			} else if rc == 0 {
				status = statusUpToDate
			}
			results = append(results, pluginResult{
				id:      "claude marketplace",
				rc:      rc,
				elapsed: int(time.Since(tStart).Seconds()) + elapsed,
				status:  status,
			})
		}
	}

	// Codex marketplace
	if enabledAgents["codex"] {
		codexPath := findCodex(homeDir)
		if codexPath != "" {
			tStart := time.Now()
			rc, elapsed := runCmd(codexPath, []string{"plugin", "marketplace", "upgrade", "--json"}, nil, dryRun)
			status := statusUpdated
			if dryRun {
				status = statusDryRun
			} else if rc != 0 {
				status = statusFailed
			} else if rc == 0 {
				status = statusUpToDate
			}
			results = append(results, pluginResult{
				id:      "codex marketplace",
				rc:      rc,
				elapsed: int(time.Since(tStart).Seconds()) + elapsed,
				status:  status,
			})
		}
	}

	// OMP marketplace
	if enabledAgents["omp"] {
		ompPath := findOmp(homeDir)
		if ompPath != "" {
			tStart := time.Now()
			rc, elapsed := runCmd(ompPath, []string{"plugin", "marketplace"}, nil, dryRun)
			status := statusUpdated
			if dryRun {
				status = statusDryRun
			} else if rc != 0 {
				status = statusFailed
			} else if rc == 0 {
				status = statusUpToDate
			}
			results = append(results, pluginResult{
				id:      "omp marketplace",
				rc:      rc,
				elapsed: int(time.Since(tStart).Seconds()) + elapsed,
				status:  status,
			})
		}
	}

	return results
}
