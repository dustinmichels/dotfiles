# update-plugins

CLI utility to keep Claude Code plugins up to date and clean up cached dependencies.

Automates three sequential maintenance steps with styled terminal badges, step-by-step progress, and execution timings:

1. Refreshes plugin marketplace catalogs (`claude plugin marketplace update`).
2. Reads installed plugins from `~/.claude/plugins/installed_plugins.json` and updates each one (`claude plugin update <id>`).
3. Prunes unused plugins and cached dependencies (`claude plugin prune`).

## Prerequisites

- [Claude Code (`claude`)](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) CLI installed and available in PATH.

## Installation

Build and install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make update-plugins
```

## Usage

```sh
# Run interactively (prompts for confirmation during updates/pruning)
update-plugins

# Auto-accept all prompts
update-plugins -y
update-plugins --yes

# Dry run: preview actions without running claude commands
update-plugins -d
update-plugins --dry-run

# Show help
update-plugins -h
update-plugins --help
```
