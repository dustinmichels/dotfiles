package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustinmichels/repo-ls/git"
	"github.com/dustinmichels/repo-ls/models"
	"github.com/dustinmichels/repo-ls/ui"
)

var (
	version = "1.0.0"
)

func printHelp() {
	help := `repo-ls: interactive git repository explorer & manager

USAGE:
  repo-ls [path] [flags]

ARGUMENTS:
  [path]       Directory to scan (default: current directory, or ~/GitRepos if run from ~)

FLAGS:
  -v, --version    Show version
  -h, --help       Show help
  -d, --depth      Maximum directory depth to search (default: 6)
  -p, --plain      Print plain text list of repositories and exit (non-interactive)

EXAMPLES:
  repo-ls                  # Scan current directory (or ~/GitRepos)
  repo-ls ~/GitRepos       # Scan specific directory
  repo-ls --plain          # Scriptable non-interactive output
`
	fmt.Print(help)
}

func main() {
	var (
		helpFlag    bool
		versionFlag bool
		depthFlag   int
		plainFlag   bool
	)

	flag.BoolVar(&helpFlag, "h", false, "Show help")
	flag.BoolVar(&helpFlag, "help", false, "Show help")
	flag.BoolVar(&versionFlag, "v", false, "Show version")
	flag.BoolVar(&versionFlag, "version", false, "Show version")
	flag.IntVar(&depthFlag, "d", 6, "Maximum directory scan depth")
	flag.IntVar(&depthFlag, "depth", 6, "Maximum directory scan depth")
	flag.BoolVar(&plainFlag, "p", false, "Plain text list output")
	flag.BoolVar(&plainFlag, "plain", false, "Plain text list output")

	flag.Usage = printHelp
	flag.Parse()

	if helpFlag {
		printHelp()
		os.Exit(0)
	}

	if versionFlag {
		fmt.Printf("repo-ls v%s\n", version)
		os.Exit(0)
	}

	// Determine root directory
	targetDir := "."
	if flag.NArg() > 0 {
		targetDir = flag.Arg(0)
	}

	// Expand ~ if present
	if strings.HasPrefix(targetDir, "~/") || targetDir == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if targetDir == "~" {
				targetDir = home
			} else {
				targetDir = filepath.Join(home, targetDir[2:])
			}
		}
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path %s: %v\n", targetDir, err)
		os.Exit(1)
	}

	// If run from home directory without args, default to ~/GitRepos if it exists
	home, _ := os.UserHomeDir()
	if flag.NArg() == 0 && absTarget == home {
		gitReposPath := filepath.Join(home, "GitRepos")
		if info, err := os.Stat(gitReposPath); err == nil && info.IsDir() {
			absTarget = gitReposPath
		}
	}

	// Scan repositories
	repos, err := git.ScanRepositories(absTarget, depthFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning repositories in %s: %v\n", absTarget, err)
		os.Exit(1)
	}

	if len(repos) == 0 {
		fmt.Printf("No git repositories found in %s (depth %d).\n", absTarget, depthFlag)
		os.Exit(0)
	}

	// Non-interactive plain mode
	// Non-interactive plain mode
	if plainFlag {
		git.EnrichGitHubVisibility(repos)
		fmt.Printf("%-3s  %-30s  %-12s  %-10s  %-12s  %-12s  %s\n", "ST", "PATH", "BRANCH", "VISIBILITY", "CREATED", "MODIFIED", "REMOTE")
		fmt.Println(strings.Repeat("─", 110))
		for _, r := range repos {
			st := "✓"
			if r.HasUncommitted {
				st = "●"
			}
			vis := string(r.GitHubVisibility)
			createdRel := models.RelativeTime(r.DateCreated)
			modifiedRel := models.RelativeTime(r.LastModified)
			fmt.Printf("%-3s  %-30s  %-12s  %-10s  %-12s  %-12s  %s\n", st, r.RelPath, r.Branch, vis, createdRel, modifiedRel, r.RemoteURL)
		}
		return
	}

	// Run interactive TUI
	m := ui.NewModel(absTarget, repos)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running repo-ls: %v\n", err)
		os.Exit(1)
	}
}
