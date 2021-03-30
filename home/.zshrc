# ----------
### PATH ###

# If you come from bash you might have to change your $PATH.
export PATH=$HOME/bin:/usr/local/bin:$PATH

# Path to your oh-my-zsh installation.
export ZSH="/Users/dustinmichels/.oh-my-zsh"

# Miniconda3
export PATH=/Users/dustinmichels/miniconda3/bin:$PATH

# Flutter
export PATH=$PATH:/Users/dustinmichels/development/flutter/bin

# -----------
### THEME ###

ZSH_THEME="robbyrussell"

# *************************
#~~ POWERLEVEL9K STUFF ~~#

# POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(context dir vcs rbenv virtualenv anaconda nvm)
# POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(status command_execution_time background_jobs root_indicator)

# POWERLEVEL9K_VIRTUALENV_BACKGROUND="grey"
# POWERLEVEL9K_VIRTUALENV_VISUAL_IDENTIFIER_COLOR="green"

# POWERLEVEL9K_ANACONDA_BACKGROUND="grey"
# POWERLEVEL9K_ANACONDA_VISUAL_IDENTIFIER_COLOR="green"

# POWERLEVEL9K_MODE='nerdfont-complete'
# POWERLEVEL9K_PROMPT_ON_NEWLINE=true

# POWERLEVEL9K_HOME_ICON='\uf06c'

# POWERLEVEL9K_LEFT_SEGMENT_SEPARATOR=$'\uE0B8' # angly

# export DEFAULT_USER="$USER"

# ZSH_THEME="powerlevel9k/powerlevel9k"
# *************************

# ------------
### CONFIG ###

# Uncomment the following line to use case-sensitive completion.
# CASE_SENSITIVE="true"

# Uncomment the following line to use hyphen-insensitive completion.
# Case-sensitive completion must be off. _ and - will be interchangeable.
# HYPHEN_INSENSITIVE="true"

# Uncomment the following line to disable bi-weekly auto-update checks.
# DISABLE_AUTO_UPDATE="true"

# Update without prompt
DISABLE_UPDATE_PROMPT=true

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
# You can set one of the optional three formats:
# "mm/dd/yyyy"|"dd.mm.yyyy"|"yyyy-mm-dd"
# or set a custom format using the strftime function format specifications,
# see 'man strftime' for details.
# HIST_STAMPS="mm/dd/yyyy"

# Would you like to use another custom folder than $ZSH/custom?
# ZSH_CUSTOM=/path/to/new-custom-folder

# Which plugins would you like to load?
# Standard plugins can be found in ~/.oh-my-zsh/plugins/*
# Custom plugins may be added to ~/.oh-my-zsh/custom/plugins/
# Example format: plugins=(rails git textmate ruby lighthouse)
# Add wisely, as too many plugins slow down shell startup.
plugins=(
    git
    zsh-autosuggestions
)

source $ZSH/oh-my-zsh.sh

# ------------------------
### User configuration ###

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

# >>> conda initialize >>>
# !! Contents within this block are managed by 'conda init' !!
__conda_setup="$('/Users/dustinmichels/miniconda3/bin/conda' 'shell.zsh' 'hook' 2>/dev/null)"
if [ $? -eq 0 ]; then
    eval "$__conda_setup"
else
    if [ -f "/Users/dustinmichels/miniconda3/etc/profile.d/conda.sh" ]; then
        . "/Users/dustinmichels/miniconda3/etc/profile.d/conda.sh"
    else
        export PATH="/Users/dustinmichels/miniconda3/bin:$PATH"
    fi
fi
unset __conda_setup
# <<< conda initialize <<<

# nvm
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh" # This loads nvm

# The next line updates PATH for the Google Cloud SDK.
if [ -f '/Users/dustinmichels/development/google-cloud-sdk/path.zsh.inc' ]; then . '/Users/dustinmichels/development/google-cloud-sdk/path.zsh.inc'; fi

# The next line enables shell command completion for gcloud.
if [ -f '/Users/dustinmichels/development/google-cloud-sdk/completion.zsh.inc' ]; then . '/Users/dustinmichels/development/google-cloud-sdk/completion.zsh.inc'; fi

# cleanup path
typeset -U PATH
