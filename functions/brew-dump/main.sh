#!/usr/bin/env zsh
# Dump Homebrew dependencies into a formatted Brewfile.
# Adds newlines between sections and formats descriptions inline after items.

set -o pipefail

usage() {
  cat <<'EOF'
Usage: brew-dump [OPTIONS] [BREW_ARGS...]

Dump Homebrew dependencies to a formatted Brewfile.
Similar to 'brew bundle dump --global --force', but adds blank lines
between sections and aligns package descriptions inline on the same line.

Options:
  -o, --stdout        Print formatted Brewfile to stdout instead of writing to file
  -f, --file=FILE     Write to FILE instead of the global Brewfile
  -q, --quiet         Suppress status output
  -n, --dry-run       Print what would be written without modifying any file
  -h, --help          Show this help message

All additional arguments are forwarded directly to 'brew bundle dump'
(e.g. --no-vscode, --no-cask, --mas, etc.).
EOF
}

if ! command -v brew &>/dev/null; then
  print -u2 "Error: 'brew' command not found in PATH."
  exit 1
fi

local to_stdout=0
local dry_run=0
local quiet=0
local target_file=""
local -a brew_args

while (( $# > 0 )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    -o|--stdout)
      to_stdout=1
      shift
      ;;
    -n|--dry-run)
      dry_run=1
      shift
      ;;
    -q|--quiet)
      quiet=1
      shift
      ;;
    -f|--file)
      if (( $# < 2 )); then
        print -u2 "Error: Option $1 requires a file path argument."
        exit 1
      fi
      target_file="$2"
      shift 2
      ;;
    --file=*)
      target_file="${1#*=}"
      shift
      ;;
    *)
      brew_args+=("$1")
      shift
      ;;
  esac
done

if [[ -z "$target_file" ]]; then
  if [[ -n "$HOMEBREW_BUNDLE_FILE_GLOBAL" ]]; then
    target_file="$HOMEBREW_BUNDLE_FILE_GLOBAL"
  elif [[ -n "$XDG_CONFIG_HOME" && -f "$XDG_CONFIG_HOME/homebrew/Brewfile" ]]; then
    target_file="$XDG_CONFIG_HOME/homebrew/Brewfile"
  elif [[ -f "$HOME/.config/homebrew/Brewfile" ]]; then
    target_file="$HOME/.config/homebrew/Brewfile"
  elif [[ -f "$HOME/.homebrew/Brewfile" ]]; then
    target_file="$HOME/.homebrew/Brewfile"
  else
    target_file="$HOME/.Brewfile"
  fi
fi

# Expand tilde if present in user-supplied path
target_file="${target_file/#\~/$HOME}"

local tmp_raw
local tmp_formatted
tmp_raw=$(mktemp -t brew_dump_raw.XXXXXX) || exit 1
tmp_formatted=$(mktemp -t brew_dump_formatted.XXXXXX) || { rm -f "$tmp_raw"; exit 1; }

cleanup() {
  rm -f "$tmp_raw" "$tmp_formatted"
}
trap cleanup EXIT INT TERM

# Run brew bundle dump to stdout (captured into tmp_raw)
if ! brew bundle dump --file=- --quiet "${brew_args[@]}" > "$tmp_raw"; then
  print -u2 "Error: 'brew bundle dump' failed."
  exit 1
fi

format_brewfile() {
  local in_file="$1"
  local out_file="$2"

  typeset -a section_items section_comments
  local current_type=""
  local pending_comments=""
  local have_printed_section=0

  flush_section() {
    if (( ${#section_items} == 0 )); then
      return
    fi

    if (( have_printed_section )); then
      print "" >> "$out_file"
    fi
    have_printed_section=1

    local max_len=0
    local i
    for i in {1..${#section_items}}; do
      local item="${section_items[i]}"
      local comment="${section_comments[i]}"
      if [[ -n "$comment" ]]; then
        if (( ${#item} > max_len )); then
          max_len=${#item}
        fi
      fi
    done

    for i in {1..${#section_items}}; do
      local item="${section_items[i]}"
      local comment="${section_comments[i]}"
      if [[ -n "$comment" ]]; then
        local pad=$(( max_len - ${#item} + 1 ))
        local spaces=$(printf "%*s" $pad "")
        print -r -- "${item}${spaces}# ${comment}" >> "$out_file"
      else
        print -r -- "${item}" >> "$out_file"
      fi
    done

    section_items=()
    section_comments=()
  }

  while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip empty lines from input
    if [[ -z "${line// }" ]]; then
      continue
    fi

    # Description comment line
    if [[ "$line" =~ "^#[[:space:]]*(.*)" ]]; then
      local desc="${match[1]}"
      desc="${desc%%[[:space:]]#}"
      if [[ -n "$pending_comments" ]]; then
        pending_comments="$pending_comments $desc"
      else
        pending_comments="$desc"
      fi
      continue
    fi

    # Entry line: starts with identifier and whitespace (e.g. brew "...", cask "...")
    if [[ "$line" =~ "^([a-zA-Z0-9_-]+)[[:space:]]" ]]; then
      local item_type="${match[1]}"
      if [[ "$item_type" != "$current_type" ]]; then
        flush_section
        current_type="$item_type"
      fi
      section_items+=("$line")
      section_comments+=("$pending_comments")
      pending_comments=""
    else
      # Any other line: flush section, output pending comments, output line
      flush_section
      current_type=""
      if [[ -n "$pending_comments" ]]; then
        print -r -- "# $pending_comments" >> "$out_file"
        pending_comments=""
      fi
      print -r -- "$line" >> "$out_file"
    fi
  done < "$in_file"

  flush_section

  # If any trailing comments remained
  if [[ -n "$pending_comments" ]]; then
    print -r -- "# $pending_comments" >> "$out_file"
  fi
}

format_brewfile "$tmp_raw" "$tmp_formatted"

if (( dry_run )); then
  if (( ! quiet )); then
    print "Dry-run: Would write formatted Brewfile to $target_file\n"
  fi
  cat "$tmp_formatted"
elif (( to_stdout )); then
  cat "$tmp_formatted"
else
  mkdir -p "$(dirname "$target_file")"
  # Overwrite target in-place to preserve hard links and symlinks
  cat "$tmp_formatted" > "$target_file"
  if (( ! quiet )); then
    print "🍏 Successfully dumped and formatted Brewfile to $target_file"
  fi
fi
