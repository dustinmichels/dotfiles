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

# */ Source various env files (Thanks, Taylor)
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
function gi() { curl -sLw "\n" https://www.toptal.com/developers/gitignore/api/$@; }

# cleanup old sessions and storage for oh-my-pi (see functions/omp-clean)
alias ompgc=omp-clean

# delegate global npm installs to mise
npm() {
  if ((${argv[(I)(i|install)]} && ${argv[(I)(-g|--global)]})); then
    local -a pkgs=(${argv:#(-*|i|install)})
    if (($#pkgs)); then
      local -a targets=(npm:${^pkgs#npm:})
      echo "Delegating to mise: mise use -g $targets"
      mise use -g "${targets[@]}"
      return
    fi
  fi
  command npm "$@"
}

# -------------------------------------------------------------------
# PATH & TOOLS
# -------------------------------------------------------------------

# Added by Antigravity
export PATH="/Users/dustinmichels/.antigravity/antigravity/bin:$PATH"

# my personal dev tools (functions)
export PATH="$PATH:$HOME/dev/bin"

# eval "$(mise activate zsh)"
eval "$(zoxide init zsh)"

# Claude
export PATH="$PATH:$HOME/.local/bin"

# Bun (for OMP)
export PATH="/Users/dustinmichels/.bun/bin:$PATH"

# bun completions
[ -s "/Users/dustinmichels/.bun/_bun" ] && source "/Users/dustinmichels/.bun/_bun"

# cleanup path
typeset -U PATH
