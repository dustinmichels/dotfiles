# brew-ls

Interactive TUI and CLI tool to view, sort, filter, and manage manually installed Homebrew formulae and casks with their original install dates.

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
