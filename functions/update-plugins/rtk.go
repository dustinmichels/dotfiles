package main

import (
	"path/filepath"
	"slices"
	"strings"
)

// rtkTarget is one agent integration owned by `rtk init -g`. files are what that init writes;
// they are hashed before and after a re-init to tell whether anything changed.
type rtkTarget struct {
	agent string // key in --agents; its config dir is ~/.<agent>
	label string
	args  []string // after `rtk init -g`
	files []string
}

func rtkTargets(home string) []rtkTarget {
	p := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	return []rtkTarget{
		{"claude", "claude", []string{"--auto-patch"},
			[]string{p(".claude", "RTK.md"), p(".claude", "CLAUDE.md"), p(".claude", "settings.json")}},
		// --codex refuses --auto-patch; plain init never prompts and merges into AGENTS.md/hooks.json.
		{"codex", "codex", []string{"--codex"},
			[]string{p(".codex", "RTK.md"), p(".codex", "AGENTS.md"), p(".codex", "hooks.json")}},
		{"omp", "omp", []string{"--agent", "omp", "--auto-patch"},
			[]string{p(".omp", "agent", "extensions", "rtk.ts")}},
		// Hook only: `rtk init --gemini` replaces all of GEMINI.md. Antigravity's plugin carries the instructions.
		{"gemini", "gemini-cli", []string{"--gemini", "--hook-only", "--auto-patch"},
			[]string{p(".gemini", "hooks", "rtk-hook-gemini.sh"), p(".gemini", "settings.json")}},
		{"gemini", "antigravity", []string{"--agent", "antigravity", "--auto-patch"},
			[]string{p(".gemini", "config", "plugins", "rtk")}},
	}
}

func hashAll(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString(dirHash(p))
		b.WriteByte('|')
	}
	return b.String()
}

// rtkOutdated asks rtk whether re-running init would change anything for this target.
func (a *app) rtkOutdated(bin string, t rtkTarget) bool {
	if t.label == "antigravity" {
		// rtk's dry run always reports rewriting the plugin, so only absence is meaningful.
		return !exists(filepath.Join(t.files[0], "plugin.json"))
	}
	args := append([]string{"init", "-g"}, slices.DeleteFunc(slices.Clone(t.args), func(s string) bool { return s == "--auto-patch" })...)
	_, out := a.run(quick, bin, append(args, "--dry-run")...)
	return strings.Contains(out, "[dry-run] would")
}

// rtk: brew → `brew upgrade rtk`, then `rtk init -g …` per agent so each agent's hook,
// extension, or plugin and its instructions match what the installed rtk version ships.
func (a *app) rtk() (row, bool) {
	bin := findExecutable(a.home, "rtk")
	if bin == "" {
		return row{}, false
	}
	version := func() string { return lastField(read(bin, "--version")) }
	before := version()
	r := row{st: stCurrent, name: "rtk", version: before, via: tilde(a.home, bin)}
	if real, err := filepath.EvalSymlinks(bin); err == nil && strings.Contains(real, "/Cellar/rtk/") {
		r.via = "brew"
		if brew := findExecutable(a.home, "brew", "/opt/homebrew/bin/brew", "/usr/local/bin/brew"); brew != "" {
			a.mutate(&r, slow, brew, "upgrade", "rtk")
		}
	}
	if !a.dryRun {
		if after := version(); after != before {
			r.version = versionChange(before, after)
			r.raise(stUpdated)
		}
	}

	var wired, changed []string
	for _, t := range rtkTargets(a.home) {
		if !a.agents[t.agent] || !exists(filepath.Join(a.home, "."+t.agent)) {
			continue
		}
		wired = append(wired, t.label)
		args := append([]string{"init", "-g"}, t.args...)
		if a.dryRun {
			if a.rtkOutdated(bin, t) {
				changed = append(changed, t.label)
				a.mutate(&r, quick, bin, args...)
			}
			continue
		}
		h := hashAll(t.files)
		if a.mutate(&r, quick, bin, args...) && hashAll(t.files) != h {
			changed = append(changed, t.label)
		}
	}
	r.addNote("hooks → " + strings.Join(wired, ", "))
	switch {
	case len(changed) == 0:
	case a.dryRun:
		r.addNote("out of date: " + strings.Join(changed, ", "))
	default:
		r.raise(stUpdated)
		r.addNote("re-synced " + strings.Join(changed, ", "))
	}
	return r, true
}
