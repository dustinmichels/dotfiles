package main

import (
	"fmt"
	"time"
)

// runPhasePrune executes Phase 7: Cache Pruning, guarded by active process checks
func runPhasePrune(homeDir string, yesFlag, dryRunFlag, skipPruneFlag bool) (pluginResult, bool) {
	if skipPruneFlag {
		return pluginResult{
			id:     "claude cache prune",
			status: statusSkipped,
		}, false
	}

	claudePath := findClaude(homeDir)
	if claudePath == "" {
		return pluginResult{
			id:     "claude cache prune",
			status: statusSkipped,
		}, false
	}

	// Safety Preflight: Risk 5 (Mid-Session Cache Deletion Race)
	if isProcessRunning("claude") {
		fmt.Printf("   %s Active Claude Code session detected; skipping prune to avoid invalidating loaded plugins.\n",
			warnStyle.Render("▲"))
		return pluginResult{
			id:      "claude cache prune",
			rc:      0,
			elapsed: 0,
			status:  statusSkipped,
		}, true
	}

	tStart := time.Now()
	args := []string{"plugin", "prune"}
	if yesFlag {
		args = append(args, "-y")
	}

	rc, elapsed := runCmd(claudePath, args, nil, dryRunFlag)
	totalElapsed := int(time.Since(tStart).Seconds()) + elapsed

	if dryRunFlag {
		return pluginResult{
			id:      "claude cache prune",
			rc:      0,
			elapsed: 0,
			status:  statusDryRun,
		}, true
	}

	if rc != 0 {
		return pluginResult{
			id:      "claude cache prune",
			rc:      rc,
			elapsed: totalElapsed,
			status:  statusFailed,
		}, true
	}

	return pluginResult{
		id:      "claude cache prune",
		rc:      0,
		elapsed: totalElapsed,
		status:  statusUpdated,
	}, true
}
