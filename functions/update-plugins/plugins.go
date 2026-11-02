package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ---- Claude Code ----------------------------------------------------------

type pluginScopeEntry struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha"`
}

// parsePluginScopeEntries accepts both shapes installed_plugins.json has used: a list of
// per-scope entries, or a single entry object.
func parsePluginScopeEntries(raw json.RawMessage) []pluginScopeEntry {
	var list []pluginScopeEntry
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var single pluginScopeEntry
	if err := json.Unmarshal(raw, &single); err == nil {
		return []pluginScopeEntry{single}
	}
	return nil
}

// readClaudePlugins returns plugin id → install entries, or nil if the file is unreadable.
func readClaudePlugins(home string) map[string][]pluginScopeEntry {
	var f struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if !readJSON(filepath.Join(claudePluginsDir(home), "installed_plugins.json"), &f) {
		return nil
	}
	out := make(map[string][]pluginScopeEntry, len(f.Plugins))
	for id, raw := range f.Plugins {
		out[id] = parsePluginScopeEntries(raw)
	}
	return out
}

// pluginFingerprint changes whenever any scope's version, commit, or install path changes.
func pluginFingerprint(entries []pluginScopeEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, strings.Join([]string{e.Scope, e.Version, e.GitCommitSha, e.InstallPath}, "|"))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

func pluginVersion(entries []pluginScopeEntry) string {
	for _, e := range entries {
		if e.Scope == "user" {
			return e.Version
		}
	}
	if len(entries) > 0 {
		return entries[0].Version
	}
	return ""
}

func (a *app) claudePluginRows() []row {
	claude := findClaude(a.home)
	if claude == "" {
		return nil
	}
	var markets map[string]struct {
		Source struct {
			Repo string `json:"repo"`
			URL  string `json:"url"`
		} `json:"source"`
	}
	readJSON(filepath.Join(claudePluginsDir(a.home), "known_marketplaces.json"), &markets)
	before := readClaudePlugins(a.home)

	var rows []row
	mr := row{st: stCurrent, name: "marketplaces", version: fmt.Sprintf("%d", len(markets)), via: "claude plugin marketplace"}
	used := map[string]bool{}
	for id := range before {
		used[id[strings.LastIndex(id, "@")+1:]] = true
	}
	var unused []string
	for _, name := range sortedKeys(markets) {
		if !used[name] {
			unused = append(unused, name)
		}
	}
	mr.note = strings.Join(sortedKeys(markets), ", ")
	if len(unused) > 0 {
		mr.addNote("no plugins installed from: " + strings.Join(unused, ", "))
	}
	a.mutate(&mr, quick, claude, "plugin", "marketplace", "update")
	rows = append(rows, mr)

	if before == nil {
		return append(rows, row{st: stWarn, name: "plugins", note: "~/.claude/plugins/installed_plugins.json unreadable"})
	}
	first := len(rows)
	for _, id := range sortedKeys(before) {
		name, market, _ := strings.Cut(id, "@")
		r := row{st: stCurrent, name: name, version: pluginVersion(before[id]), via: market}
		args := []string{"plugin", "update", id}
		if a.yes {
			args = append(args, "-y")
		}
		if a.mutate(&r, quick, claude, args...) && !a.dryRun {
			after := readClaudePlugins(a.home)[id]
			r.version = versionChange(r.version, pluginVersion(after))
			if pluginFingerprint(after) != pluginFingerprint(before[id]) {
				r.raise(stUpdated)
			}
		}
		if a.dryRun {
			r.detail = nil
		}
		rows = append(rows, r)
	}
	if a.dryRun && len(rows) > first {
		rows[first].detail = []string{"↳ claude plugin update <plugin>@<marketplace>, for each plugin"}
	}
	return append(rows, a.claudePrune(claude))
}

// claudePrune removes auto-installed plugin dependencies nothing needs anymore.
// Without -y it only reports (`prune --dry-run`); with -y it prunes unless Claude is running.
func (a *app) claudePrune(claude string) row {
	r := row{st: stCurrent, name: "orphaned deps", via: "claude plugin prune"}
	if !a.yes || a.dryRun {
		out := strings.TrimSpace(read(claude, "plugin", "prune", "--dry-run"))
		r.note = firstLine(out)
		if out != "" && !strings.HasPrefix(out, "Nothing to prune") {
			r.raise(stWarn)
			r.addNote("rerun with -y to remove")
		}
		return r
	}
	if isProcessRunning("claude") {
		r.raise(stWarn)
		r.note = "skipped: a Claude Code session is running"
		return r
	}
	if a.mutate(&r, quick, claude, "plugin", "prune", "-y") {
		r.note = "pruned"
	}
	return r
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// ---- Codex ----------------------------------------------------------------

func (a *app) codexPluginRows() []row {
	codex := findCodex(a.home)
	if codex == "" {
		return nil
	}
	var markets struct {
		Marketplaces []struct {
			Name   string `json:"name"`
			Source *struct {
				Type string `json:"sourceType"`
			} `json:"marketplaceSource"`
		} `json:"marketplaces"`
	}
	json.Unmarshal([]byte(read(codex, "plugin", "marketplace", "list", "--json")), &markets)

	sourceOf := map[string]string{}
	var names, git []string
	for _, m := range markets.Marketplaces {
		names = append(names, m.Name)
		switch {
		case m.Source == nil:
			sourceOf[m.Name] = "OpenAI"
		case m.Source.Type == "git":
			sourceOf[m.Name] = "git"
			git = append(git, m.Name)
		default:
			sourceOf[m.Name] = "Codex.app"
		}
	}

	mr := row{st: stInfo, name: "marketplaces", version: fmt.Sprintf("%d", len(names)), via: "codex plugin marketplace", note: strings.Join(names, ", ")}
	if len(git) > 0 {
		mr.st = stCurrent
		mr.addNote("git: " + strings.Join(git, ", "))
		a.mutate(&mr, quick, codex, "plugin", "marketplace", "upgrade")
	}
	rows := []row{mr}

	// Codex installs plugins straight from marketplace snapshots: app-bundled ones update with
	// Codex itself, git ones with the marketplace upgrade above.
	var list struct {
		Installed []struct {
			Name        string `json:"name"`
			Marketplace string `json:"marketplaceName"`
		} `json:"installed"`
	}
	a.progress("codex plugin list")
	json.Unmarshal([]byte(read(codex, "plugin", "list", "--json")), &list)
	byMarket := map[string][]string{}
	for _, p := range list.Installed {
		byMarket[p.Marketplace] = append(byMarket[p.Marketplace], p.Name)
	}
	for _, m := range sortedKeys(byMarket) {
		src := sourceOf[m]
		if src == "" {
			src = "OpenAI"
		}
		r := row{st: stInfo, name: m, version: plural(len(byMarket[m]), "plugin"), via: src, note: strings.Join(byMarket[m], ", ")}
		if src == "git" {
			r.st = mr.st
		}
		rows = append(rows, r)
	}
	return rows
}

// ---- omp ------------------------------------------------------------------

// parseOmpMarketplaces reads `omp plugin marketplace list` ("  <name>  <source>" lines).
func parseOmpMarketplaces(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); strings.HasPrefix(line, "  ") && len(f) >= 2 {
			names = append(names, f[0])
		}
	}
	return names
}

type ompPlugin struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

func (a *app) ompPluginRows() []row {
	omp := findOmp(a.home)
	if omp == "" {
		return nil
	}
	markets := parseOmpMarketplaces(read(omp, "plugin", "marketplace", "list"))
	mr := row{st: stCurrent, name: "marketplaces", version: fmt.Sprintf("%d", len(markets)), via: "omp plugin marketplace", note: strings.Join(markets, ", ")}
	if len(markets) > 0 {
		a.mutate(&mr, quick, omp, "plugin", "marketplace", "update")
	}
	rows := []row{mr}

	list := func() (map[string]ompPlugin, map[string]string) {
		var l map[string][]ompPlugin // {"npm": [...], "marketplace": [...]}
		json.Unmarshal([]byte(read(omp, "plugin", "list", "--json")), &l)
		plugins, kind := map[string]ompPlugin{}, map[string]string{}
		for k, ps := range l {
			for _, p := range ps {
				name := p.Name
				if name == "" {
					name = p.ID
				}
				plugins[name], kind[name] = p, k
			}
		}
		return plugins, kind
	}
	before, kind := list()
	if len(before) == 0 {
		return append(rows, row{st: stInfo, name: "plugins", note: "none installed"})
	}

	ur := row{}
	ok := a.mutate(&ur, quick, omp, "plugin", "upgrade")
	after := before
	if ok && !a.dryRun {
		after, _ = list()
	}
	for _, name := range sortedKeys(before) {
		r := row{st: stCurrent, name: name, version: versionChange(before[name].Version, after[name].Version), via: kind[name]}
		r.raise(ur.st)
		if after[name].Version != before[name].Version {
			r.raise(stUpdated)
		}
		rows = append(rows, r)
	}
	rows[1].detail = ur.detail
	return rows
}
