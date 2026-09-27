# AI Tools

### Tool Management & Updates

| Tool          | Installed Via                    | Update Command                                                      | Check Version               |
| :------------ | :------------------------------- | :------------------------------------------------------------------ | :-------------------------- |
| **RTK**       | Homebrew formula (`.Brewfile`)   | `brew upgrade rtk`                                                  | `rtk --version`             |
| **CodeGraph** | mise npm backend (`config.toml`) | `mise up npm:@colbymchenry/codegraph`<br>_(or `codegraph upgrade`)_ | `codegraph upgrade --check` |

> **Note on CodeGraph updates**: After updating CodeGraph, run `codegraph install -t antigravity --yes` to ensure Antigravity's static MCP config points to the new mise binary.

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
