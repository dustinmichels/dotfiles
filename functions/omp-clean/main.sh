#!/usr/bin/env zsh
# cleanup old sessions and storage for oh-my-pi

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
  cat <<'EOF'
Usage: omp-clean [OPTIONS]

Clean up Oh-My-Pi storage:
  1. Removes oversized log files (>10MB, >1d old) from ~/.omp/agent
  2. Runs storage garbage collection (sweeps blobs, archives cold sessions, WAL checkpoints)

Options:
  --dry-run                         Simulate cleanup without deleting or modifying files
  --log-size=SIZE                   Size threshold for stale logs (default: 10M)
  --log-days=DAYS                   Minimum age in days for stale logs (default: 1)
  -h, --help                        Show this help message

All additional flags are passed to 'omp gc' (e.g. --cold-archive-after-days=N,
--retain-newest-global=N, --retain-newest-per-cwd=N).
Run 'omp gc --help' for the complete list of storage options.
EOF
  exit 0
fi

if ! command -v omp &>/dev/null; then
  print -u2 "Error: 'omp' command not found in PATH."
  exit 1
fi

setopt localoptions localtraps no_monitor

local target_dir="${PI_CODING_AGENT_DIR:-$HOME/.omp/agent}"
local start_time=$SECONDS
local dry_run=0
local log_size_threshold="10M"
local log_days=1
local -a gc_args

for arg in "$@"; do
  if [[ "$arg" == "--dry-run" ]]; then
    dry_run=1
  elif [[ "$arg" =~ ^--log-size=(.+) ]]; then
    log_size_threshold="${match[1]}"
  elif [[ "$arg" =~ ^--log-days=(.+) ]]; then
    log_days="${match[1]}"
  else
    gc_args+=("$arg")
  fi
done


if ((! dry_run)); then
  gc_args=("--apply" "${gc_args[@]}")
fi

local is_tty=0
if [[ -t 1 && -z "$NO_COLOR" && "$TERM" != "dumb" ]]; then
  is_tty=1
fi

local c_reset="\e[0m"
local c_bold="\e[1m"
local c_cyan="\e[36m"
local c_green="\e[32m"
local c_yellow="\e[33m"
local c_red="\e[31m"
local c_gray="\e[90m"

if ((! is_tty)); then
  c_reset="" c_bold="" c_cyan="" c_green="" c_yellow="" c_red="" c_gray=""
fi

local mode_label=""
((dry_run)) && mode_label=" [dry-run]"

format_bytes() {
  local b=$1
  if ((b >= 1073741824)); then
    printf "%.1f GB" $((b / 1073741824.0))
  elif ((b >= 1048576)); then
    printf "%.1f MB" $((b / 1048576.0))
  elif ((b >= 1024)); then
    printf "%.1f KB" $((b / 1024.0))
  else
    printf "%d B" "$b"
  fi
}

parse_bytes() {
  local s="${1// /}"
  local num unit mult=1
  if [[ "$s" =~ ^([0-9.]+)([a-zA-Z]+)$ ]]; then
    num="${match[1]}"
    unit="${(U)match[2]}"
    case "$unit" in
      B|BYTES) mult=1 ;;
      K|KB|KIB) mult=1024 ;;
      M|MB|MIB) mult=$((1024 * 1024)) ;;
      G|GB|GIB) mult=$((1024 * 1024 * 1024)) ;;
      T|TB|TIB) mult=$((1024 * 1024 * 1024 * 1024)) ;;
    esac
    printf "%.0f" "$((num * mult))"
  else
    echo 0
  fi
}

get_dir_size() {
  local d="$1"
  [[ -d "$d" ]] || { echo 0; return; }
  local kb
  kb=$(du -sk "$d" 2>/dev/null | awk '{print $1}')
  echo $(( ${kb:-0} * 1024 ))
}

local size_before=0
if [[ -d "$target_dir" ]]; then
  size_before=$(get_dir_size "$target_dir")
fi
local step1_bytes=0
local step2_blob_bytes=0
local step2_wal_bytes=0

printf "%b🧹 %b[omp-clean]%b Cleaning Oh-My-Pi storage%b%s%b (%b%s%b)\n" \
  "$c_bold" "$c_cyan" "$c_reset$c_bold" \
  "$c_yellow" "$mode_label" "$c_reset$c_bold" \
  "$c_gray" "$target_dir" "$c_reset"

# Step 1: Oversized logs
local log_label=">${log_size_threshold}, >${log_days}d old"
if [[ -d "$target_dir" ]]; then
  local -a old_logs
  old_logs=(${(f)"$(find "$target_dir" -type f -name "*.log" -size "+${log_size_threshold}" -mtime "+${log_days}" 2>/dev/null)"})
  if ((${#old_logs} > 0)); then
    for f in "${old_logs[@]}"; do
      local sz=$(stat -f %z "$f" 2>/dev/null || echo 0)
      ((step1_bytes += sz))
    done

    local freed_str=$(format_bytes $step1_bytes)

    if ((dry_run)); then
      printf "  %b✔%b [1/2] Stale logs: would remove %d file(s) (%s) (%s)\n" \
        "$c_green" "$c_reset" "${#old_logs}" "$freed_str" "$log_label"
    else
      find "$target_dir" -type f -name "*.log" -size "+${log_size_threshold}" -mtime "+${log_days}" -delete 2>/dev/null
      printf "  %b✔%b [1/2] Stale logs: removed %d file(s) (%s freed) (%s)\n" \
        "$c_green" "$c_reset" "${#old_logs}" "$freed_str" "$log_label"
    fi
  else
    printf "  %b✔%b [1/2] Stale logs: no oversized files found (%s)\n" \
      "$c_green" "$c_reset" "$log_label"
  fi
else
  printf "  %b⚠%b [1/2] Stale logs: directory not found (%s)\n" \
    "$c_yellow" "$c_reset" "$target_dir"
fi

# Step 2: Storage Garbage Collection
local tmp_out
tmp_out=$(mktemp -t omp-clean.XXXXXX) || exit 1
local gc_pid=""
local interrupted=0

cleanup() {
  [[ -n "$gc_pid" ]] && kill $gc_pid 2>/dev/null
  rm -f "$tmp_out"
  ((is_tty)) && printf "\e[?25h"
}
trap cleanup EXIT

on_sigint() {
  interrupted=1
  printf "\n  %b✖%b Interrupted.\n" "$c_red" "$c_reset"
  return 130
}
trap on_sigint INT TERM

local -a cmd
cmd=(omp gc --archive --blobs --wal --cold-archive-after-days=14 "${gc_args[@]}")

"${cmd[@]}" >"$tmp_out" 2>&1 &
gc_pid=$!

local step2_start=$SECONDS
local -a spin
spin=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
local i=1

format_time() {
  local s=$1
  if ((s >= 60)); then
    printf "%dm %02ds" $((s / 60)) $((s % 60))
  else
    printf "%ds" "$s"
  fi
}

if ((is_tty)); then
  printf "\e[?25l"
  while kill -0 $gc_pid 2>/dev/null; do
    if ((interrupted)); then
      break
    fi
    local elapsed=$((SECONDS - step2_start))
    local el_str=$(format_time $elapsed)
    printf "\r\e[K  %b%s%b [2/2] Running storage garbage collection... %b(%s)%b" \
      "$c_cyan" "${spin[$i]}" "$c_reset" "$c_gray" "$el_str" "$c_reset"
    i=$(((i % ${#spin[@]}) + 1))
    sleep 0.1
  done
else
  printf "  [2/2] Running storage garbage collection...\n"
fi

if ((interrupted)); then
  exit 130
fi

wait $gc_pid
local exit_code=$?
gc_pid=""
((is_tty)) && printf "\e[?25h"

local step2_duration=$((SECONDS - step2_start))
local step2_str=$(format_time $step2_duration)

if ((exit_code == 0)); then
  if ((is_tty)); then
    printf "\r\e[K  %b✔%b [2/2] Storage garbage collection complete %b(%s)%b\n" \
      "$c_green" "$c_reset" "$c_gray" "$step2_str" "$c_reset"
  else
    printf "  ✔ [2/2] Storage garbage collection complete (%s)\n" "$step2_str"
  fi

  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    if [[ "$line" =~ ^GC\ (applied|dry-run) ]]; then
      printf "      %b│%b %b%s%b\n" "$c_gray" "$c_reset" "$c_bold" "$line" "$c_reset"
    elif [[ "$line" =~ ^blobs:\ *(.*) ]]; then
      local blob_detail="${match[1]}"
      if [[ "$blob_detail" =~ ^[0-9]+/[0-9]+\ +files,\ *([^,]+), ]]; then
        step2_blob_bytes=$(parse_bytes "${match[1]}")
      fi
      printf "      %b│%b  %bBlobs:%b    %s\n" "$c_gray" "$c_reset" "$c_cyan" "$c_reset" "$blob_detail"
    elif [[ "$line" =~ ^sessions:\ *(.*) ]]; then
      printf "      %b│%b  %bSessions:%b %s\n" "$c_gray" "$c_reset" "$c_cyan" "$c_reset" "${match[1]}"
    elif [[ "$line" =~ ^sessions\ skipped\ active:\ *(.*) ]]; then
      printf "      %b│%b  %bActive:%b   %s kept active\n" "$c_gray" "$c_reset" "$c_yellow" "$c_reset" "${match[1]}"
    elif [[ "$line" =~ ^wal:\ *(.*) ]]; then
      local wal_detail="${match[1]}"
      if [[ "$wal_detail" =~ (checkpointed|checkpoint\ dry-run),\ *([0-9.]+[A-Za-z]+)\ across ]]; then
        step2_wal_bytes=$(parse_bytes "${match[2]}")
      fi
      printf "      %b│%b  %bWAL:%b      %s\n" "$c_gray" "$c_reset" "$c_cyan" "$c_reset" "$wal_detail"
    elif [[ "$line" =~ (error|failed|Error|Failed) ]]; then
      printf "      %b│%b  %b%s%b\n" "$c_gray" "$c_reset" "$c_red" "$line" "$c_reset"
    else
      printf "      %b│%b  %s\n" "$c_gray" "$c_reset" "$line"
    fi
  done <"$tmp_out"
else
  if ((is_tty)); then
    printf "\r\e[K  %b✖%b [2/2] Storage garbage collection failed %b(%s, exit %d)%b\n" \
      "$c_red" "$c_reset" "$c_gray" "$step2_str" "$exit_code" "$c_reset"
  else
    printf "  ✖ [2/2] Storage garbage collection failed (%s, exit %d)\n" "$step2_str" "$exit_code"
  fi
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    printf "      %b│%b  %b%s%b\n" "$c_gray" "$c_reset" "$c_red" "$line" "$c_reset"
  done <"$tmp_out"
fi

local total_duration=$((SECONDS - start_time))
local total_str=$(format_time $total_duration)

if ((exit_code == 0)); then
  local total_saved_bytes=0
  if ((dry_run)); then
    total_saved_bytes=$((step1_bytes + step2_blob_bytes + step2_wal_bytes))
    local saved_str=$(format_bytes $total_saved_bytes)
    printf "%b✨ Cleaned in %s (%s would be saved)%b\n" "$c_green$c_bold" "$total_str" "$saved_str" "$c_reset"
  else
    local size_after=0
    [[ -d "$target_dir" ]] && size_after=$(get_dir_size "$target_dir")
    local size_delta=$((size_before - size_after))
    local known_bytes=$((step1_bytes + step2_blob_bytes + step2_wal_bytes))
    if ((size_delta > known_bytes)); then
      total_saved_bytes=$size_delta
    else
      total_saved_bytes=$known_bytes
    fi
    ((total_saved_bytes < 0)) && total_saved_bytes=0
    local saved_str=$(format_bytes $total_saved_bytes)
    printf "%b✨ Cleaned in %s (%s saved)%b\n" "$c_green$c_bold" "$total_str" "$saved_str" "$c_reset"
  fi
fi

exit $exit_code
