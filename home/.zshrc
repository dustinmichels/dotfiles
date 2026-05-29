#  _____    _
# |__  /___| |__  _ __ ___
#   / // __| '_ \| '__/ __|
#  / /_\__ \ | | | | | (__
# /____|___/_| |_|_|  \___|

# -------------------------------------------------------------------
# OH-MY-ZSH
# -------------------------------------------------------------------

export ZSH="$HOME/.oh-my-zsh"

ZSH_THEME="robbyrussell"

plugins=(
  git
  mise
  zsh-autosuggestions
)

source $ZSH/oh-my-zsh.sh

# -------------------------------------------------------------------
# USER CONFIG
# -------------------------------------------------------------------

# Preferred editor for local and remote sessions
# if [[ -n $SSH_CONNECTION ]]; then
#   export EDITOR='vim'
# else
#   export EDITOR='mvim'
# fi


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

# gi - gitignore
# eg,
#     gi vue,python,macos >> .gitignore
function gi() { curl -sLw "\n" https://www.toptal.com/developers/gitignore/api/$@ ;}

# -------------------------------------------------------------------
# PATH & TOOLS
# -------------------------------------------------------------------

# Added by Antigravity
export PATH="/Users/dustinmichels/.antigravity/antigravity/bin:$PATH"

# connect4
export PATH="$PATH:$HOME/GitRepos/connect4/bin"

# eval "$(mise activate zsh)"
eval "$(zoxide init zsh)"

# Claude?
export PATH="$PATH:$HOME/.local/bin"

# cleanup path
typeset -U PATH
