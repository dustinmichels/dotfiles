#  _____    _
# |__  /___| |__  _ __ ___
#   / // __| '_ \| '__/ __|
#  / /_\__ \ | | | | | (__
# /____|___/_| |_|_|  \___|

# -------------------------------------------------------------------
# PATH STUFF
# -------------------------------------------------------------------

# If you come from bash you might have to change your $PATH.
# export PATH=$HOME/bin:/usr/local/bin:$PATH

# Path to your oh-my-zsh installation.
export ZSH="$HOME/.oh-my-zsh"

# Set name of the theme to load --- if set to "random", it will
# load a random theme each time oh-my-zsh is loaded, in which case,
# to know which specific one was loaded, run: echo $RANDOM_THEME
# See https://github.com/ohmyzsh/ohmyzsh/wiki/Themes
ZSH_THEME="robbyrussell"

# -------------------------------------------------------------------
# OHMYZSH SETTINGS
# -------------------------------------------------------------------

# Set list of themes to pick from when loading at random
# Setting this variable when ZSH_THEME=random will cause zsh to load
# a theme from this variable instead of looking in $ZSH/themes/
# If set to an empty array, this variable will have no effect.
# ZSH_THEME_RANDOM_CANDIDATES=( "robbyrussell" "agnoster" )

# Uncomment the following line to use case-sensitive completion.
# CASE_SENSITIVE="true"

# Uncomment the following line to use hyphen-insensitive completion.
# Case-sensitive completion must be off. _ and - will be interchangeable.
# HYPHEN_INSENSITIVE="true"

# Uncomment one of the following lines to change the auto-update behavior
# zstyle ':omz:update' mode disabled  # disable automatic updates
# zstyle ':omz:update' mode auto      # update automatically without asking
# zstyle ':omz:update' mode reminder  # just remind me to update when it's time

# Uncomment the following line to change how often to auto-update (in days).
# zstyle ':omz:update' frequency 13

# Uncomment the following line if pasting URLs and other text is messed up.
# DISABLE_MAGIC_FUNCTIONS="true"

# Uncomment the following line to disable colors in ls.
# DISABLE_LS_COLORS="true"

# Uncomment the following line to disable auto-setting terminal title.
# DISABLE_AUTO_TITLE="true"

# Uncomment the following line to enable command auto-correction.
# ENABLE_CORRECTION="true"

# Uncomment the following line to display red dots whilst waiting for completion.
# You can also set it to another string to have that shown instead of the default red dots.
# e.g. COMPLETION_WAITING_DOTS="%F{yellow}waiting...%f"
# Caution: this setting can cause issues with multiline prompts in zsh < 5.7.1 (see #5765)
# COMPLETION_WAITING_DOTS="true"

# Uncomment the following line if you want to disable marking untracked files
# under VCS as dirty. This makes repository status check for large repositories
# much, much faster.
# DISABLE_UNTRACKED_FILES_DIRTY="true"

# Uncomment the following line if you want to change the command execution time
# stamp shown in the history command output.
# You can set one of the optional three formats:
# "mm/dd/yyyy"|"dd.mm.yyyy"|"yyyy-mm-dd"
# or set a custom format using the strftime function format specifications,
# see 'man strftime' for details.
# HIST_STAMPS="mm/dd/yyyy"

# Would you like to use another custom folder than $ZSH/custom?
# ZSH_CUSTOM=/path/to/new-custom-folder

# Which plugins would you like to load?
# Standard plugins can be found in $ZSH/plugins/
# Custom plugins may be added to $ZSH_CUSTOM/plugins/
# Example format: plugins=(rails git textmate ruby lighthouse)
# Add wisely, as too many plugins slow down shell startup.
plugins=(
  git
  zsh-autosuggestions
  mise
  # z
  # asdf
  # poetry
)

source $ZSH/oh-my-zsh.sh

# -------------------------------------------------------------------
# USER CONFIG
# -------------------------------------------------------------------

# export MANPATH="/usr/local/man:$MANPATH"

# You may need to manually set your language environment
# export LANG=en_US.UTF-8

# Preferred editor for local and remote sessions
# if [[ -n $SSH_CONNECTION ]]; then
#   export EDITOR='vim'
# else
#   export EDITOR='mvim'
# fi

# Compilation flags
# export ARCHFLAGS="-arch x86_64"

# Set personal aliases, overriding those provided by oh-my-zsh libs,
# plugins, and themes. Aliases can be placed here, though oh-my-zsh
# users are encouraged to define aliases within the ZSH_CUSTOM folder.
# For a full list of active aliases, run `alias`.
#
# Example aliases
# alias zshconfig="mate ~/.zshrc"
# alias ohmyzsh="mate ~/.oh-my-zsh"

# alias air='$(go env GOPATH)/bin/air'

# -------------------------------------------------------------------
# FUNCTIONS
# -------------------------------------------------------------------

# */ See external IP address */
function exip {
  curl ipecho.net/plain
  echo
}

# */ Create a new directory and enter it */
function mkd() {
  mkdir -p "$@" && cd "$@"
}

# */ See 10 biggest items */
function biggest() {
  du -ah * | sort -rh | head -10
}

# */ Source various env files
# */
# */ Example usage:
# */    envup && go run .
# */    envup production && go run .
function envup() {
  file=$([ -z "$1" ] && echo ".env" || echo ".env.$1")
  [ "$1" = "-f" ] && shift && file=$1
  if [ -f "$file" ]; then
    IFS=$'\n'
    env_vars=($(sed '/^#.*/d; /^[[:space:]]*$/d; s/^export //' $file))
    for v in $env_vars; do
      eval export $v
    done
  else
    echo "$file does not exist"
    return 1
  fi
}

# -------------------------------------------------------------------
# MORE PATH STUFF
# -------------------------------------------------------------------

# golang
# export PATH="$PATH:$HOME/go/bin/"

# connect4
export PATH="$PATH:$HOME/GitRepos/connect4/bin"

# Created by `pipx` on 2023-11-19 17:32:16
# export PATH="$PATH:/Users/dustinmichels/.local/bin"

# bun completions
# [ -s "/Users/dustinmichels/.bun/_bun" ] && source "/Users/dustinmichels/.bun/_bun"

# bun
# export BUN_INSTALL="$HOME/.bun"
# export PATH="$BUN_INSTALL/bin:$PATH"
# export PATH="$HOME/.composer/vendor/bin:$PATH"

# java
# export JAVA_HOME=/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home
# export PATH="$JAVA_HOME/bin:$PATH"

# . "$HOME/.cargo/env"

# Added by Antigravity
export PATH="/Users/dustinmichels/.antigravity/antigravity/bin:$PATH"

eval "$(mise activate zsh)"

eval "$(zoxide init zsh)"

# cleanup path
typeset -U PATH
