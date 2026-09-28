#  _____    _
# |__  /___| |__  _ __ ___
#   / // __| '_ \| '__/ __|
#  / /_\__ \ | | | | | (__
# /____|___/_| |_|_|  \___|

# Prevent duplicate entries in PATH automatically
typeset -U path PATH

# -------------------------------------------------------------------
# OH-MY-ZSH
# -------------------------------------------------------------------

export ZSH="$HOME/.oh-my-zsh"

ZSH_THEME="robbyrussell"

plugins=(
  git
  mise
  zoxide
  zsh-autosuggestions
)

source $ZSH/oh-my-zsh.sh

# -------------------------------------------------------------------
# FUNCTIONS
# -------------------------------------------------------------------

# */ See external IP address */
function exip() {
  curl -s https://icanhazip.com
}

# */ Create a new directory and enter it */
function mkd() {
  mkdir -p "$@" && cd "${@: -1}"
}

# */ See 10 biggest items in current directory (top-level only) */
function biggest() {
  du -sh * .*(N) 2>/dev/null | sort -rh | head -10
}

# */ Source env files (Thanks, Taylor!)
# */
# */ Example usage:
# */    envup && go run .
# */    envup production && go run .
function envup() {
  local file
  if [ "$1" = "-f" ]; then
    file="$2"
  elif [ -n "$1" ]; then
    file=".env.$1"
  else
    file=".env"
  fi
  if [ -f "$file" ]; then
    local IFS=$'\n'
    local v
    local -a env_vars=($(sed '/^#.*/d; /^[[:space:]]*$/d; s/^export //' "$file"))
    for v in $env_vars; do
      eval export "$v"
    done
  else
    echo "$file does not exist" >&2
    return 1
  fi
}

# /* gi - gitignore
# /*
# /* eg, gi vue,python,macos >> .gitignore
function gi() { curl -sLw "\n" "https://www.toptal.com/developers/gitignore/api/$*"; }

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

# My own dev tools / functions (functions/Makefile)
export PATH="$HOME/dev/bin:$PATH"

# User binaries (uv tools, claude)
export PATH="$PATH:$HOME/.local/bin"

# Bun global tools (omp)
export PATH="$PATH:$HOME/.bun/bin"

# Bun completions
[ -s "$HOME/.bun/_bun" ] && source "$HOME/.bun/_bun"
