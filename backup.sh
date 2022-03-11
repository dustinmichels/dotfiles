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
  .config/kitty/kitty.conf
  .config/kitty/theme.conf
)

for val in $files; do
  echo $val
  rm home/$val
  ln ~/$val home/$val
done
