# dotfiles

Some of my customized configuration files

## Setting up Terminal Land

### Install Programs

1. Install [iterm2](https://www.iterm2.com/) as terminal replacement.

2. Install [Oh-My-Zsh](https://github.com/robbyrussell/oh-my-zsh) as bash replacement.

   ```bash
   sh -c "$(curl -fsSL https://raw.githubusercontent.com/robbyrussell/oh-my-zsh/master/tools/install.sh)"

   # add autosuggestion plugin
   git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions
   ```

3. Install [homebrew](https://brew.sh/) package manager.

   ```bash
   /usr/bin/ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
   ```

4. Install stuff from brewfile

   ```bash
   brew bundle install
   ```

5. Customize Iterm2

   - Launch [tmux automatically](https://medium.com/@sasom/auto-starting-tmux-in-iterm2-4276182d452a).

     - Navigate to “Preferences > Profiles > PROFILE >Command > Send text at start” and set it to:

     ```text
     tmux ls && read tmux_session && tmux attach -t ${tmux_session:-default} || tmux new -s ${tmux_session:-default}
     ```

   - Set font to "Hack Regular Nerd Font Complete"

   - Set color preset to "Tango Dark" (or Snazzy). Download more color schemes [here](https://iterm2colorschemes.com/)

6. Install [Miniconda](https://docs.conda.io/en/latest/miniconda.html).

   - Install [bash installer](https://repo.anaconda.com/miniconda/Miniconda3-latest-MacOSX-x86_64.sh)

   ```bash
   cd ~/Downloads
   bash Miniconda3-latest-MacOSX-x86_64.sh
   ```

### Copy dotfiles

```bash
cp -R home/. $HOME/

# or run:
./setup.sh
```

### Global node/npm

```bash
# npm
npm install -g @google/clasp typescript

# yarn
yarn global add @vue/cli eslint
```

### Resources

- [mas-cli/mas](https://github.com/mas-cli/mas)

- [homebrew/homebrew-bundle](https://github.com/Homebrew/homebrew-bundle)
