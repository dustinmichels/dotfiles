package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustinmichels/agent-ls/storage"
)

func TestParseDaysDuration(t *testing.T) {
	d, err := parseDaysDuration("14d")
	if err != nil {
		t.Fatalf("expected 14d to parse, got err: %v", err)
	}
	expected := 14 * 24 * time.Hour
	if d != expected {
		t.Fatalf("expected %v, got %v", expected, d)
	}

	d2, err := parseDaysDuration("30d")
	if err != nil {
		t.Fatalf("expected 30d to parse, got err: %v", err)
	}
	expected2 := 30 * 24 * time.Hour
	if d2 != expected2 {
		t.Fatalf("expected %v, got %v", expected2, d2)
	}

	d3, err := parseDaysDuration("24h")
	if err != nil {
		t.Fatalf("expected 24h to parse, got err: %v", err)
	}
	if d3 != 24*time.Hour {
		t.Fatalf("expected 24h, got %v", d3)
	}
}

func TestEndToEndCleanDryRunAndTrashWithRestore(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("PATH", "/usr/bin:/bin")

	oldTime := time.Now().Add(-30 * 24 * time.Hour)

	// 1. Fixture: OMP active session
	ompProj := filepath.Join(tmpHome, ".omp", "agent", "sessions", "-GitRepos-test")
	if err := os.MkdirAll(ompProj, 0755); err != nil {
		t.Fatal(err)
	}
	ompJsonl := filepath.Join(ompProj, "2026-08-01T00-00-00-000Z_test1.jsonl")
	ompSidecar := filepath.Join(ompProj, "2026-08-01T00-00-00-000Z_test1")
	ompLock := filepath.Join(ompProj, ".2026-08-01T00-00-00-000Z_test1.jsonl.lock.os")
	_ = os.WriteFile(ompJsonl, []byte("omp log transcript"), 0644)
	_ = os.MkdirAll(ompSidecar, 0755)
	_ = os.WriteFile(filepath.Join(ompSidecar, "__advisor.jsonl"), []byte("advisor data"), 0644)
	_ = os.WriteFile(ompLock, []byte{}, 0600)
	_ = os.Chtimes(ompJsonl, oldTime, oldTime)

	// 2. Fixture: Antigravity session present in BOTH stores (antigravity and antigravity-ide)
	antiConvDir := filepath.Join(tmpHome, ".gemini", "antigravity", "conversations")
	antiIdeConvDir := filepath.Join(tmpHome, ".gemini", "antigravity-ide", "conversations")
	antiBrainDir := filepath.Join(tmpHome, ".gemini", "antigravity", "brain", "anti-test-convo")
	_ = os.MkdirAll(antiConvDir, 0755)
	_ = os.MkdirAll(antiIdeConvDir, 0755)
	_ = os.MkdirAll(antiBrainDir, 0755)

	antiDb1 := filepath.Join(antiConvDir, "anti-test-convo.db")
	antiDb2 := filepath.Join(antiIdeConvDir, "anti-test-convo.db")
	antiWalkthrough := filepath.Join(antiBrainDir, "walkthrough.md")
	_ = os.WriteFile(antiDb1, []byte("db1 content"), 0644)
	_ = os.WriteFile(antiDb2, []byte("db2 content"), 0644)
	_ = os.WriteFile(antiWalkthrough, []byte("# Walkthrough: Test Convo"), 0644)
	_ = os.Chtimes(antiDb1, oldTime, oldTime)

	// 3. Fixture: Claude session older than 14d (MUST NOT BE DELETED)
	claudeProj := filepath.Join(tmpHome, ".claude", "projects", "-Users-test-repo")
	_ = os.MkdirAll(claudeProj, 0755)
	claudeJsonl := filepath.Join(claudeProj, "claude-session.jsonl")
	_ = os.WriteFile(claudeJsonl, []byte(`{"type":"user","message":{"content":"hello"}}`), 0644)
	_ = os.Chtimes(claudeJsonl, oldTime, oldTime)

	// --- TEST A: Dry Run ---
	runCleanCmd([]string{"--dry-run"})

	// Assert dry run did not delete anything
	for _, p := range []string{ompJsonl, ompSidecar, ompLock, antiDb1, antiDb2, antiWalkthrough, claudeJsonl} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Fatalf("dry-run deleted file that should have been kept: %s", p)
		}
	}

	// --- TEST B: Live Clean ---
	runCleanCmd([]string{})

	// 1. Assert OMP files are gone from original path
	if _, err := os.Stat(ompJsonl); !os.IsNotExist(err) {
		t.Fatalf("expected ompJsonl to be removed, but still exists")
	}

	// 2. Assert Antigravity files are gone from original paths in BOTH stores
	if _, err := os.Stat(antiDb1); !os.IsNotExist(err) {
		t.Fatalf("expected antiDb1 to be removed, but still exists")
	}
	if _, err := os.Stat(antiDb2); !os.IsNotExist(err) {
		t.Fatalf("expected antiDb2 to be removed, but still exists")
	}

	// 3. Assert Claude session was PRESERVED (list-only)
	if _, err := os.Stat(claudeJsonl); os.IsNotExist(err) {
		t.Fatalf("claude session was deleted, but it should be protected from deletion")
	}

	// 4. Assert files exist in Trash preserving relative directory layout (no collisions)
	trashRoot := filepath.Join(tmpHome, ".Trash", "agent-ls")
	entries, err := os.ReadDir(trashRoot)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected trash entries in %s, got err=%v, count=%d", trashRoot, err, len(entries))
	}

	var foundAntiTrashDir string
	for _, e := range entries {
		if strings.Contains(e.Name(), "anti-test-convo") {
			foundAntiTrashDir = filepath.Join(trashRoot, e.Name())
			break
		}
	}
	if foundAntiTrashDir == "" {
		t.Fatalf("expected to find antigravity session trash folder in %s", trashRoot)
	}

	// Check that BOTH antiDb1 and antiDb2 exist under their distinct relative paths in Trash!
	trashedDb1 := filepath.Join(foundAntiTrashDir, ".gemini", "antigravity", "conversations", "anti-test-convo.db")
	trashedDb2 := filepath.Join(foundAntiTrashDir, ".gemini", "antigravity-ide", "conversations", "anti-test-convo.db")
	if _, err := os.Stat(trashedDb1); os.IsNotExist(err) {
		t.Fatalf("expected trashedDb1 at %s, but not found", trashedDb1)
	}
	if _, err := os.Stat(trashedDb2); os.IsNotExist(err) {
		t.Fatalf("expected trashedDb2 at %s (basename collision avoided!), but not found", trashedDb2)
	}

	// Check manifest exists
	manifestPath := filepath.Join(foundAntiTrashDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("expected manifest.json at %s, but not found", manifestPath)
	}

	// --- TEST C: Programmatic Restore from Trash ---
	if err := storage.RestoreFromTrash(foundAntiTrashDir); err != nil {
		t.Fatalf("RestoreFromTrash failed: %v", err)
	}

	// Assert files are restored to original locations
	if _, err := os.Stat(antiDb1); os.IsNotExist(err) {
		t.Fatalf("expected antiDb1 to be restored at %s", antiDb1)
	}
	if _, err := os.Stat(antiDb2); os.IsNotExist(err) {
		t.Fatalf("expected antiDb2 to be restored at %s", antiDb2)
	}
}
