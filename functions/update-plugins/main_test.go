package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeSkill(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParsePluginScopeEntries(t *testing.T) {
	list := parsePluginScopeEntries([]byte(`[
		{"scope":"user","version":"1.0.0"},
		{"scope":"project","version":"2.0.0"}
	]`))
	if len(list) != 2 || list[1].Version != "2.0.0" {
		t.Fatalf("array form: got %+v", list)
	}
	single := parsePluginScopeEntries([]byte(`{"scope":"user","version":"1.0.0"}`))
	if len(single) != 1 || single[0].Version != "1.0.0" {
		t.Fatalf("object form: got %+v", single)
	}
	if got := parsePluginScopeEntries([]byte(`"nope"`)); len(got) != 0 {
		t.Fatalf("invalid form: got %+v", got)
	}
}

func TestPluginVersionPrefersUserScope(t *testing.T) {
	entries := []pluginScopeEntry{{Scope: "project", Version: "2.0.0"}, {Scope: "user", Version: "1.0.0"}}
	if v := pluginVersion(entries); v != "1.0.0" {
		t.Fatalf("got %q, want user-scope 1.0.0", v)
	}
}

func TestPluginFingerprintDetectsCommitOnlyChange(t *testing.T) {
	before := []pluginScopeEntry{{Scope: "user", Version: "1.0.0", GitCommitSha: "a", InstallPath: "/p"}}
	after := []pluginScopeEntry{{Scope: "user", Version: "1.0.0", GitCommitSha: "b", InstallPath: "/p"}}
	if pluginFingerprint(before) == pluginFingerprint(after) {
		t.Fatal("same version but new commit must count as updated")
	}
	reordered := []pluginScopeEntry{{Scope: "project", Version: "2"}, before[0]}
	if pluginFingerprint(reordered) != pluginFingerprint([]pluginScopeEntry{before[0], {Scope: "project", Version: "2"}}) {
		t.Fatal("scope order must not affect the fingerprint")
	}
}

func TestChangedSkills(t *testing.T) {
	before := skillLock{Skills: map[string]lockEntry{"same": {"r", "h1"}, "bumped": {"r", "h1"}}}
	after := skillLock{Skills: map[string]lockEntry{"same": {"r", "h1"}, "bumped": {"r", "h2"}, "new": {"r", "h3"}}}
	got := sortedKeys(changedSkills(before, after))
	if !slices.Equal(got, []string{"bumped", "new"}) {
		t.Fatalf("got %v", got)
	}
}

func TestReconcileSpokeHealsPrunesAndKeepsCopies(t *testing.T) {
	tmp := t.TempDir()
	hubPath := filepath.Join(tmp, "hub")
	for _, s := range []string{"alpha", "beta", "gamma"} {
		writeSkill(t, filepath.Join(hubPath, s), "# "+s)
	}
	spoke := filepath.Join(tmp, "spoke")
	writeSkill(t, filepath.Join(spoke, "alpha"), "# alpha copy") // installer-made copy
	writeSkill(t, filepath.Join(spoke, "mine"), "# local")       // spoke-only skill
	if err := os.Symlink(filepath.Join(tmp, "gone"), filepath.Join(spoke, "dead")); err != nil {
		t.Fatal(err)
	}

	hub := hubSkills(hubPath)
	rep, err := reconcileSpoke(hub, "pi", spoke, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rep.healed, []string{"beta", "gamma"}) {
		t.Errorf("healed = %v", rep.healed)
	}
	if !slices.Equal(rep.pruned, []string{"dead"}) {
		t.Errorf("pruned = %v", rep.pruned)
	}
	if !slices.Equal(rep.copies, []string{"alpha"}) || !slices.Equal(rep.local, []string{"mine"}) {
		t.Errorf("copies = %v, local = %v", rep.copies, rep.local)
	}
	if data, err := os.ReadFile(filepath.Join(spoke, "beta", "SKILL.md")); err != nil || string(data) != "# beta" {
		t.Errorf("healed link does not resolve to hub: %q %v", data, err)
	}
	if data, _ := os.ReadFile(filepath.Join(spoke, "alpha", "SKILL.md")); string(data) != "# alpha copy" {
		t.Error("existing copy must not be replaced")
	}
	if _, err := os.Lstat(filepath.Join(spoke, "dead")); !os.IsNotExist(err) {
		t.Error("dangling link not removed")
	}

	again, err := reconcileSpoke(hub, "pi", spoke, false)
	if err != nil || len(again.healed)+len(again.pruned) != 0 {
		t.Fatalf("second pass not idempotent: %+v %v", again, err)
	}
}

func TestReconcileSpokeDryRunChangesNothing(t *testing.T) {
	tmp := t.TempDir()
	hubPath := filepath.Join(tmp, "hub")
	writeSkill(t, filepath.Join(hubPath, "alpha"), "# alpha")
	spoke := filepath.Join(tmp, "spoke")

	rep, err := reconcileSpoke(hubSkills(hubPath), "pi", spoke, true)
	if err != nil || !slices.Equal(rep.healed, []string{"alpha"}) {
		t.Fatalf("dry run should report alpha: %+v %v", rep, err)
	}
	if exists(spoke) {
		t.Fatal("dry run created the spoke dir")
	}
}

func TestStaleCopiesSyncFromSource(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "runtime", "skill")
	writeSkill(t, src, "v2")
	fresh := skillLoc{"claude", filepath.Join(tmp, "claude", "s")}
	old := skillLoc{"pi", filepath.Join(tmp, "pi", "s")}
	writeSkill(t, fresh.path, "v2")
	writeSkill(t, old.path, "v1")

	stale := staleCopies([]skillLoc{fresh, old}, dirHash(src))
	if len(stale) != 1 || stale[0] != old {
		t.Fatalf("stale = %v", stale)
	}
	if err := copyTree(src, old.path); err != nil {
		t.Fatal(err)
	}
	if len(staleCopies([]skillLoc{fresh, old}, dirHash(src))) != 0 {
		t.Fatal("copy still stale after sync")
	}
}

func TestSkillCopiesSkipsSymlinks(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, filepath.Join(hubDir(home), "s"), "x")
	claude := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(hubDir(home), "s"), filepath.Join(claude, "s")); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(home, ".pi", "agent", "skills", "s"), "x")

	if got := labels(skillCopies(home, "s")); !slices.Equal(got, []string{"hub", "pi"}) {
		t.Fatalf("copies = %v, want hub and pi (claude is a link)", got)
	}
}

func TestLinkOmpSkills(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, filepath.Join(hubDir(home), "s"), "x")

	created, err := linkOmpSkills(home, false)
	if err != nil || !created {
		t.Fatalf("first link: created=%v err=%v", created, err)
	}
	if !exists(filepath.Join(ompSkillsDir(home), "s", "SKILL.md")) {
		t.Fatal("omp skills dir does not resolve to hub")
	}
	if created, err = linkOmpSkills(home, false); err != nil || created {
		t.Fatalf("second link: created=%v err=%v", created, err)
	}

	other := t.TempDir()
	writeSkill(t, filepath.Join(ompSkillsDir(other), "s"), "x")
	if _, err := linkOmpSkills(other, false); err == nil {
		t.Fatal("a real omp skills dir must be reported, not replaced")
	}
}

func TestParseOmpMarketplaces(t *testing.T) {
	out := "Configured Marketplaces:\n\n  claude-plugins-official  anthropics/claude-plugins-official\n  mine  me/repo\n"
	if got := parseOmpMarketplaces(out); !slices.Equal(got, []string{"claude-plugins-official", "mine"}) {
		t.Fatalf("got %v", got)
	}
	if got := parseOmpMarketplaces("No marketplaces configured\n\nAdd one with: omp plugin marketplace add <source>"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
