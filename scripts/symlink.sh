#!/bin/zsh

# Symlink dotfiles from this git repo to the home directory.

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"

files=(
  .zshrc
  .zprofile
  .zshenv
  .vimrc
  .tmux.conf
  .gitconfig
  .gitignore_global
  .Brewfile

  # VSCode
  "Library/Application Support/Code/User/settings.json"

  # config
  .config/starship.toml
  .config/mise/config.toml
  .config/zed/settings.json

  # claude
  .claude/settings.json
  .claude/CLAUDE.md

  # antigravity
  .gemini/settings.json
  .gemini/GEMINI.md

  # omp
  .omp/agent/config.yml
  .omp/agent/mcp.json
  .omp/agent/lsp.json
  .omp/agent/extensions/rtk.ts
)

for val in "${files[@]}"; do
  # If file exists in $HOME but not in repo, copy it into repo first
  if [[ -e "$HOME/$val" && ! -L "$HOME/$val" && ! -e "$REPO_DIR/home/$val" ]]; then
    mkdir -p "$REPO_DIR/home/$(dirname "$val")"
    cp "$HOME/$val" "$REPO_DIR/home/$val"
  fi

  if [[ ! -e "$REPO_DIR/home/$val" ]]; then
    echo "  ⏭️  $val (not found in repo)"
    continue
  fi

  mkdir -p "$HOME/$(dirname "$val")"

  # Back up if $HOME file is a regular file that differs from repo
  if [[ -f "$HOME/$val" && ! -L "$HOME/$val" ]] && ! cmp -s "$HOME/$val" "$REPO_DIR/home/$val"; then
    mv "$HOME/$val" "$HOME/$val.bak"
    echo "  ⚠️  $val differed, backed up to $val.bak"
  fi

  rm -f "$HOME/$val"
  ln -s "$REPO_DIR/home/$val" "$HOME/$val"
  echo "  ✅ $val"
done
