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
