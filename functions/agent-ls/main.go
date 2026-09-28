package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustinmichels/agent-ls/models"
	"github.com/dustinmichels/agent-ls/providers"
	"github.com/dustinmichels/agent-ls/storage"
	"github.com/dustinmichels/agent-ls/ui"
	"golang.org/x/term"
)

func parseDaysDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func printSummary(summaries []models.AgentSummary, toolFilter string) {
	fmt.Printf("%-14s  %10s  %12s  %12s  %14s\n", "AGENT", "SESSIONS", "ALLOCATED", ">14D SESSIONS", ">14D RECLAIMABLE")
	fmt.Println(strings.Repeat("─", 70))
	var totCnt, totOldCnt int
	var totBytes, totOldBytes int64

	for _, s := range summaries {
		if toolFilter != "" && toolFilter != "all" && s.Tool != toolFilter {
			continue
		}
		totCnt += s.TotalCount
		totBytes += s.TotalBytes
		totOldCnt += s.OldCount
		totOldBytes += s.OldBytes

		fmt.Printf("%-14s  %10d  %12s  %12d  %14s\n",
			s.Tool, s.TotalCount, storage.FormatBytes(s.TotalBytes),
			s.OldCount, storage.FormatBytes(s.OldBytes))
	}
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%-14s  %10d  %12s  %12d  %14s\n",
		"TOTAL", totCnt, storage.FormatBytes(totBytes),
		totOldCnt, storage.FormatBytes(totOldBytes))
}

func printPlain(sessions []models.Session, now time.Time, cutoff time.Time, toolFilter string, olderOnly bool) {
	fmt.Printf("%-12s  %-16s  %-42s  %10s  %10s  %s\n",
		"TOOL", "PROJECT", "TITLE", "SIZE", "AGE", "STATUS")
	fmt.Println(strings.Repeat("─", 100))

	for _, s := range sessions {
		if toolFilter != "" && toolFilter != "all" && s.Tool != toolFilter {
			continue
		}
		if olderOnly && !s.IsOlderThan(cutoff) {
			continue
		}

		toolName := s.Tool
		if s.IsArchived {
			toolName = s.Tool + " (arch)"
		}

		status := "active"
		if s.IsLive {
			status = "LIVE"
		} else if s.IsOlderThan(cutoff) {
			status = ">14d"
		}

		title := s.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		fmt.Printf("%-12s  %-16s  %-42s  %10s  %10s  %s\n",
			toolName, s.Project, title,
			storage.FormatBytes(s.AllocatedBytes),
			storage.FormatDuration(s.Age(now)),
			status)
	}
}

func runCleanCmd(args []string) {
	cleanCmd := flag.NewFlagSet("clean", flag.ExitOnError)
	flagOlderThan := cleanCmd.String("older-than", "14d", "Age threshold for old conversations (e.g. 14d, 30d)")
	flagTool := cleanCmd.String("tool", "all", "Filter to specific tool (omp, antigravity, claude, codex)")
	flagDryRun := cleanCmd.Bool("dry-run", false, "Simulate cleanup without moving files")
	flagN := cleanCmd.Bool("n", false, "Simulate cleanup without moving files (shorthand)")

	cleanCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: agent-ls clean [OPTIONS]\n\n"+
			"Batch move conversations older than threshold to ~/.Trash/agent-ls/\n\n"+
			"Options:\n")
		cleanCmd.PrintDefaults()
	}

	_ = cleanCmd.Parse(args)

	dur, err := parseDaysDuration(*flagOlderThan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing --older-than: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	cutoff := now.Add(-dur)

	mgr := providers.NewManager()
	sessions, err := mgr.ScanAll("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning sessions: %v\n", err)
		os.Exit(1)
	}

	dryRun := *flagDryRun || *flagN
	var toDelete []models.Session
	var reclaimBytes int64
	hasOmp := false

	for _, s := range sessions {
		if s.Tool != "omp" && s.Tool != "antigravity" {
			continue
		}
		if *flagTool != "" && *flagTool != "all" && s.Tool != *flagTool {
			continue
		}
		if s.IsOlderThan(cutoff) {
			if s.IsLive {
				fmt.Printf("Skipping live session %s (%s)\n", s.ID, s.Tool)
				continue
			}
			toDelete = append(toDelete, s)
			reclaimBytes += s.AllocatedBytes
			if s.Tool == "omp" {
				hasOmp = true
			}
		}
	}
	if dryRun {
		fmt.Printf("DRY RUN: would move %d conversations to ~/.Trash/agent-ls/ (%s reclaimable, older than %s):\n",
			len(toDelete), storage.FormatBytes(reclaimBytes), *flagOlderThan)
		for _, s := range toDelete {
			fmt.Printf("  • [%s] %-14s %-30s %s (%s)\n",
				s.Tool, s.Project, s.Title, storage.FormatBytes(s.AllocatedBytes), storage.FormatDuration(s.Age(now)))
		}
		return
	}

	fmt.Printf("Moving %d conversations to ~/.Trash/agent-ls/ (%s)...\n",
		len(toDelete), storage.FormatBytes(reclaimBytes))

	deletedCount := 0
	var freedBytes int64
	for _, s := range toDelete {
		if err := mgr.DeleteSession(s); err != nil {
			fmt.Fprintf(os.Stderr, "  Error trashing %s: %v\n", s.ID, err)
		} else {
			deletedCount++
			freedBytes += s.AllocatedBytes
		}
	}

	if hasOmp {
		fmt.Println("Running omp gc to sweep unreferenced blobs...")
		_ = providers.RunOmpGC()
	}

	fmt.Printf("✨ Successfully moved %d sessions to Trash (%s reclaimable — freed when Trash is emptied)!\n",
		deletedCount, storage.FormatBytes(freedBytes))
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "clean" {
		runCleanCmd(os.Args[2:])
		return
	}

	var (
		flagSummary   = flag.Bool("summary", false, "Print agent summary table and exit")
		flagPlain     = flag.Bool("plain", false, "Output plain text table without TUI")
		flagP         = flag.Bool("p", false, "Output plain text table without TUI (shorthand)")
		flagJSON      = flag.Bool("json", false, "Output JSON without TUI")
		flagJ         = flag.Bool("j", false, "Output JSON without TUI (shorthand)")
		flagTool      = flag.String("tool", "all", "Filter to specific tool (omp, antigravity, claude, codex)")
		flagOlderThan = flag.String("older-than", "14d", "Age threshold for old conversations (e.g. 14d, 30d)")
		flagOlderOnly = flag.Bool("older-only", false, "Show only conversations older than the age threshold")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: agent-ls [OPTIONS] [COMMAND]\n\n"+
			"Browse and clean AI agent sessions across Oh My Pi, Antigravity, Claude, and Codex.\n\n"+
			"Commands:\n"+
			"  (default)           Launch interactive TUI browser & cleaner\n"+
			"  clean               Batch move conversations older than threshold to ~/.Trash/agent-ls/\n\n"+
			"Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	dur, err := parseDaysDuration(*flagOlderThan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing --older-than: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	cutoff := now.Add(-dur)

	mgr := providers.NewManager()
	sessions, err := mgr.ScanAll("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning sessions: %v\n", err)
		os.Exit(1)
	}

	// 1. Summary flag
	if *flagSummary {
		summaries := mgr.Summaries(sessions, cutoff)
		printSummary(summaries, *flagTool)
		return
	}

	// 2. JSON flag
	if *flagJSON || *flagJ {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(sessions)
		return
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// 3. Plain text flag or non-TTY pipe
	if *flagPlain || *flagP || !isTTY {
		printPlain(sessions, now, cutoff, *flagTool, *flagOlderOnly)
		return
	}

	// 4. Interactive Bubble Tea TUI
	p := tea.NewProgram(
		ui.NewModel(mgr, sessions, *flagTool),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
