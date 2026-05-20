#!/bin/zsh

# --- BACKUP EXISTING DOTFILES ---
# Symlink selected dotfiles in home directory to this git repo.

files=(
  .zshrc
  .vimrc
  .tmux.conf
  .gitconfig
  .gitignore_global
  .Brewfile
  .config/starship.toml
  .config/mise/config.toml
)

for val in $files; do
  if [[ ! -e ~/$val ]]; then
    echo "skipping $val (not found in home)"
    continue
  fi
  echo $val
  mkdir -p home/$(dirname $val)
  rm -f home/$val
  ln ~/$val home/$val
done
