# tokei-time

Interactive git code metrics over time powered by `tokei`.

Count files, lines, code, comments, and blanks across programming languages, travel back through git commit history, compare diffs to previous commits or the latest working tree, and visualize everything in an interactive stacked bar chart in your terminal.

## Features

- **Time Travel**: Inspect code metrics across recent commits or uncommitted working tree changes.
- **Diff Comparison**: Compare the diff introduced by each commit vs. its parent commit, or measure cumulative drift against the latest commit / working tree.
- **Interactive Stacked Bar Graph**:
  - Proportional multi-color stacked bars colored by programming language.
  - Dual visualization modes:
    - **Horizontal List View**: Commit timeline with hashes, relative timestamps, commit subjects, stacked bars, metric totals, and delta indicators.
    - **Vertical Timeline Chart**: ASCII/Unicode chart showing code growth curves over time with language proportions.
- **Tab Metric Switching**: Use `Tab` or `Shift+Tab` to seamlessly switch between **Files**, **Lines**, **Code**, **Comments**, and **Blanks**.
- **Selected Commit Inspector**:
  - Detailed metadata (commit hash, author, date, subject).
  - Overall metric totals and delta comparisons.
  - Per-language breakdown table with shares (%), deltas, and file counts.
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

```sh
# Run in the current git repository (default 10 commits)
tokei-time

# Analyze a specific repository or path
tokei-time ~/GitRepos/my-project

# Analyze 20 commits back
tokei-time -n 20

# Start directly with the "lines" or "files" metric
tokei-time -m files
tokei-time -m lines
```

### Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Cycle through metrics (**Files** → **Lines** → **Code** → **Comments** → **Blanks**) |
| `1` – `5` | Jump directly to metric (`1`: Code, `2`: Lines, `3`: Files, `4`: Comments, `5`: Blanks) |
| `↑` / `↓` or `k` / `j` | Navigate commits up/down |
| `←` / `→` or `h` / `l` | Navigate commits in vertical chart mode |
| `Home` / `End` or `g` / `G` | Jump to earliest or latest commit |
| `v` | Toggle between **Horizontal Stacked Bars** and **Vertical Timeline Chart** |
| `c` / `d` | Toggle diff comparison mode (**Δ vs Previous** vs. **Δ vs Latest**) |
| `+` / `-` | Increase or decrease number of commits loaded (±5) |
| `r` | Refresh repository data |
| `q` / `Esc` / `Ctrl+C` | Quit |
