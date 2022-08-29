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
export ZSH="/Users/dustinmichels/.oh-my-zsh"

# Poetry (python)
export PATH="$HOME/.poetry/bin:$PATH"

# Go
export PATH="$PATH:/Users/dustinmichels/go/bin"

# Journey
export JOURNEY_ENV=dev
export GOPRIVATE=github.com/journeyid

# -------------------------------------------------------------------
# OHMYZSH SETTINGS
# -------------------------------------------------------------------

# Set name of the theme to load --- if set to "random", it will
# load a random theme each time oh-my-zsh is loaded, in which case,
# to know which specific one was loaded, run: echo $RANDOM_THEME
# See https://github.com/ohmyzsh/ohmyzsh/wiki/Themes
ZSH_THEME="robbyrussell"

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
    poetry
    z
    asdf
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
alias brew='env PATH="${PATH//$(pyenv root)\/shims:/}" brew' # Python (pyenv)

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

# Taylor's Proxy
TUNNEL=35.87.44.52
tunnel() {
    echo "forwarding $1..."
    \ssh -o "ExitOnForwardFailure yes" -N -R 9002:localhost:$1 ubuntu@$TUNNEL
}

# -------------------------------------------------------------------
# MORE PATH STUFF
# -------------------------------------------------------------------

# The next line updates PATH for the Google Cloud SDK.
if [ -f '/Users/dustinmichels/dev/google-cloud-sdk/path.zsh.inc' ]; then . '/Users/dustinmichels/dev/google-cloud-sdk/path.zsh.inc'; fi

# The next line enables shell command completion for gcloud.
if [ -f '/Users/dustinmichels/dev/google-cloud-sdk/completion.zsh.inc' ]; then . '/Users/dustinmichels/dev/google-cloud-sdk/completion.zsh.inc'; fi

# Created by `pipx` on 2022-02-22 23:15:22
export PATH="$PATH:/Users/dustinmichels/.local/bin"

# >>> conda initialize >>>
# # !! Contents within this block are managed by 'conda init' !!
__conda_setup="$('/Users/dustinmichels/opt/anaconda3/bin/conda' 'shell.zsh' 'hook' 2>/dev/null)"
if [ $? -eq 0 ]; then
    eval "$__conda_setup"
else
    if [ -f "/Users/dustinmichels/opt/anaconda3/etc/profile.d/conda.sh" ]; then
        . "/Users/dustinmichels/opt/anaconda3/etc/profile.d/conda.sh"
    else
        export PATH="/Users/dustinmichels/opt/anaconda3/bin:$PATH"
    fi
fi
unset __conda_setup
# <<< conda initialize <<<

# turn off conda by default. Can reactivate with 'conda activate'
conda deactivate

# bun completions
[ -s "/Users/dustinmichels/.bun/_bun" ] && source "/Users/dustinmichels/.bun/_bun"

# Bun
export BUN_INSTALL="/Users/dustinmichels/.bun"
export PATH="$BUN_INSTALL/bin:$PATH"

# cleanup path
typeset -U PATH
