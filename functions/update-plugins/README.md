# update-plugins

Unified agent updater covering **Claude Code**, **Oh-My-Pi (OMP)**, **Codex**, **Pi**, and **Gemini / Antigravity**.

Automates eight sequential maintenance phases with styled terminal badges, step-by-step progress, and execution timings:

1. **Tool Binaries**: Upgrades tool binaries like `browser-use` (`uv tool upgrade browser-use` or `browser-use --update`) and reloads daemons.
2. **Canonical Skills Hub**: Synchronizes canonical skills in `~/.agents/skills/` via `npx skills update -g -y`.
3. **Standalone Tool Skills**: Regenerates tool skills (`browser-use skill install --no-install`).
4. **Hub-and-Spoke Reconciliation**: Verifies and heals skill symlinks across `~/.claude/skills/`, `~/.pi/agent/skills/`, `~/.gemini/config/skills/`, and `~/.omp/agent/skills`, while pruning dead/dangling symlinks.
5. **Remote Marketplaces**: Refreshes plugin catalogs for Claude, Codex, and OMP.
6. **Agent-Specific Plugins**: Iterates and updates installed plugins (with fingerprint verification for Claude).
7. **Cache Pruning**: Cleans up unused plugin cache (guarded by active process preflight check).
8. **Process Signals**: Detects active sessions (`claude`, `codex`, `omp`) and outputs targeted reload instructions.

## Prerequisites

- Go (1.22+)
- Connected agent CLIs (`claude`, `codex`, `omp`, etc.) available in standard PATH or registered fallback locations.

## Installation

Build and install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make update-plugins
```

## Usage

```sh
# Run full update pipeline across all connected agents
update-plugins

# Auto-accept all confirmation prompts
update-plugins -y

# Dry run: preview actions without making changes
update-plugins -d
update-plugins --dry-run

# Run only skill phases (Phases 1-4)
update-plugins --skills-only

# Run only plugin and marketplace phases (Phases 5-7)
update-plugins --plugins-only

# Skip cache pruning step
update-plugins --skip-prune

# Target specific agents
update-plugins --agents claude,omp

# Show help
update-plugins -h
```
