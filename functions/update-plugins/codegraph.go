package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const codegraphMiseTool = "npm:@colbymchenry/codegraph"

// codegraphWiring lists where each agent registers the codegraph MCP server.
// omp is wired by hand (dotfiles mcp.json); `codegraph install` has no omp target.
func codegraphWiring(home string) []struct{ agent, label, file string } {
	return []struct{ agent, label, file string }{
		{"claude", "claude", filepath.Join(home, ".claude.json")},
		{"codex", "codex", filepath.Join(home, ".codex", "config.toml")},
		{"gemini", "gemini-cli", filepath.Join(home, ".gemini", "settings.json")},
		{"gemini", "antigravity", filepath.Join(home, ".gemini", "config", "mcp_config.json")},
		{"omp", "omp (manual)", filepath.Join(home, ".omp", "agent", "mcp.json")},
	}
}

func hasCodegraphMCP(file string) bool {
	if strings.HasSuffix(file, ".toml") {
		data, err := os.ReadFile(file)
		return err == nil && strings.Contains(string(data), "[mcp_servers.codegraph]")
	}
	var cfg struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	return readJSON(file, &cfg) && cfg.MCPServers["codegraph"] != nil
}

var refreshedLine = regexp.MustCompile(`(?m)^\s*(.+?): refreshed `)

// refreshedAgents parses `codegraph install --refresh` output ("  Claude Code: refreshed <path>").
func refreshedAgents(out string) []string {
	var agents []string
	for _, m := range refreshedLine.FindAllStringSubmatch(out, -1) {
		if !slices.Contains(agents, m[1]) {
			agents = append(agents, m[1])
		}
	}
	return agents
}

// codegraph: upgrade through whatever installed it (mise here; `codegraph upgrade` handles its own
// bundle/npm installs but would `npm i -g` a second copy over a mise one), then
// `codegraph install --refresh -l global` rewrites the MCP entries and instruction blocks of every
// already-configured agent. The refresh writes Antigravity's entry as a versioned mise install path,
// which breaks once mise prunes that version after a plain `mise up`; it is pinned back to the
// mise shim, which always resolves to the configured version.
func (a *app) codegraph() (row, bool) {
	bin := findExecutable(a.home, "codegraph")
	if bin == "" {
		return row{}, false
	}
	version := func() string { return strings.TrimPrefix(lastField(read(bin, "--version")), "v") }
	before := version()
	r := row{st: stCurrent, name: "codegraph", version: before, via: tilde(a.home, bin)}

	ok := true
	shim := ""
	real, _ := filepath.EvalSymlinks(bin)
	if mise := findExecutable(a.home, "mise", "/opt/homebrew/bin/mise", "~/.local/bin/mise"); strings.Contains(real, "/mise/installs/") && mise != "" {
		r.via = "mise (npm)"
		if p := filepath.Join(miseDataDir(a.home), "shims", "codegraph"); exists(p) {
			shim = p
		}
		ok = a.mutate(&r, slow, mise, "upgrade", codegraphMiseTool)
	} else {
		ok = a.mutate(&r, slow, bin, "upgrade")
	}
	if a.dryRun {
		if m := regexp.MustCompile(`latest v?(\S+)`).FindStringSubmatch(read(bin, "upgrade", "--check")); m != nil && m[1] != before {
			r.addNote("latest " + m[1])
		}
	} else if after := version(); after != before {
		r.version = versionChange(before, after)
		r.raise(stUpdated)
	}

	var wired []string
	for _, w := range codegraphWiring(a.home) {
		if a.agents[w.agent] && hasCodegraphMCP(w.file) {
			wired = append(wired, w.label)
		}
	}
	r.addNote("MCP → " + strings.Join(wired, ", "))
	if !ok {
		return r, true
	}
	// The refresh is global: it covers every agent codegraph configured, regardless of --agents.
	agy := filepath.Join(a.home, ".gemini", "config", "mcp_config.json")
	agyBefore := codegraphEntry(agy)
	refreshed, out := a.mutateOut(&r, quick, bin, "install", "--refresh", "-l", "global")
	if shim != "" {
		if a.dryRun {
			r.detail = append(r.detail, "↳ pin antigravity's codegraph command to the mise shim")
		} else if err := pinCommand(agy, shim); err != nil {
			r.raise(stWarn)
			r.addNote("antigravity: " + err.Error())
		}
	}
	if refreshed && !a.dryRun {
		agents := slices.DeleteFunc(refreshedAgents(out), func(s string) bool { return s == "Antigravity IDE" })
		if codegraphEntry(agy) != agyBefore {
			agents = append(agents, "Antigravity IDE")
		}
		if len(agents) > 0 {
			r.raise(stUpdated)
			r.addNote("refreshed " + strings.Join(agents, ", "))
		}
	}
	return r, true
}

func miseDataDir(home string) string {
	if d := os.Getenv("MISE_DATA_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "mise")
	}
	return filepath.Join(home, ".local", "share", "mise")
}

// codegraphEntry returns the codegraph MCP entry of a JSON config, re-encoded for comparison.
func codegraphEntry(file string) string {
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if !readJSON(file, &cfg) {
		return ""
	}
	var v any
	if json.Unmarshal(cfg.MCPServers["codegraph"], &v) != nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// pinCommand points the codegraph entry's command at path, editing only that string so the
// rest of the file keeps its formatting. No-op when the entry is absent or already pinned.
func pinCommand(file, path string) error {
	var cfg struct {
		MCPServers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if !readJSON(file, &cfg) {
		return nil
	}
	cur := cfg.MCPServers["codegraph"].Command
	if cur == "" || cur == path {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	old, _ := json.Marshal(cur)
	repl, _ := json.Marshal(path)
	updated := strings.Replace(string(data), `"command": `+string(old), `"command": `+string(repl), 1)
	if updated == string(data) {
		return fmt.Errorf("could not locate the codegraph command in %s", file)
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	return os.WriteFile(file, []byte(updated), info.Mode().Perm())
}
