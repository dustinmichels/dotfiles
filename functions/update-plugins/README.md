# update-plugins

Inventory and update every AI tool on this machine: agent hosts, external tools, skills, and plugins.
For each item it shows the version, how it was installed, and where it propagates. It then updates
each one through its own installer so the change reaches every agent.

## What it covers

| Section | Items | Update path |
| --- | --- | --- |
| **Agents** | claude, codex, omp, pi, gemini | Read-only. Each agent updates itself. |
| **Tools** | browser-use (uv tool) | `uv tool upgrade browser-use`, then `browser-use skill install --no-install` (rewrites its SKILL.md in every assistant dir), then `browser-use --reload` |
| | hindsight (npx installer) | `npx @vectorize-io/hindsight-coding-agents@latest update` re-stages `~/.hindsight/coding-agents`. That command leaves the skill copies the installer made in each agent's skill dir untouched, so this tool re-syncs any stale copy from the runtime. |
| **Skills** | `~/.agents/skills` hub, grouped by source repo from `~/.agents/.skill-lock.json` | `npx skills update -g -y`. Hub skills that skills.sh doesn't track are attributed to their tool, or flagged as unmanaged. |
| **Skill links** | `~/.claude/skills`, `~/.pi/agent/skills`, `~/.gemini/config/skills`, `~/.omp/agent/skills` | Adds a symlink for each missing hub skill and removes dead links. omp links the whole dir to the hub. Reports copies that differ from the hub. |
| **Claude plugins** | marketplaces, installed plugins, orphaned deps | `claude plugin marketplace update`, `claude plugin update <id>`. With `-y` it also runs `claude plugin prune`. |
| **Codex plugins** | marketplaces, installed plugins by marketplace | `codex plugin marketplace upgrade` for git marketplaces. App-bundled plugins update with Codex. |
| **omp plugins** | marketplaces, installed plugins | `omp plugin marketplace update`, `omp plugin upgrade` |

Row markers: `↑` updated, `✓` current, `○` would update (dry run), `!` needs attention, `✖` failed, `·` inventory only.
Command output stays hidden unless a command fails, in which case its last lines are shown. Use `-v` to stream all output.

## Installation

```sh
cd functions
make update-plugins
```

## Usage

```sh
update-plugins -d                  # inventory + planned commands; changes nothing
update-plugins                     # update everything
update-plugins -y                  # also confirm prompts and prune orphaned Claude plugin deps
update-plugins --only skills       # sections: tools,skills,plugins
update-plugins --agents claude,omp # agents: claude,codex,omp,pi,gemini
update-plugins -v                  # stream command output
```

The exit code is 1 if any update failed.
