# omp-clean

Maintenance script to clean up Oh-My-Pi (`omp`) coding agent local storage and logs.

Executes a two-step cleanup process:

1. Removes oversized `.log` files from `~/.omp/agent` (default: >10MB and >1 day old).
2. Runs `omp gc` storage garbage collection with progress feedback (sweeps unreferenced blobs, archives cold sessions older than 14 days, and checkpoints the SQLite WAL).

## Prerequisites

- [Oh-My-Pi (`omp`)](https://github.com/can1357/oh-my-pi) installed and available in PATH.

## Installation

Install to `~/dev/bin` via the functions Makefile:

```sh
cd functions
make omp-clean
```

## Usage

```sh
# Run standard cleanup and garbage collection
omp-clean

# Dry-run: preview stale logs and GC savings without modifying files
omp-clean --dry-run

# Custom thresholds for log pruning
omp-clean --log-size=20M     # prune logs larger than 20MB (default: 10M)
omp-clean --log-days=7       # prune logs older than 7 days (default: 1)

# Pass additional options directly to 'omp gc'
omp-clean --cold-archive-after-days=30
omp-clean --retain-newest-global=20
```
