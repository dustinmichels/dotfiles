# agent-ls

Browse, inspect disk usage, and clean AI coding agent sessions across multiple assistants: Oh My Pi (`omp`), Antigravity, Claude Code, and Codex.

Scans local conversation stores, aggregates metadata (project, title, size, age, active status), flags sessions older than a retention threshold (default: 14 days), and provides an interactive terminal UI along with headless CLI modes for safely trashing old conversations.

## Installation

Build and install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make agent-ls
```

## Usage

### Interactive TUI

Launch the full-screen terminal session browser and cleaner:

```sh
agent-ls
```

#### Keybindings

- `↑` / `k`, `↓` / `j`: Navigate session rows
- `Space`: Toggle selection of current session
- `O`: Select all sessions older than 14 days
- `a`: Toggle selection of all visible sessions
- `t`: Cycle tool filter (`all` → `omp` → `antigravity` → `claude` → `codex`)
- `o`: Toggle view filter (show all vs. only sessions >14 days old)
- `/`: Search / live filter by title, project, or session ID
- `d`: Move selected sessions to Trash (`~/.Trash/agent-ls/`) with confirmation prompt
- `q` / `Ctrl+C`: Quit

### Non-Interactive / CLI

```sh
# Print disk usage summary table by agent
agent-ls --summary

# Output plain text table (used automatically when piped)
agent-ls --plain
agent-ls -p

# Output JSON
agent-ls --json
agent-ls -j

# Filter to a specific agent
agent-ls --tool=omp
agent-ls --tool=antigravity
agent-ls --tool=claude
agent-ls --tool=codex

# Filter by age threshold
agent-ls --older-than=30d --older-only

# Batch cleanup old sessions to Trash without TUI
agent-ls clean --dry-run
agent-ls clean --older-than=14d
agent-ls clean --older-than=30d --tool=omp
```
