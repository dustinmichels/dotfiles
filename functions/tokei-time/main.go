package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	version = "1.0.0"
)

func printHelp() {
	helpText := `tokei-time: interactive git code metrics over time

USAGE:
    tokei-time [OPTIONS] [PATH]

ARGUMENTS:
    [PATH]                     Path to repository or directory (default: ".")

OPTIONS:
    -n, --commits <COUNT>      Number of commits to analyze (default: 0 = all history with lazy-loading)
    -g, --group <MODE>         Summary grouping: commit, day, week, month, year (default: commit)
    -m, --metric <METRIC>      Initial metric to display: code, lines, files, comments, blanks (default: code)
    -v, --version              Show version
    -h, --help                 Show this help message

KEYBOARD SHORTCUTS:
    Tab / Shift+Tab            Cycle through metrics (Files, Lines, Code, Comments, Blanks)
    1, 2, 3, 4, 5              Jump directly to: 1=Code, 2=Lines, 3=Files, 4=Comments, 5=Blanks
    s / g (or S / G)           Toggle summary stats: Commits ↔ Day ↔ Week ↔ Month ↔ Year (averaging period)
    ↑ / ↓ or k / j             Navigate commits or summary periods
    ← / → or h / l             Navigate in vertical chart mode
    v                          Toggle view (Horizontal Stacked Bars vs Vertical Timeline Chart)
    c / d                      Toggle diff mode (Δ vs Previous vs Δ vs Latest)
    + / -                      Increase or decrease number of commits analyzed (±10)
    a                          Load all commits in repository history
    r                          Refresh data from repository
    q / Esc / Ctrl+C           Quit
`
	fmt.Print(helpText)
}

func main() {
	var (
		limitFlag   int
		groupFlag   string
		metricFlag  string
		versionFlag bool
		helpFlag    bool
	)

	flag.IntVar(&limitFlag, "n", 0, "Number of commits to analyze (0 for all history)")
	flag.IntVar(&limitFlag, "commits", 0, "Number of commits to analyze (0 for all history)")
	flag.StringVar(&groupFlag, "g", "commit", "Grouping mode (commit, day, week, month, year)")
	flag.StringVar(&groupFlag, "group", "commit", "Grouping mode (commit, day, week, month, year)")
	flag.StringVar(&metricFlag, "m", "code", "Initial metric (code, lines, files, comments, blanks)")
	flag.StringVar(&metricFlag, "metric", "code", "Initial metric (code, lines, files, comments, blanks)")
	flag.BoolVar(&versionFlag, "v", false, "Show version")
	flag.BoolVar(&versionFlag, "version", false, "Show version")
	flag.BoolVar(&helpFlag, "h", false, "Show help")
	flag.BoolVar(&helpFlag, "help", false, "Show help")

	flag.Usage = printHelp
	flag.Parse()

	if helpFlag {
		printHelp()
		os.Exit(0)
	}

	if versionFlag {
		fmt.Printf("tokei-time v%s\n", version)
		os.Exit(0)
	}

	// Target directory
	targetDir := "."
	if flag.NArg() > 0 {
		targetDir = flag.Arg(0)
	}

	// 1. Verify git is installed
	if err := CheckGitInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 2. Verify tokei is installed
	if err := CheckTokeiInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 3. Find git root
	repoRoot, err := GetGitRoot(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 4. Initial data collection with fast startup (initial batch of 25 commits)
	initialBatch := 25
	if limitFlag > 0 && limitFlag < initialBatch {
		initialBatch = limitFlag
	}
	if limitFlag <= 0 {
		fmt.Printf("Analyzing %s with tokei (all commits, streaming history)...\n", repoRoot)
	} else {
		fmt.Printf("Analyzing %s with tokei (fetching %d commits)...\n", repoRoot, limitFlag)
	}
	initResult, err := CollectInitialHistory(repoRoot, limitFlag, initialBatch, func(done, total int) {
		fmt.Printf("\rScanning initial commits: %d/%d...", done, total)
	})
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing repository: %v\n", err)
		os.Exit(1)
	}

	if len(initResult.InitialSnapshots) == 0 {
		fmt.Fprintf(os.Stderr, "No commits found in %s\n", repoRoot)
		os.Exit(1)
	}

	// 5. Setup model & metric
	initialGroup, err := ParseGroupMode(groupFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v, defaulting to commit\n", err)
		initialGroup = GroupCommit
	}
	m := NewModelWithLazyLoading(repoRoot, initResult, limitFlag, initialGroup)
	switch strings.ToLower(metricFlag) {
	case "files":
		m.activeMetric = MetricFiles
	case "lines":
		m.activeMetric = MetricLines
	case "code":
		m.activeMetric = MetricCode
	case "comments":
		m.activeMetric = MetricComments
	case "blanks":
		m.activeMetric = MetricBlanks
	}

	// 6. Run Bubble Tea
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
