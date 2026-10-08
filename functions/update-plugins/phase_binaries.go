package main

import (
	"time"
)

// runPhaseBinaries executes Phase 1: Tool Binaries
// Upgrades browser-use via uv or native updater, then reloads the daemon.
func runPhaseBinaries(homeDir string, yesFlag, dryRunFlag bool) (pluginResult, bool) {
	buPath := findBrowserUse(homeDir)
	if buPath == "" {
		return pluginResult{
			id:     "browser-use (binary)",
			status: statusSkipped,
		}, false
	}

	tStart := time.Now()
	uvPath := findUV(homeDir)

	var rc int
	var tElapsed int
	if uvPath != "" {
		rc, tElapsed = runCmd(uvPath, []string{"tool", "upgrade", "browser-use"}, nil, dryRunFlag)
	} else {
		args := []string{"--update"}
		if yesFlag {
			args = append(args, "-y")
		}
		rc, tElapsed = runCmd(buPath, args, nil, dryRunFlag)
	}

	if rc != 0 && !dryRunFlag {
		return pluginResult{
			id:      "browser-use (binary)",
			rc:      rc,
			elapsed: int(time.Since(tStart).Seconds()),
			status:  statusFailed,
		}, true
	}

	// Daemon reload
	runCmd(buPath, []string{"--reload"}, nil, dryRunFlag)

	totalElapsed := int(time.Since(tStart).Seconds())
	if dryRunFlag {
		return pluginResult{
			id:      "browser-use (binary)",
			rc:      0,
			elapsed: tElapsed,
			status:  statusDryRun,
		}, true
	}

	return pluginResult{
		id:      "browser-use (binary)",
		rc:      0,
		elapsed: totalElapsed,
		status:  statusUpdated,
	}, true
}
