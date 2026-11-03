# AI Tools

### Tool Management & Updates

| Tool          | Installed Via                    | Update Command                                                      | Check Version               |
| :------------ | :------------------------------- | :------------------------------------------------------------------ | :-------------------------- |
| **RTK**       | Homebrew formula (`.Brewfile`)   | `update-plugins --only tools` (`brew upgrade rtk`, then `rtk init -g …` per agent) | `rtk --version`             |
| **CodeGraph** | mise npm backend (`config.toml`) | `update-plugins --only tools` (`mise upgrade npm:@colbymchenry/codegraph`, then `codegraph install --refresh -l global`) | `codegraph upgrade --check` |

> **Note on CodeGraph updates**: `codegraph install --refresh` writes Antigravity's MCP entry as a versioned mise install path, which breaks once mise prunes that version. `update-plugins` pins it back to the mise shim (`~/.local/share/mise/shims/codegraph`), which survives any `mise up`. Don't use `codegraph upgrade` here: it only knows bundle/npm installs and would `npm i -g` a second copy.

### Health Check & Validation

Run the automated diagnostic suite across OMP, Claude Code, and Antigravity:

```sh
./scripts/check_ai_tools.py
```

### References

- [CodeGraph](https://github.com/colbymchenry/codegraph)
- [RTK](https://github.com/rtk-ai/rtk)

## Skills

Maybe try again:

[Fallow](https://github.com/fallow-rs/fallow-skills)

```sh
npx skills add fallow-rs/fallow-skills
```
