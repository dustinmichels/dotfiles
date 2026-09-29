# tokei-time

Interactive git code metrics over time powered by `tokei`.

Count files, lines, code, comments, and blanks across programming languages, travel back through git commit history, compare diffs to previous commits or the latest working tree, and visualize everything in an interactive stacked bar chart in your terminal.

## Features

- **Time Travel**: Inspect code metrics across recent commits or uncommitted working tree changes.
- **Summary & Aggregation Modes**:
  - Toggle between **Individual Commits** and **Summary Stats** grouped by **Day**, **Week**, **Month**, or **Year**.
  - When summary mode is active, displays the **average across all commits** within each time period.
  - Computes period-to-period deltas ($\Delta \text{ Prev}$) and drift from the latest period ($\Delta \text{ Latest}$).
- **Diff Comparison**: Compare the diff introduced by each commit/period vs. its predecessor, or measure cumulative drift against the latest state.
- **Interactive Stacked Bar Graph**:
  - Proportional multi-color stacked bars colored by programming language.
  - Dual visualization modes:
    - **Horizontal List View**: Timeline with hashes/period labels, commit counts, relative spans, stacked bars, metric averages/totals, and delta indicators.
    - **Vertical Timeline Chart**: ASCII/Unicode chart showing code growth curves over time with language proportions.
- **Tab Metric Switching**: Use `Tab` or `Shift+Tab` to seamlessly switch between **Files**, **Lines**, **Code**, **Comments**, and **Blanks**.
- **Selected Item Inspector**:
  - Detailed metadata (commit hash/period, author/commits count, date span, subject).
  - Overall metric totals/averages and delta comparisons.
  - Per-language breakdown table with shares (%), deltas, and file counts.
  - Sub-commits list showing contributing commits for aggregated periods.
  - Git additions and deletions line counts (`+X / -Y lines`).
- **Color Legend**: Language swatches with recognizable syntax/brand colors.
## Prerequisites

- [tokei](https://github.com/XAMPPRocky/tokei) (`brew install tokei` or `cargo install tokei`)
- [git](https://git-scm.com/)

## Installation

Build and install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make tokei-time
```

Or build directly:

```sh
cd functions/tokei-time
go build -o ~/dev/bin/tokei-time .
```

## Usage

# Run in the current git repository (captures whole history with streaming lazy-loading)
tokei-time

# Analyze a specific repository or path
tokei-time ~/GitRepos/my-project

# Limit to the last N commits (instead of full history)
tokei-time -n 20
# Start directly in summary mode (day, week, month, year)
tokei-time -g day
tokei-time -g week
tokei-time -g month

# Start directly with a specific metric
tokei-time -m files
tokei-time -m lines

### Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Cycle through metrics (**Files** → **Lines** → **Code** → **Comments** → **Blanks**) |
| `1` – `5` | Jump directly to metric (`1`: Code, `2`: Lines, `3`: Files, `4`: Comments, `5`: Blanks) |
| `s` / `g` | Toggle summary mode (**Commits** ↔ **Day** ↔ **Week** ↔ **Month** ↔ **Year**) |
| `S` / `G` | Toggle summary mode in reverse |
| `↑` / `↓` or `k` / `j` | Navigate commits or summary periods |
| `←` / `→` or `h` / `l` | Navigate in vertical chart mode |
| `Home` / `End` | Jump to earliest or latest commit / period |
| `v` | Toggle between **Horizontal Stacked Bars** and **Vertical Timeline Chart** |
| `c` / `d` | Toggle diff comparison mode (**Δ vs Previous** vs. **Δ vs Latest**) |
| `+` / `-` | Increase or decrease number of commits loaded (±10) |
| `a` | Fetch and display all commits in repository history |
| `r` | Refresh repository data |
| `q` / `Esc` / `Ctrl+C` | Quit |
