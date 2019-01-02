export TERM="xterm-256color"

# If you come from bash you might have to change your $PATH.
# export PATH=$HOME/bin:/usr/local/bin:$PATH

# rbenv
eval "$(rbenv init -)"

# Path to Brew
export PATH="/usr/local/Cellar:$PATH"

# Path to your oh-my-zsh installation.
export ZSH=/Users/dustymichels/.oh-my-zsh

# Path to Julia
export PATH="/Applications/Julia-0.6.app/Contents/Resources/julia/bin:$PATH"

# Path to Miniconda
export PATH="/Users/dustymichels/miniconda3/bin:$PATH"

# Add GHC 7.8.4 to the PATH, via http://ghcformacosx.github.io/
export GHC_DOT_APP="/Applications/ghc-7.8.4.app"
if [[ -d "$GHC_DOT_APP" ]]; then
  export PATH="${HOME}/.cabal/bin:${GHC_DOT_APP}/Contents/bin:${PATH}"
fi

# Set name of the theme to load. Optionally, if you set this to "random"
# See https://github.com/robbyrussell/oh-my-zsh/wiki/Themes
ZSH_THEME="avit"

##################################
# POWERLEVEL9K STUFF

# POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(context dir rbenv nvm vcs anaconda)
# POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(status root_indicator background_jobs)

# POWERLEVEL9K_VIRTUALENV_BACKGROUND="white"
# POWERLEVEL9K_ANACONDA_BACKGROUND="white"

# POWERLEVEL9K_MODE='awesome-patched'
# POWERLEVEL9K_PROMPT_ON_NEWLINE=true

# POWERLEVEL9K_HOME_ICON="\ue17e"

# export DEFAULT_USER="$USER"

# ZSH_THEME="powerlevel9k/powerlevel9k"
##################################

# Uncomment the following line to use case-sensitive completion.
# CASE_SENSITIVE="true"

# Uncomment the following line to use hyphen-insensitive completion. Case
# sensitive completion must be off. _ and - will be interchangeable.
# HYPHEN_INSENSITIVE="true"

# Uncomment the following line to disable bi-weekly auto-update checks.
# DISABLE_AUTO_UPDATE="true"

# Uncomment the following line to change how often to auto-update (in days).
# export UPDATE_ZSH_DAYS=13

# Uncomment the following line to disable colors in ls.
# DISABLE_LS_COLORS="true"

# Uncomment the following line to disable auto-setting terminal title.
# DISABLE_AUTO_TITLE="true"

# Uncomment the following line to enable command auto-correction.
# ENABLE_CORRECTION="true"

# Uncomment the following line to display red dots whilst waiting for completion.
# COMPLETION_WAITING_DOTS="true"

# Uncomment the following line if you want to disable marking untracked files
# under VCS as dirty. This makes repository status check for large repositories
# much, much faster.
# DISABLE_UNTRACKED_FILES_DIRTY="true"

# Uncomment the following line if you want to change the command execution time
# stamp shown in the history command output.
# The optional three formats: "mm/dd/yyyy"|"dd.mm.yyyy"|"yyyy-mm-dd"
# HIST_STAMPS="mm/dd/yyyy"

# Would you like to use another custom folder than $ZSH/custom?
# ZSH_CUSTOM=/path/to/new-custom-folder

# Which plugins would you like to load? (plugins can be found in ~/.oh-my-zsh/plugins/*)
# Custom plugins may be added to ~/.oh-my-zsh/custom/plugins/
# Example format: plugins=(rails git textmate ruby lighthouse)
# Add wisely, as too many plugins slow down shell startup.
plugins=(git)

source $ZSH/oh-my-zsh.sh

# User configuration

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

# ssh
# export SSH_KEY_PATH="~/.ssh/rsa_id"

# Set personal aliases, overriding those provided by oh-my-zsh libs,
# plugins, and themes. Aliases can be placed here, though oh-my-zsh
# users are encouraged to define aliases within the ZSH_CUSTOM folder.
# For a full list of active aliases, run `alias`.
#
# Example aliases
# alias zshconfig="mate ~/.zshrc"
# alias ohmyzsh="mate ~/.oh-my-zsh"

# eval "$(rbenv init -)"

alias pc="open -a /Applications/PyCharm.app/Contents/MacOS/pycharm"

# eval $(thefuck --alias)

# added by travis gem
[ -f /Users/dustymichels/.travis/travis.sh ] && source /Users/dustymichels/.travis/travis.sh
function gi() { curl -L -s https://www.gitignore.io/api/$@ ;}

# OPAM configuration
. /Users/dustymichels/.opam/opam-init/init.zsh > /dev/null 2> /dev/null || true

