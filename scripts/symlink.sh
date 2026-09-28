#!/bin/zsh

# Symlink selected dotfiles in home directory to this git repo.

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
  if [[ ! -e "$HOME/$val" ]]; then
    echo "  ⏭️  $val (not found)"
    continue
  fi
  mkdir -p "home/$(dirname "$val")"
  rm -f "home/$val"
  ln "$HOME/$val" "home/$val"
  echo "  ✅ $val"
done
