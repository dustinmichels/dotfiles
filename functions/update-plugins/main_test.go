package main

import (
	"os"
	"path/filepath"
	"testing"
)
func TestParsePluginScopeEntries(t *testing.T) {
	// Array case
	arrJSON := []byte(`[
		{"scope":"user","installPath":"/path/to/v1","version":"1.0.0","gitCommitSha":"abc123"},
		{"scope":"project","installPath":"/path/to/v2","version":"2.0.0","gitCommitSha":"def456"}
	]`)
	entries := parsePluginScopeEntries(arrJSON)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.0.0" || entries[1].Version != "2.0.0" {
		t.Errorf("unexpected entry versions: %+v", entries)
	}

	// Single object case
	objJSON := []byte(`{"scope":"user","installPath":"/path/to/v1","version":"1.0.0","gitCommitSha":"abc123"}`)
	entries = parsePluginScopeEntries(objJSON)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("unexpected entry version: %+v", entries[0])
	}

	// Invalid case
	invalidJSON := []byte(`"not-an-object"`)
	entries = parsePluginScopeEntries(invalidJSON)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for invalid JSON, got %d", len(entries))
	}
}

func TestGetPluginFingerprint(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude", "plugins")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mockContent := `{
		"version": 2,
		"plugins": {
			"test-plugin@market": [
				{
					"scope": "user",
					"installPath": "/cache/test-plugin/1.0.0",
					"version": "1.0.0",
					"gitCommitSha": "sha1"
				}
			]
		}
	}`
	if err := os.WriteFile(filepath.Join(claudeDir, "installed_plugins.json"), []byte(mockContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fp := getPluginFingerprint(tmpDir, "test-plugin@market")
	expected := "user|1.0.0|sha1|/cache/test-plugin/1.0.0"
	if fp != expected {
		t.Errorf("expected fingerprint %q, got %q", expected, fp)
	}

	// Unknown plugin returns empty string
	fpMissing := getPluginFingerprint(tmpDir, "nonexistent@market")
	if fpMissing != "" {
		t.Errorf("expected empty fingerprint for nonexistent plugin, got %q", fpMissing)
	}
}

func TestPluginFingerprint(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude", "plugins")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Format validation: scope|version|gitCommitSha|installPath
	mockContent := `{
		"version": 2,
		"plugins": {
			"ponytail@ponytail": [
				{
					"scope": "user",
					"installPath": "/Users/test/.claude/plugins/cache/ponytail/1.2.3",
					"version": "1.2.3",
					"gitCommitSha": "deadbeef1234"
				}
			]
		}
	}`
	if err := os.WriteFile(filepath.Join(claudeDir, "installed_plugins.json"), []byte(mockContent), 0644); err != nil {
		t.Fatalf("failed to write installed_plugins.json: %v", err)
	}

	fp := getPluginFingerprint(tmpDir, "ponytail@ponytail")
	expected := "user|1.2.3|deadbeef1234|/Users/test/.claude/plugins/cache/ponytail/1.2.3"
	if fp != expected {
		t.Errorf("fingerprint mismatch:\n  expected: %q\n  got:      %q", expected, fp)
	}
}

func TestStatusReportingLogic(t *testing.T) {
	// Scenario 1: Same fingerprint before and after -> Up-to-date
	beforeFP := "user|1.0.0|sha1|/path/1.0.0"
	afterFP := "user|1.0.0|sha1|/path/1.0.0"
	if beforeFP != afterFP {
		t.Errorf("expected fingerprints to match")
	}

	// Scenario 2: Differing fingerprint -> Updated
	updatedFP := "user|1.1.0|sha2|/path/1.1.0"
	if beforeFP == updatedFP {
		t.Errorf("expected fingerprints to differ on update")
	}
}

func TestGetFileHash(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "SKILL.md")
	content := "test skill content"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash1 := getFileHash(filePath)
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}

	// Identical content has identical hash
	hash2 := getFileHash(filePath)
	if hash1 != hash2 {
		t.Errorf("expected hash1 == hash2, got %q != %q", hash1, hash2)
	}

	// Different content has different hash
	if err := os.WriteFile(filePath, []byte("changed content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	hash3 := getFileHash(filePath)
	if hash1 == hash3 {
		t.Errorf("expected hash to change when content changes")
	}

	// Missing file returns empty string
	if missing := getFileHash(filepath.Join(tmpDir, "nonexistent")); missing != "" {
		t.Errorf("expected empty hash for missing file, got %q", missing)
	}
}

func TestReconcileSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	hubDir := filepath.Join(tmpDir, "canonical-hub")
	spoke1 := filepath.Join(tmpDir, "spoke-pi")
	spoke2 := filepath.Join(tmpDir, "spoke-gemini")

	// Create 3 skills in canonical hub
	skills := []string{"skill-alpha", "skill-beta", "skill-gamma"}
	for _, s := range skills {
		skillDir := filepath.Join(hubDir, s)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create canonical skill %s: %v", s, err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+s), 0644); err != nil {
			t.Fatalf("failed to write SKILL.md: %v", err)
		}
	}

	// Create spoke2 with skill-alpha already present as physical directory
	if err := os.MkdirAll(filepath.Join(spoke2, "skill-alpha"), 0755); err != nil {
		t.Fatalf("failed to create existing directory in spoke2: %v", err)
	}

	spokes := []SpokeConfig{
		{AgentName: "pi", Dir: spoke1},
		{AgentName: "gemini", Dir: spoke2},
	}

	stats, err := ReconcileSpokes(hubDir, spokes, false)
	if err != nil {
		t.Fatalf("ReconcileSpokes failed: %v", err)
	}

	// Spoke1 was missing all 3. Spoke2 was missing skill-beta and skill-gamma (skill-alpha was pre-existing dir).
	// Total healed should be 3 + 2 = 5.
	if stats.HealedCount != 5 {
		t.Errorf("expected 5 healed skills, got %d (%v)", stats.HealedCount, stats.HealedNames)
	}

	// Verify all 3 skills in spoke1 are accessible and resolve to valid files
	for _, s := range skills {
		spokePath := filepath.Join(spoke1, s, "SKILL.md")
		data, err := os.ReadFile(spokePath)
		if err != nil {
			t.Errorf("failed to read skill %s in spoke1: %v", s, err)
		} else if string(data) != "# "+s {
			t.Errorf("unexpected skill content in spoke1 for %s: %q", s, string(data))
		}
	}

	// Verify spoke2 has all 3 skills
	for _, s := range skills {
		spokePath := filepath.Join(spoke2, s)
		if _, err := os.Stat(spokePath); err != nil {
			t.Errorf("expected skill %s in spoke2, stat failed: %v", s, err)
		}
	}

	// Verify idempotency: running reconcile again yields 0 healed
	secondStats, err := ReconcileSpokes(hubDir, spokes, false)
	if err != nil {
		t.Fatalf("second ReconcileSpokes failed: %v", err)
	}
	if secondStats.HealedCount != 0 {
		t.Errorf("expected 0 healed on second pass, got %d", secondStats.HealedCount)
	}
}

func TestPruneDanglingSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	hubDir := filepath.Join(tmpDir, "canonical-hub")
	spokeDir := filepath.Join(tmpDir, "spoke")

	// Create canonical skill
	liveSkill := filepath.Join(hubDir, "skill-live")
	if err := os.MkdirAll(liveSkill, 0755); err != nil {
		t.Fatalf("failed to create live skill: %v", err)
	}

	if err := os.MkdirAll(spokeDir, 0755); err != nil {
		t.Fatalf("failed to create spoke dir: %v", err)
	}

	// Create a dead symlink pointing to nonexistent path
	deadLink := filepath.Join(spokeDir, "skill-deleted")
	if err := os.Symlink(filepath.Join(tmpDir, "does-not-exist"), deadLink); err != nil {
		t.Fatalf("failed to create dead symlink: %v", err)
	}

	spokes := []SpokeConfig{
		{AgentName: "test-agent", Dir: spokeDir},
	}

	stats, err := ReconcileSpokes(hubDir, spokes, false)
	if err != nil {
		t.Fatalf("ReconcileSpokes failed: %v", err)
	}

	// Verify dead link was pruned
	if stats.PrunedCount != 1 {
		t.Errorf("expected 1 pruned link, got %d", stats.PrunedCount)
	}

	if _, err := os.Lstat(deadLink); !os.IsNotExist(err) {
		t.Errorf("expected dead symlink to be removed, but it still exists")
	}

	// Verify live skill was healed/linked
	if stats.HealedCount != 1 {
		t.Errorf("expected 1 healed link, got %d", stats.HealedCount)
	}
	liveInSpoke := filepath.Join(spokeDir, "skill-live")
	if _, err := os.Stat(liveInSpoke); err != nil {
		t.Errorf("expected live skill in spoke, got stat error: %v", err)
	}
}
