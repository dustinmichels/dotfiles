# brew-ls

Interactive TUI, web UI, and CLI tool to view, sort, filter, and manage manually installed Homebrew formulae and casks with their original install dates.

Easily inspect package metadata, view reverse dependencies, sort chronologically, upgrade, or uninstall.

## Installation

Build and install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make brew-ls
```

## Usage

### Interactive TUI

Launch the full-screen terminal interface:

```sh
brew-ls
```

#### Shortcuts

- **Navigation**: `↑` / `k`, `↓` / `j`, `Home` / `g`, `End` / `G`, mouse wheel, or click a row
- **Filter view**: `Tab` / `t` (cycle All → Formulae → Casks)
- **Search**: `/` (live filter by name and description), `Esc` to clear
- **Sorting**: `s` / `d` (toggle newest ↔ oldest), `n` (alphabetical A–Z)
- **Actions**:
  - `Enter`: Open details modal (shows version, homepage, install date, reverse dependencies)
  - `u`: Upgrade selected package (`brew upgrade`)
  - `x` / `d`: Uninstall package (`brew uninstall`, press twice to confirm)
  - `o`: Open package homepage in browser
  - `?`: Toggle help overlay
  - `q` / `Ctrl+C`: Quit

### Web UI

Serve a local web page (bundled into the binary) and open it in your browser:

```sh
brew-ls --web              # http://127.0.0.1:4747
brew-ls --web --port 8080
brew-ls --web --no-open    # don't launch the browser
```

- Every installed package appears as a cell in the month it was first installed; click a cell to inspect it, or a year to filter to it.
- Filter by name/description (`/` focuses the box), by type, or to packages with updates; sort by date or name.
- The details panel shows version vs. latest, install date, license, tap, dependencies, reverse dependencies, caveats, and the raw `brew info` JSON.
- **Upgrade**, **Uninstall**, and **Add package** (searches Homebrew, then **Install**) run `brew` on the server and stream its output into the page. One command runs at a time.

The server listens on `127.0.0.1` only and rejects requests whose `Host` or `Origin` isn't the brew-ls page, so other websites can't drive it.

### Non-Interactive / CLI

```sh
# Print plain text table to stdout
brew-ls --plain

# Output JSON
brew-ls --json

# Filter by package type
brew-ls --formula   # or -f
brew-ls --cask      # or -c

# Sort oldest first (in plain mode)
brew-ls --plain --reverse   # or -r
```
