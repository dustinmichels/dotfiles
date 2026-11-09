# AI Tools

## Coding Agents

- **Oh My Pi (`omp`)**: primary terminal agent, installed as a Bun global (`~/.bun/bin`). Config lives in `home/.omp/agent/`:
  - `config.yml`: model roles (Claude Opus default, Gemini via Antigravity for `smol`/`advisor`, local Kokoro for speech), Hindsight memory backend, auto-update.
  - `mcp.json`: MCP servers (codegraph, `mise mcp`; context7 disabled).
  - `lsp.json`: language servers pointed at mise shims (rust-analyzer, typescript, vue, tailwind).
  - Storage cleanup: `omp-clean` (see `functions/omp-clean`).
- **Claude Code**: native installer (`~/.local/bin`), plus the `anthropic.claude-code` VS Code extension. Config in `home/.claude/`:
  - `settings.json`: RTK, CodeGraph, Hindsight, and Orca hooks; pre-approved codegraph/rtk commands; plugins from `claude-plugins-official` (context7, frontend-design, code-simplifier, claude-security, huggingface/netlify/liquid skills) and the `ponytail` marketplace; voice mode on.
  - `CLAUDE.md`: pulls in `RTK.md` and the CodeGraph instruction block.
- **Antigravity / Gemini**: `antigravity` cask. Config in `home/.gemini/`: `settings.json` (RTK shell hook, codegraph and mise MCP servers, rtk permission grants) and `GEMINI.md` (CodeGraph block).
- **Codex**: updates with its app. Not configured in this repo, but `update-plugins`, `agent-ls`, RTK, and CodeGraph all wire into it.
- **Pi**: skill links and agent inventory only (`~/.pi/agent/skills`).

## Editors & Desktop Apps

- **Zed**: `agent_servers` registers Antigravity, Claude, and Codex over ACP (registry), plus Oh My Pi via `omp acp`.
- **VS Code**: GitHub Copilot enabled (with next edit suggestions), Claude Code extension.
- **Orca** (`stablyai/orca` cask): IDE that orchestrates agents across terminals and worktrees. Installs its own hooks and status line into Claude's `settings.json`.
- **Hermes Desktop** and **OpenHuman** casks: desktop AI assistants. `.zprofile` puts `~/.local/bin` on `PATH` for the Hermes agent command.

## Shared Agent Tooling

- **RTK**: CLI proxy that compresses command output to cut token use. Homebrew formula in `.Brewfile`. Wired into every agent by `rtk init -g`.
  - Update: `update-plugins --only tools` (runs `brew upgrade rtk`, then `rtk init -g …` per agent).
  - Check version: `rtk --version`.
- **CodeGraph**: code index served over MCP (`codegraph serve --mcp`) and a Claude prompt hook. Installed through mise's npm backend (`config.toml`). omp's MCP entry is maintained by hand in `mcp.json` because `codegraph install` has no omp target.
  - First-time wiring: `codegraph install -t claude,antigravity -l global`. Answer **No** to "Install the codegraph CLI on your PATH?" (Yes runs `npm i -g` into whatever node is first on PATH, duplicating the mise install). The installer rewrites `~/.claude/settings.json` as a plain file, replacing the dotfiles symlink: copy it back into the repo and relink. Afterwards, pin Antigravity's entry to the mise shim (`update-plugins --only tools` does this).
  - Update: `update-plugins --only tools` (runs `mise upgrade npm:@colbymchenry/codegraph`, then `codegraph install --refresh -l global`).
  - Check version: `codegraph upgrade --check`.
- **Hindsight**: per-project long-term memory. Runtime staged at `~/.hindsight/coding-agents` by the `@vectorize-io/hindsight-coding-agents` npx installer. Hooked into Claude Code (session start, prompt submit, stop) and loaded by omp as an extension, a skill, and its memory backend (local API at `127.0.0.1:9077`). `update-plugins` updates it and re-syncs stale skill copies.
- **mise MCP**: `mise mcp` (with `MISE_EXPERIMENTAL=1`) is exposed to omp and Gemini.
- **browser-use** (uv tool): browser automation CLI that installs its skill into every agent dir.
- **TokenEconomizer** (omp only): Microsoft's cost-control plugin. Learns from past sessions per repo to inject repo facts (micro-skills), redirect stuck failure loops (loop guard), and prune unused tools. omp loads its Pi extension (`package.json` `pi.extensions`) and calls it `--host pi`. State lives in `~/.tokeneconomizer`, outside this repo.
  - Install: `omp plugin install git:github.com/microsoft/TokenEconomizer` (lands in `~/.omp/plugins/node_modules/tokeneconomizer`). Needs Python 3.10+ on `PATH`.
  - Update: `update-plugins --only plugins --agents omp` (runs `omp plugin upgrade tokeneconomizer`).
  - Use: `/te status` inside omp, or `~/.tokeneconomizer/bin/te --host pi status` from a terminal in the repo. Settings: `/te config set loopguard_nudges=2 [--global]`. Disable for one run: `TE_DISABLE=1 omp`.
  - omp caveats (it targets upstream Pi 1.2–1.3, not omp): session cost tracking and the loop guard work (omp's session files use Pi's format). Tool pruning never applies on Pi hosts. Micro-skills is broken: the Pi extension writes `event.systemPromptOptions.sections`, which omp's `before_agent_start` event doesn't have (it carries `systemPrompt: string[]` and takes a `{ systemPrompt }` override instead), so with `microskills=on` the handler throws and no facts reach the prompt. Leave micro-skills off on omp.
- **headroom-ai** (uv tool, `mcp` and `proxy` extras): context-compression MCP server and proxy.
- **kanban-mcp** (mise pipx): kanban board MCP server.
- **Document parsing for agents** (uv tools): `docling`, `liteparse`, `trafilatura`.
- **llama.cpp** (brew): local LLM inference.

> **Note on CodeGraph updates**: `codegraph install --refresh` writes Antigravity's MCP entry as a versioned mise install path, which breaks once mise prunes that version. `update-plugins` pins it back to the mise shim (`~/.local/share/mise/shims/codegraph`), which survives any `mise up`. Don't use `codegraph upgrade` here: it only knows bundle/npm installs and would `npm i -g` a second copy.

## Maintenance Commands

Built from `functions/` (`make <name>`):

- `update-plugins`: inventories and updates agents, external tools (browser-use, codegraph, hindsight, rtk), skills.sh skills, per-agent skill links, and Claude/Codex/omp plugins. Use `--dry-run` to preview, `--only tools,skills,plugins` and `--agents claude,codex,omp,pi,gemini` to narrow.
- `agent-ls`: TUI/CLI to browse and trash old sessions across omp, Antigravity, Claude, and Codex (`agent-ls clean --older-than=14d`).
- `omp-clean`: prunes large omp logs and runs `omp gc`.

### Health Check & Validation

Run the diagnostic suite for RTK and CodeGraph across OMP, Claude Code, and Antigravity:

```sh
./scripts/check_ai_tools.py
```

### References

- [CodeGraph](https://github.com/colbymchenry/codegraph)
- [RTK](https://github.com/rtk-ai/rtk)
- [Oh My Pi](https://github.com/can1357/oh-my-pi)
- [TokenEconomizer](https://github.com/microsoft/TokenEconomizer)

## Skills

Skills install into the shared hub `~/.agents/skills` (tracked in `~/.agents/.skill-lock.json`) via `npx skills add …`. `update-plugins` symlinks each hub skill into `~/.claude/skills`, `~/.pi/agent/skills`, and `~/.gemini/config/skills`, and links omp's whole skills dir to the hub.

Maybe try again:

[Fallow](https://github.com/fallow-rs/fallow-skills) (the `fallow` CLI is already in `.Brewfile`)

```sh
npx skills add fallow-rs/fallow-skills
```
