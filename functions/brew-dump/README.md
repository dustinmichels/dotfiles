# brew-dump

Dump Homebrew dependencies into a cleanly formatted `Brewfile`.

Wraps `brew bundle dump`, adding section spacing, aligning package descriptions inline with trailing comments, appending initial install dates from Cellar and Caskroom, and sorting formulae and casks chronologically with most recent installs first.

## Installation

Install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make brew-dump
```

## Usage

```sh
# Dump to global Brewfile (~/.Brewfile or ~/.config/homebrew/Brewfile)
brew-dump

# Print formatted Brewfile to stdout without writing to file
brew-dump --stdout
brew-dump -o

# Dry-run: preview output without modifying any file
brew-dump --dry-run
brew-dump -n

# Write to a specific file
brew-dump -f ~/path/to/Brewfile
brew-dump --file=~/path/to/Brewfile

# Suppress status output
brew-dump -q
brew-dump --quiet

# Forward arguments directly to 'brew bundle dump'
brew-dump --no-cask
brew-dump --no-vscode
brew-dump --mas
```
