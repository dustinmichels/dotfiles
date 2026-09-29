# repo-ls

Interactive Git repository explorer and manager built with Go and Bubble Tea.

Recursively scans directories (such as `~/GitRepos`), shows uncommitted dirty states, detects GitHub linkage and visibility (`PUBLIC` / `PRIVATE` / `LOCAL`), displays live code statistics via `tokei`, and provides safe local and remote (GitHub) repository deletion.

## Features

- **Fast Recursive Scanning**: Recursively discovers git repositories at any depth while automatically ignoring noisy build and cache folders (`node_modules`, `target`, `dist`, `.cache`, `.venv`, `Pods`, etc.).
- **Subtle Dirty Indicators**: Flags repositories with uncommitted changes via a clean red dot (`●`) without tinting the entire row red.
- **Date Created & Last Modified**: Displays `DATE-CREATED` and `LAST-MODIFIED` timestamps for every repository, taking into account commit history and uncommitted changes.
- **Flexible Sorting**: Press `s` (or `S` in reverse) to cycle sorting by Path, Name, Last Modified, Date Created, or Dirty First, or specify `--sort <mode>` on startup.
- **Collapsible Folder & Parent Groups**: Subfolders containing repositories (e.g. `ARCHIVE`, `_TUFTS`) and parent repositories with nested repos (e.g. `my-tools`) can be collapsed and expanded with `Space`, `Enter`, or arrow keys. Press `Cmd+←` / `Alt+←` / `c` to collapse all, and `Cmd+→` / `Alt+→` / `e` to expand all.
- **Global Portfolio Summary**: Deselect any repository (press `Esc` or move `↑` from the top) to view aggregate code statistics (`tokei`), portfolio-wide language distribution, commit totals, and recent activity across all repositories.
- **GitHub Integration**:
  - Detects remote URL, remote name, and GitHub repository slug (`owner/repo`).
  - Identifies visibility (`PUBLIC`, `PRIVATE`, or `LOCAL` only) by batch-checking the authenticated user's repositories via `gh`.
  - Press `o` to open any repository directly in GitHub in your browser.
- **Interactive Inspector & Tokei Code Stats**:
  - Live inspector pane displays branch, commit count, latest commit (hash, relative time, author, subject), exact dates, and changed files preview.
  - Automatically runs `tokei` asynchronously to display total lines of code, comments, blanks, file count, and a top-languages breakdown chart.
- **Keyboard & Mouse Navigation**:
  - Press `Tab` / `Shift+Tab` to seamlessly move through filter options (`All`, `Dirty`, `Clean`, `GitHub`, `Local`, `Public`, `Private`) and modal choices.
  - Click any row with your mouse to inspect it; double click or press `Enter` to expand full details.
  - Scroll with mouse wheel or arrow keys / vim bindings (`j`/`k`).
- **Safe Repository Deletion**:
  - Press `d` or `x` on any selected repo to open the deletion dialog.
  - Choose to delete locally only, delete on GitHub only (via `gh repo delete`), or delete both.
  - Cycle options with `Tab` and confirm with explicit repo name or `y`.
## Installation

Build and install to `~/dev/bin` via the `functions/Makefile`:

```sh
cd functions
make repo-ls
```

Because `~/dev/bin` is in your `$PATH`, you can run `repo-ls` from anywhere.

## Usage

### Interactive TUI

Run in your current directory or pass a target directory (such as `~/GitRepos`):

```sh
repo-ls ~/GitRepos
```

*(Note: If run with no arguments from your home directory `~`, `repo-ls` automatically defaults to `~/GitRepos` if it exists.)*

#### Keybindings & Controls

| Key | Action |
| --- | --- |
| `↑` / `k` | Move selection up (moving up from top deselects to show all-repos summary) |
| `↓` / `j` | Move selection down (from unselected state, selects first item) |
| `Home` / `End` / `G` | Jump to top / bottom |
| `PgUp` / `PgDown` | Page scroll |
| `Left Click` | Select clicked repository or toggle folder collapse |
| `Mouse Wheel` | Scroll table or inspector |
| `Space` / `←` / `→` | Toggle collapse / expand on folder or parent repo |
| `c` / `Cmd+←` / `Alt+←` / `Ctrl+←` | Collapse all folders |
| `e` / `Cmd+→` / `Alt+→` / `Ctrl+→` | Expand all folders |
| `Enter` / `i` | Toggle full-screen inspector view (or toggle folder if on folder) |
| `Tab` / `Shift+Tab` | Cycle filter options / modal choices |
| `/` | Live search filter (name, path, remote) |
| `Esc` | Deselect repo to view summary / clear search / close modals |
| `u` | Toggle filter: Uncommitted (dirty) only |
| `g` | Toggle filter: GitHub repos only |
| `l` | Toggle filter: Local repos only |
| `p` | Toggle filter: Public GitHub repos only |
| `P` | Toggle filter: Private GitHub repos only |
| `a` | Reset filter to all |
| `s` / `S` | Cycle sort mode forward / backward (Path → Name → Last Modified → Date Created → Dirty First) |
| `o` | Open repository in GitHub in your browser |
| `d` / `x` | Open deletion dialog for selected repository |
| `r` | Rescan repositories |
| `q` / `Ctrl+C` | Quit |
### Non-Interactive / Plain Mode

Output a tab-delimited plain text table for scripting or quick command-line checking:

```sh
repo-ls --plain --sort modified ~/GitRepos
```
