package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// getFileHash computes the SHA256 hex digest of a file, or returns empty string on error
func getFileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// getBrowserUseVersion retrieves the output of `browser-use --version`
func getBrowserUseVersion(buPath string) string {
	if buPath == "" {
		return ""
	}
	cmd := exec.Command(buPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runPhaseCanonicalSkills executes Phase 2: Canonical Skills Hub update
func runPhaseCanonicalSkills(homeDir string, dryRunFlag bool) (pluginResult, bool) {
	npxPath := findNPX(homeDir)
	if npxPath == "" {
		return pluginResult{
			id:     "canonical-skills (npx)",
			status: statusSkipped,
		}, false
	}

	tStart := time.Now()
	rc, _ := runCmd(npxPath, []string{"skills", "update", "-g", "-y"}, nil, dryRunFlag)
	totalElapsed := int(time.Since(tStart).Seconds())

	if dryRunFlag {
		return pluginResult{
			id:      "canonical-skills (npx)",
			rc:      0,
			elapsed: 0,
			status:  statusDryRun,
		}, true
	}

	if rc != 0 {
		return pluginResult{
			id:      "canonical-skills (npx)",
			rc:      rc,
			elapsed: totalElapsed,
			status:  statusFailed,
		}, true
	}

	return pluginResult{
		id:      "canonical-skills (npx)",
		rc:      0,
		elapsed: totalElapsed,
		status:  statusUpdated,
	}, true
}

// runPhaseStandaloneSkills executes Phase 3: Standalone Tool Skill Generation
func runPhaseStandaloneSkills(homeDir string, dryRunFlag bool) (pluginResult, bool) {
	buPath := findBrowserUse(homeDir)
	if buPath == "" {
		return pluginResult{
			id:     "browser-use (skill)",
			status: statusSkipped,
		}, false
	}

	tStart := time.Now()
	skillPath := filepath.Join(CanonicalSkillsHub(homeDir), "browser-use", "SKILL.md")

	beforeVer := ""
	beforeHash := ""
	if !dryRunFlag {
		beforeVer = getBrowserUseVersion(buPath)
		beforeHash = getFileHash(skillPath)
	}

	rcSkill, _ := runCmd(buPath, []string{"skill", "install", "--no-install"}, nil, dryRunFlag)
	totalElapsed := int(time.Since(tStart).Seconds())

	if rcSkill != 0 && !dryRunFlag {
		return pluginResult{
			id:      "browser-use (skill)",
			rc:      rcSkill,
			elapsed: totalElapsed,
			status:  statusFailed,
		}, true
	}

	if dryRunFlag {
		return pluginResult{
			id:      "browser-use (skill)",
			rc:      0,
			elapsed: totalElapsed,
			status:  statusDryRun,
		}, true
	}

	afterVer := getBrowserUseVersion(buPath)
	afterHash := getFileHash(skillPath)

	isUpToDate := (beforeVer == afterVer && beforeHash == afterHash && beforeHash != "")
	if isUpToDate {
		return pluginResult{
			id:      "browser-use (skill)",
			rc:      0,
			elapsed: totalElapsed,
			status:  statusUpToDate,
		}, true
	}

	return pluginResult{
		id:      "browser-use (skill)",
		rc:      0,
		elapsed: totalElapsed,
		status:  statusUpdated,
	}, true
}
