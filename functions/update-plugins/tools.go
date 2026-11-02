package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// toolOwnedSkills maps hub skills that are written by an external tool (not skills.sh)
// to that tool. Their copies are refreshed by the tool's own update path.
var toolOwnedSkills = map[string]string{
	"browser-use":            "browser-use CLI",
	"hindsight-coding-agent": "hindsight installer",
}

type skillLoc struct{ label, path string }

// skillCopies finds every real (non-symlink) copy of a skill across known skill roots.
// Symlinked entries resolve to the hub and need no refresh of their own.
func skillCopies(home, name string) []skillLoc {
	var locs []skillLoc
	for _, root := range skillRoots(home) {
		p := filepath.Join(root.dir, name)
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			continue
		}
		if exists(filepath.Join(p, "SKILL.md")) {
			locs = append(locs, skillLoc{root.label, p})
		}
	}
	return locs
}

func labels(locs []skillLoc) []string {
	out := make([]string, len(locs))
	for i, l := range locs {
		out[i] = l.label
	}
	return out
}

func staleCopies(locs []skillLoc, srcHash string) []skillLoc {
	var stale []skillLoc
	for _, l := range locs {
		if dirHash(l.path) != srcHash {
			stale = append(stale, l)
		}
	}
	return stale
}

func (a *app) toolRows() []row {
	var rows []row
	if r, ok := a.browserUse(); ok {
		rows = append(rows, r)
	}
	if r, ok := a.hindsight(); ok {
		rows = append(rows, r)
	}
	return rows
}

// browserUse: uv tool → `uv tool upgrade`, then `browser-use skill install` rewrites
// SKILL.md in every assistant dir it targets, then the daemon reloads.
func (a *app) browserUse() (row, bool) {
	bu := findBrowserUse(a.home)
	if bu == "" {
		return row{}, false
	}
	uv := findUV(a.home)
	r := row{st: stCurrent, name: "browser-use"}

	version := func() string { return uvToolVersion(uv, "browser-use") }
	before := version()
	if before != "" {
		r.via = "uv tool"
	} else {
		r.via = tilde(a.home, bu)
		version = func() string { return firstField(read(bu, "--version")) }
		before = version()
	}
	r.version = before

	skillPath := filepath.Join(hubDir(a.home), "browser-use")
	skillBefore := dirHash(skillPath)

	var ok bool
	if r.via == "uv tool" {
		ok = a.mutate(&r, quick, uv, "tool", "upgrade", "browser-use")
	} else {
		args := []string{"--update"}
		if a.yes {
			args = append(args, "-y")
		}
		ok = a.mutate(&r, quick, bu, args...)
	}
	ok = ok && a.mutate(&r, quick, bu, "skill", "install", "--no-install")
	ok = ok && a.mutate(&r, quick, bu, "--reload")

	if !a.dryRun {
		after := version()
		r.version = versionChange(before, after)
		if ok && (after != before || dirHash(skillPath) != skillBefore) {
			r.raise(stUpdated)
		}
	}

	copies := skillCopies(a.home, "browser-use")
	r.addNote("skill → " + strings.Join(labels(copies), ", "))
	if stale := staleCopies(copies, dirHash(skillPath)); len(stale) > 0 && !a.dryRun {
		r.raise(stWarn)
		r.addNote("stale copies: " + strings.Join(labels(stale), ", "))
	}
	return r, true
}

// uvToolVersion parses `uv tool list` ("browser-use v0.13.14") for an installed tool.
func uvToolVersion(uv, tool string) string {
	for _, line := range strings.Split(read(uv, "tool", "list"), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == tool {
			return strings.TrimPrefix(f[1], "v")
		}
	}
	return ""
}

// hindsight: `hindsight-coding-agents update` re-stages the runtime only. The installer copied
// its skill into each wired agent's skill dir at install time, so those copies are re-synced
// from the runtime here — otherwise agents keep reading the old skill text.
func (a *app) hindsight() (row, bool) {
	rt := hindsightRuntime(a.home)
	var pkg struct{ Version string }
	if !readJSON(filepath.Join(rt, "package.json"), &pkg) {
		return row{}, false
	}
	r := row{st: stCurrent, name: "hindsight", version: pkg.Version, via: "npm package"}
	var origin struct{ Source string }
	if readJSON(filepath.Join(rt, ".install-origin.json"), &origin) && strings.Contains(origin.Source, "/_npx/") {
		r.via = "npx installer"
	}

	npx := findNPX(a.home)
	if npx == "" {
		r.raise(stWarn)
		r.addNote("npx not found; cannot update")
		return r, true
	}
	ok := a.mutate(&r, slow, npx, "-y", "@vectorize-io/hindsight-coding-agents@latest", "update")
	if ok && !a.dryRun && readJSON(filepath.Join(rt, "package.json"), &pkg) {
		if pkg.Version != r.version {
			r.version = versionChange(r.version, pkg.Version)
			r.raise(stUpdated)
		}
	}

	src := filepath.Join(rt, "skill")
	copies := skillCopies(a.home, "hindsight-coding-agent")
	r.addNote("skill → " + strings.Join(labels(copies), ", "))
	stale := staleCopies(copies, dirHash(src))
	if len(stale) == 0 || !ok {
		return r, true
	}
	if a.dryRun {
		r.detail = append(r.detail, "↳ sync skill from runtime → "+strings.Join(labels(stale), ", "))
		r.raise(stPlanned)
		return r, true
	}
	for _, l := range stale {
		if err := copyTree(src, l.path); err != nil {
			r.raise(stFailed)
			r.detail = append(r.detail, fmt.Sprintf("sync skill → %s: %v", l.label, err))
			return r, true
		}
	}
	r.raise(stUpdated)
	if len(stale) == len(copies) {
		r.note = "skill synced → " + strings.Join(labels(copies), ", ")
	} else {
		r.addNote("synced " + strings.Join(labels(stale), ", "))
	}
	return r, true
}
