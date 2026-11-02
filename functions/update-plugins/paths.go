package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func hubDir(home string) string           { return filepath.Join(home, ".agents", "skills") }
func skillsLockFile(home string) string   { return filepath.Join(home, ".agents", ".skill-lock.json") }
func claudePluginsDir(home string) string { return filepath.Join(home, ".claude", "plugins") }
func ompSkillsDir(home string) string     { return filepath.Join(home, ".omp", "agent", "skills") }
func hindsightRuntime(home string) string {
	return filepath.Join(home, ".hindsight", "coding-agents")
}

// spokeDirs are agent skill dirs that get one symlink per hub skill.
// omp links its whole skills dir to the hub instead (see linkOmpSkills).
func spokeDirs(home string) []struct{ agent, dir string } {
	return []struct{ agent, dir string }{
		{"claude", filepath.Join(home, ".claude", "skills")},
		{"pi", filepath.Join(home, ".pi", "agent", "skills")},
		{"gemini", filepath.Join(home, ".gemini", "config", "skills")},
	}
}

// skillRoots are every skill dir that installers (skills.sh, browser-use, hindsight) write into.
func skillRoots(home string) []struct{ label, dir string } {
	return []struct{ label, dir string }{
		{"hub", hubDir(home)},
		{"claude", filepath.Join(home, ".claude", "skills")},
		{"pi", filepath.Join(home, ".pi", "agent", "skills")},
		{"gemini", filepath.Join(home, ".gemini", "config", "skills")},
		{"gemini-cli", filepath.Join(home, ".gemini", "skills")},
		{"codex", filepath.Join(home, ".codex", "skills")},
		{"copilot", filepath.Join(home, ".copilot", "skills")},
		{"cursor", filepath.Join(home, ".cursor", "skills")},
		{"openclaw", filepath.Join(home, ".openclaw", "skills")},
		{"opencode", filepath.Join(home, ".config", "opencode", "skills")},
	}
}

func findExecutable(home, name string, fallbacks ...string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, fb := range fallbacks {
		if strings.HasPrefix(fb, "~/") {
			fb = filepath.Join(home, fb[2:])
		}
		if fi, err := os.Stat(fb); err == nil && !fi.IsDir() {
			return fb
		}
	}
	return ""
}

func findClaude(home string) string {
	return findExecutable(home, "claude", "~/.local/bin/claude", "~/.npm-global/bin/claude")
}
func findCodex(home string) string {
	return findExecutable(home, "codex", "/Applications/Codex.app/Contents/Resources/codex", "~/.local/bin/codex")
}
func findOmp(home string) string {
	return findExecutable(home, "omp", "~/.bun/bin/omp", "~/.local/bin/omp")
}
func findUV(home string) string {
	return findExecutable(home, "uv", "~/.cargo/bin/uv", "~/.local/bin/uv")
}
func findNPX(home string) string {
	return findExecutable(home, "npx", "/usr/local/bin/npx", "~/.nvm/current/bin/npx")
}
func findBrowserUse(home string) string {
	return findExecutable(home, "browser-use", "~/.local/bin/browser-use")
}

func tilde(home, p string) string {
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, v) == nil
}

// dirHash fingerprints a directory tree (paths + contents), following a symlinked root.
func dirHash(dir string) string {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return ""
	}
	h := sha256.New()
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// copyTree copies src over dst, overwriting files (same semantics as the installers' cpSync).
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
