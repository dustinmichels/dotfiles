package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Agent directory paths relative to user home directory
func CanonicalSkillsHub(homeDir string) string {
	return filepath.Join(homeDir, ".agents", "skills")
}

func ClaudeSkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "skills")
}

func PiSkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".pi", "agent", "skills")
}

func GeminiSkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "config", "skills")
}

func OmpSkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".omp", "agent", "skills")
}

func ClaudePluginsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "plugins")
}

func CodexConfigFile(homeDir string) string {
	return filepath.Join(homeDir, ".codex", "config.toml")
}

func CodexMarketplacesDir(homeDir string) string {
	return filepath.Join(homeDir, ".codex", ".tmp", "marketplaces")
}

func OmpAgentDir(homeDir string) string {
	return filepath.Join(homeDir, ".omp", "agent")
}

// expandPath expands leading ~ to homeDir
func expandPath(homeDir, p string) string {
	if p == "~" {
		return homeDir
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir, p[2:])
	}
	return p
}

// findExecutable checks PATH first, then checks explicit fallback paths
func findExecutable(name string, fallbacks []string, homeDir string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, fb := range fallbacks {
		expanded := expandPath(homeDir, fb)
		if fi, err := os.Stat(expanded); err == nil && !fi.IsDir() {
			return expanded
		}
	}
	return ""
}

func findCodex(homeDir string) string {
	return findExecutable("codex", []string{
		"/Applications/Codex.app/Contents/Resources/codex",
		"~/.local/bin/codex",
	}, homeDir)
}

func findOmp(homeDir string) string {
	return findExecutable("omp", []string{
		"~/.bun/bin/omp",
		"~/.local/bin/omp",
	}, homeDir)
}

func findClaude(homeDir string) string {
	return findExecutable("claude", []string{
		"~/.local/bin/claude",
		"~/.npm-global/bin/claude",
	}, homeDir)
}

func findUV(homeDir string) string {
	return findExecutable("uv", []string{
		"~/.cargo/bin/uv",
		"~/.local/bin/uv",
	}, homeDir)
}

func findNPX(homeDir string) string {
	return findExecutable("npx", []string{
		"/usr/local/bin/npx",
		"~/.nvm/current/bin/npx",
	}, homeDir)
}

func findBrowserUse(homeDir string) string {
	return findExecutable("browser-use", []string{
		"~/.local/bin/browser-use",
	}, homeDir)
}

// runCmd executes a command with standard environment variables, non-interactive stdin, and timeout
func runCmd(name string, args []string, stdin io.Reader, dryRun bool) (int, int) {
	if dryRun {
		fmt.Printf("   %s %s %s\n",
			mutedColorTag("⚡ dry-run: would run:"),
			name,
			strings.Join(args, " "),
		)
		return 0, 0
	}

	if stdin == nil {
		stdin = strings.NewReader("")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tStart := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"CI=1",
		"DEBIAN_FRONTEND=noninteractive",
	)

	err := cmd.Run()
	tElapsed := int(time.Since(tStart).Seconds())
	rc := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		} else {
			rc = 1
		}
	}
	return rc, tElapsed
}

// isProcessRunning checks if an exact process name is currently running
func isProcessRunning(name string) bool {
	cmd := exec.Command("pgrep", "-x", name)
	return cmd.Run() == nil
}
