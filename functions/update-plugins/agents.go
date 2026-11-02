package main

import (
	"path/filepath"
	"strings"
)

var agentNames = []string{"claude", "codex", "omp", "pi", "gemini"}

// agentRows inventories the agent hosts themselves: version and where/how each is installed.
// Read-only: each agent updates itself (native updater, app, `omp update`).
func (a *app) agentRows() []row {
	var rows []row
	add := func(name, version, via, note string) {
		if a.agents[name] {
			rows = append(rows, row{st: stInfo, name: name, version: version, via: via, note: note})
		}
	}

	if p := findClaude(a.home); p != "" && a.agents["claude"] {
		via := tilde(a.home, p)
		if real, err := filepath.EvalSymlinks(p); err == nil && strings.Contains(real, "/.local/share/claude/versions/") {
			via = "native installer"
		}
		add("claude", firstField(read(p, "--version")), via, "self-updates")
	}

	if p := findCodex(a.home); p != "" && a.agents["codex"] {
		via := tilde(a.home, p)
		if i := strings.Index(p, ".app/"); i >= 0 {
			via = filepath.Base(p[:i+4])
		}
		add("codex", lastField(read(p, "--version")), via, "updates with the app")
	}

	if p := findOmp(a.home); p != "" && a.agents["omp"] {
		v := read(p, "--version")
		if i := strings.LastIndex(v, "/"); i >= 0 {
			v = v[i+1:]
		}
		add("omp", v, tilde(a.home, p), "omp update")
	}

	if p := findExecutable(a.home, "pi"); p != "" && a.agents["pi"] {
		add("pi", lastField(read(p, "--version")), tilde(a.home, p), "")
	} else if d := filepath.Join(a.home, ".pi", "agent"); exists(d) {
		add("pi", "—", tilde(a.home, d), "config only; no pi CLI on PATH")
	}

	switch {
	case exists("/Applications/Antigravity.app"):
		add("gemini", "—", "Antigravity.app", "updates with the app")
	case exists(filepath.Join(a.home, ".gemini")):
		add("gemini", "—", "~/.gemini", "config only")
	}
	return rows
}

func firstField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

func lastField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[len(f)-1]
	}
	return ""
}
