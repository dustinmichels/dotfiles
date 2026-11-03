package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	quick = 2 * time.Minute
	slow  = 5 * time.Minute // npx fetches, skills.sh repo clones
)

var nonInteractiveEnv = []string{"GIT_TERMINAL_PROMPT=0", "CI=1", "DEBIAN_FRONTEND=noninteractive"}

// run executes a command with empty stdin, capturing combined output (streamed too with -v).
func (a *app) run(timeout time.Duration, bin string, args ...string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), nonInteractiveEnv...)
	var buf bytes.Buffer
	var w io.Writer = &buf
	if a.verbose {
		fmt.Println(dimStyle.Render("$ " + cmdLine(bin, args)))
		w = io.MultiWriter(os.Stdout, &buf)
	}
	cmd.Stdout = w
	cmd.Stderr = w

	err := cmd.Run()
	switch {
	case err == nil:
		return 0, buf.String()
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return 124, buf.String() + "\ntimed out after " + timeout.String()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), buf.String()
	}
	return 1, err.Error()
}

// read runs a read-only query command and returns trimmed stdout, or "" on failure.
func read(bin string, args ...string) string {
	if bin == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), nonInteractiveEnv...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// mutate runs a state-changing command for row r. In dry-run it only records the command.
// On failure it marks the row failed with the tail of the command output.
func (a *app) mutate(r *row, timeout time.Duration, bin string, args ...string) bool {
	ok, _ := a.mutateOut(r, timeout, bin, args...)
	return ok
}

// mutateOut is mutate that also returns the command's combined output ("" in dry-run).
func (a *app) mutateOut(r *row, timeout time.Duration, bin string, args ...string) (bool, string) {
	line := cmdLine(bin, args)
	if a.dryRun {
		r.detail = append(r.detail, "↳ "+line)
		r.raise(stPlanned)
		return true, ""
	}
	a.progress(r.name + ": " + line)
	rc, out := a.run(timeout, bin, args...)
	if rc == 0 {
		return true, out
	}
	r.raise(stFailed)
	r.detail = append(r.detail, fmt.Sprintf("%s → exit %d", line, rc))
	r.detail = append(r.detail, tailLines(out, 4)...)
	return false, out
}

func cmdLine(bin string, args []string) string {
	return strings.Join(append([]string{filepath.Base(bin)}, args...), " ")
}

func tailLines(s string, n int) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// progress shows a transient status line while a slow step runs (TTY only).
func (a *app) progress(msg string) {
	if !a.tty || a.verbose {
		return
	}
	if w := a.width - 6; w > 10 && len(msg) > w {
		msg = msg[:w-1] + "…"
	}
	fmt.Printf("\r\033[K  %s", dimStyle.Render("⋯ "+msg))
	a.progressShown = true
}

func (a *app) clearProgress() {
	if a.progressShown {
		fmt.Print("\r\033[K")
		a.progressShown = false
	}
}

func isProcessRunning(name string) bool {
	return exec.Command("pgrep", "-x", name).Run() == nil
}
