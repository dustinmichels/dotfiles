# dotfiles

Some of my customized configuration files

## Setup new macbook

### Install programs

1. Install [iterm2](https://www.iterm2.com/) as terminal replacement.

2. Install [Oh-My-Zsh](https://github.com/robbyrussell/oh-my-zsh) as bash replacement.

   ```bash
   sh -c "$(curl -fsSL https://raw.githubusercontent.com/robbyrussell/oh-my-zsh/master/tools/install.sh)"
   ```

3. Install [homebrew](https://brew.sh/) package manager.

   ```bash
   /usr/bin/ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
   ```

4. Install tmux with brew.

   ```bash
   brew install tmux
   ```

5. Add tmux to [iterm2 profile](https://medium.com/@sasom/auto-starting-tmux-in-iterm2-4276182d452a).

   - Navigate to “Preferences > Profiles > PROFILE >Command > Send text at start” and set it to:

   ```plaintext
   tmux ls && read tmux_session && tmux attach -t ${tmux_session:-default} || tmux new -s ${tmux_session:-default}
   ```

   - Also set color preset to "Pastel (Dark Background)"

### Copy dotfiles

```bash
cp -R home/. $HOME/
```
