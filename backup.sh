# --- BACKUP EXISTING DOTFILES ---
files=".zshrc .vimrc .tmux.conf .gitconfig .gitignore_global .config/kitty/kitty.conf .config/kitty/theme.conf"
for val in $files; do
  echo $val
  rm home/$val
  ln ~/$val home/$val
done