# gh-init

Interactive CLI tool to initialize a remote GitHub repository for the current local git repo and connect it as `origin`.

Uses [Huh](https://github.com/charmbracelet/huh) and [Bubble Tea](https://github.com/charmbracelet/bubbletea) to guide repository creation with terminal forms that validate git status, suggest the repository name, configure visibility, create the remote via GitHub CLI (`gh`), and configure the git remote URL.

## Prerequisites

- [Git](https://git-scm.com/)
- [GitHub CLI (`gh`)](https://cli.github.com/) installed and authenticated (`gh auth login`)

## Installation

Build and install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make gh-init
```

## Usage

Run from the root of any initialized local git repository:

```sh
cd path/to/my-repo
gh-init
```

The interactive prompt walks through:

1. **Repository name**: GitHub repository name (defaults to current directory name)
2. **Visibility**: Select `Private` or `Public` (defaults to Private)
3. **Connect current directory to remote?**: Automatically runs `git remote add origin https://github.com/<user>/<repo>`
