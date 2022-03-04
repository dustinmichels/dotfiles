# dotfiles

## Usage

When setting up a new computer, clone the repo.

```sh
# copy files to home directory
./setup.sh
```

To backup existing dotfiles:

```sh
# link files in home directory to repo
./backup.sh
```

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

### Python

#### Pyenv

```sh
# install dependencies
brew install openssl readline sqlite3 xz zlib

# install pyenv
brew install pyenv

# post install
echo 'eval "$(pyenv init --path)"' >> ~/.zprofile
echo 'eval "$(pyenv init -)"' >> ~/.zshrc
```

List available pythons

```sh
pyenv install --list | grep " 3\.[678]"
```

Install one, eg:

```sh
pyenv install 3.8.12
```

Configure default:

```sh
pyenv global 3.8.12
```

#### Poetry

Install poetry
https://python-poetry.org/docs/#installation

### Resources

- [homebrew/homebrew-bundle](https://github.com/Homebrew/homebrew-bundle)
