# dotfiles

## Usage

I symlink my dotfiles with copies in this repo.

When setting up a new computer, clone the repo, then run `./setup_new_computer.sh` to copy files to their proper place.

```sh
# copy files to home directory
./scripts/setup_new_computer.sh
```

Then (or alternatively) to symlink existing dotfiles:

```sh
# link files in home directory to repo
./scripts/symlink.sh
```

### Brewfile

Periodically backup homebrew by running:

```sh
brew bundle dump --global --force
```

This outputs Brew programs to `~/.Brewfile`.

To install anew:

```sh
brew bundle --global

# OR (to uninstall and reinstall)
./scripts/homebrew_refresh.sh
```

See more brew stuff [here](https://gist.github.com/ChristopherA/a579274536aab36ea9966f301ff14f3f)

## Other programs

Install and setup manually.

### Terminal

[iterm2](https://www.iterm2.com/) with manta colors.

### Shell

Using [oh-my-zsh](https://ohmyz.sh/).

```sh
sh -c "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
```

Install [autosuggestion plugin](https://github.com/zsh-users/zsh-autosuggestions):

```sh
git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions
```

### Homebrew

Install [homebrew](https://brew.sh/) package manager.

Install packages from Brewfile

```sh
brew bundle install
```

### Mise

- [Install mise](https://mise.jdx.dev/installing-mise.html)

Mise is installed with Homebrew.

```sh
# see global
mise ls --global
```

### Python

### Resources

- [homebrew/homebrew-bundle](https://github.com/Homebrew/homebrew-bundle)

## More notes

- [asdf](notes/asdf.md)
