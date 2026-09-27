#!/usr/bin/env zsh
# Dump Homebrew dependencies into a formatted Brewfile.
# Adds newlines between sections and formats descriptions inline after items.

set -o pipefail

usage() {
	cat <<'EOF'
Usage: brew-dump [OPTIONS] [BREW_ARGS...]

Dump Homebrew dependencies to a formatted Brewfile.
Similar to 'brew bundle dump --global --force', but adds blank lines
between sections, aligns package descriptions inline with installation
dates, and sorts formulae and casks by most recent first.

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

while (($# > 0)); do
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	-o | --stdout)
		to_stdout=1
		shift
		;;
	-n | --dry-run)
		dry_run=1
		shift
		;;
	-q | --quiet)
		quiet=1
		shift
		;;
	-f | --file)
		if (($# < 2)); then
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
tmp_formatted=$(mktemp -t brew_dump_formatted.XXXXXX) || {
	rm -f "$tmp_raw"
	exit 1
}

cleanup() {
	rm -f "$tmp_raw" "$tmp_formatted"
}
trap cleanup EXIT INT TERM

# Run brew bundle dump to stdout (captured into tmp_raw)
if ! brew bundle dump --file=- --quiet "${brew_args[@]}" >"$tmp_raw"; then
	print -u2 "Error: 'brew bundle dump' failed."
	exit 1
fi

format_brewfile() {
	local in_file="$1"
	local out_file="$2"

	local brew_prefix="${HOMEBREW_PREFIX:-}"
	if [[ -z "$brew_prefix" ]]; then
		if command -v brew &>/dev/null; then
			brew_prefix="$(brew --prefix 2>/dev/null)"
		elif [[ -d "/opt/homebrew" ]]; then
			brew_prefix="/opt/homebrew"
		elif [[ -d "/usr/local" ]]; then
			brew_prefix="/usr/local"
		fi
	fi
	local cellar_dir="$brew_prefix/Cellar"
	local cask_dir="$brew_prefix/Caskroom"

	typeset -A pkg_epochs pkg_dates

	# Read initial install dates (birthtimes) from Cellar and Caskroom
	if stat -f "%B" / &>/dev/null; then
		local stat_output
		stat_output="$(stat -f "%N%t%B%t%SB" -t "%-m/%-d/%Y" "$cellar_dir"/*(N/) "$cask_dir"/*(N/) 2>/dev/null)"
		local sline
		for sline in "${(@f)stat_output}"; do
			[[ -z "$sline" ]] && continue
			local -a parts
			parts=("${(@ps:\t:)sline}")
			local name="${parts[1]:t}"
			local ep="${parts[2]}"
			local dt="${parts[3]}"
			if [[ "$ep" == "0" || -z "$ep" ]]; then
				local fallback
				fallback="$(stat -f "%m%t%Sm" -t "%-m/%-d/%Y" "${parts[1]}" 2>/dev/null)"
				if [[ -n "$fallback" ]]; then
					local -a fb_parts
					fb_parts=("${(@ps:\t:)fallback}")
					ep="${fb_parts[1]}"
					dt="${fb_parts[2]}"
				fi
			fi
			if [[ -n "$ep" && "$ep" != "0" ]]; then
				pkg_epochs[$name]="$ep"
				pkg_dates[$name]="$dt"
			fi
		done
	fi

	typeset -a section_items section_comments
	local current_type=""
	local pending_comments=""
	local have_printed_section=0

	flush_section() {
		if ((${#section_items} == 0)); then
			return
		fi

		if ((have_printed_section)); then
			print "" >>"$out_file"
		fi
		have_printed_section=1

		local i
		# For brew and cask sections: append date to comment and sort most recent first
		if [[ "$current_type" == "brew" || "$current_type" == "cask" ]]; then
			typeset -a sort_keys
			for i in {1..${#section_items}}; do
				local item="${section_items[i]}"
				local comment="${section_comments[i]}"
				local pkg=""
				if [[ "$item" =~ "^(brew|cask)[[:space:]]+\"([^\"]+)\"" ]]; then
					pkg="${match[2]}"
				fi
				local base="${pkg:t}"
				local date_str="${pkg_dates[$pkg]:-${pkg_dates[$base]:-}}"
				local epoch="${pkg_epochs[$pkg]:-${pkg_epochs[$base]:-0}}"

				if [[ -n "$date_str" ]]; then
					if [[ -n "$comment" ]]; then
						comment="${comment} (${date_str})"
					else
						comment="(${date_str})"
					fi
					section_comments[i]="$comment"
				fi

				local inv=$((999999999999 - epoch))
				sort_keys+=("$(printf "%012d:%05d:%d" $inv $i $i)")
			done

			typeset -a new_items new_comments
			local k
			for k in ${(o)sort_keys}; do
				local idx="${k##*:}"
				new_items+=("${section_items[idx]}")
				new_comments+=("${section_comments[idx]}")
			done
			section_items=("${new_items[@]}")
			section_comments=("${new_comments[@]}")
		fi

		local max_len=0
		for i in {1..${#section_items}}; do
			local item="${section_items[i]}"
			local comment="${section_comments[i]}"
			if [[ -n "$comment" ]]; then
				if ((${#item} > max_len)); then
					max_len=${#item}
				fi
			fi
		done

		for i in {1..${#section_items}}; do
			local item="${section_items[i]}"
			local comment="${section_comments[i]}"
			if [[ -n "$comment" ]]; then
				local pad=$((max_len - ${#item} + 1))
				local spaces=$(printf "%*s" $pad "")
				print -r -- "${item}${spaces}# ${comment}" >>"$out_file"
			else
				print -r -- "${item}" >>"$out_file"
			fi
		done

		section_items=()
		section_comments=()
	}

	while IFS= read -r line || [[ -n "$line" ]]; do
		# Skip empty lines from input
		if [[ -z "${line// /}" ]]; then
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
			local item_line="$line"
			if [[ -z "$pending_comments" && "$line" =~ "^([^#]+)[[:space:]]+#[[:space:]]*(.*)" ]]; then
				item_line="${match[1]}"
				item_line="${item_line%%[[:space:]]#}"
				local inline_desc="${match[2]}"
				setopt local_options extended_glob
				inline_desc="${inline_desc%[[:space:]]##\([0-9]##/[0-9]##/[0-9]##\)}"
				[[ "$inline_desc" =~ "^\([0-9]+/[0-9]+/[0-9]+\)$" ]] && inline_desc=""
				pending_comments="$inline_desc"
			fi
			section_items+=("$item_line")
			section_comments+=("$pending_comments")
			pending_comments=""
		else
			# Any other line: flush section, output pending comments, output line
			flush_section
			current_type=""
			if [[ -n "$pending_comments" ]]; then
				print -r -- "# $pending_comments" >>"$out_file"
				pending_comments=""
			fi
			print -r -- "$line" >>"$out_file"
		fi
	done <"$in_file"

	flush_section

	# If any trailing comments remained
	if [[ -n "$pending_comments" ]]; then
		print -r -- "# $pending_comments" >>"$out_file"
	fi
}

format_brewfile "$tmp_raw" "$tmp_formatted"

if ((dry_run)); then
	if ((! quiet)); then
		print "Dry-run: Would write formatted Brewfile to $target_file\n"
	fi
	cat "$tmp_formatted"
elif ((to_stdout)); then
	cat "$tmp_formatted"
else
	mkdir -p "$(dirname "$target_file")"
	# Overwrite target in-place to preserve hard links and symlinks
	cat "$tmp_formatted" >"$target_file"
	if ((! quiet)); then
		print "🍏 Successfully dumped and formatted Brewfile to $target_file"
	fi
fi
