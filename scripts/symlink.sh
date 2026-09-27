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
  .config/starship.toml
  .config/mise/config.toml
  .config/zed/settings.json
  .claude/settings.json
  .omp/agent/config.yml
  .omp/agent/mcp.json
  .omp/agent/lsp.json
)

for val in $files; do
  if [[ ! -e ~/$val ]]; then
    echo "  ⏭️  $val (not found)"
    continue
  fi
  mkdir -p home/$(dirname $val)
  rm -f home/$val
  ln ~/$val home/$val
  echo "  ✅ $val"
done
