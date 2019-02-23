# dotfiles

Some of my customized configuration files

## Setting up Terminal Land

### Install Programs

1. Install [iterm2](https://www.iterm2.com/) as terminal replacement.

2. Install [Oh-My-Zsh](https://github.com/robbyrussell/oh-my-zsh) as bash replacement.

   ```bash
   sh -c "$(curl -fsSL https://raw.githubusercontent.com/robbyrussell/oh-my-zsh/master/tools/install.sh)"
   ```

3. Install [homebrew](https://brew.sh/) package manager.

   ```bash
   /usr/bin/ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
   ```

4. Install [tmux](https://github.com/tmux/tmux) using Homebrew.

   ```bash
   brew install tmux
   ```

5. Install Hack Nerd Font (which includes icons)

   ```bash
   brew tap caskroom/fonts
   brew cask install font-hack-nerd-font
   ```

6. Customize Iterm2

   - Launch [tmux automatically](https://medium.com/@sasom/auto-starting-tmux-in-iterm2-4276182d452a).

     - Navigate to “Preferences > Profiles > PROFILE >Command > Send text at start” and set it to:

     ```text
     tmux ls && read tmux_session && tmux attach -t ${tmux_session:-default} || tmux new -s ${tmux_session:-default}
     ```

   - Set font to "Hack Regular Nerd Font Complete"

   - Set color preset to "Tango Dark"

7. Install [Miniconda](https://docs.conda.io/en/latest/miniconda.html).

   - Install [bash installer](https://repo.anaconda.com/miniconda/Miniconda3-latest-MacOSX-x86_64.sh)

   ```bash
   cd ~/Downloads
   bash Miniconda3-latest-MacOSX-x86_64.sh
   ```

### Copy dotfiles

```bash
cp -R home/. $HOME/
```

---

## More Programs

### VS Code

- Install extensions

  - Prettier
  - Material Theme
  - Material Icon Theme
  - Python
  - Django

- Settings

  - Code > Preferences > Settings [⌘ ,]
  - Paste `vscode/settings.json`

- Keybindings

  - File > Preferences > Keyboard Shortcuts [⌘ K, ⌘ S]
  - Paste `vscode/keybindings.json`
