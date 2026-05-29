#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "🍏 Starting Homebrew reset process..."

# 1. Verify the Brewfile exists before doing anything drastic
if [ ! -f "Brewfile" ]; then
    echo "❌ Error: Brewfile not found in the current directory."
    echo "Please run 'brew bundle dump' first or navigate to the correct folder."
    exit 1
fi

echo "⚠️  This will uninstall ALL Homebrew packages, casks, and repositories."
read -p "Are you sure you want to continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Operation cancelled."
    exit 1
fi

# 2. Uninstall all installed formulae and casks
echo "🧹 Uninstalling all currently installed Homebrew packages..."
# Using 'force' ensures we remove all versions, and 'zap' cleanly removes casks
brew remove --force $(brew list --formula) 2>/dev/null || echo "No formulae to remove."
brew remove --cask --force $(brew list --cask) 2>/dev/null || echo "No casks to remove."

# 3. Clean up untracked dependencies, caches, and dead symlinks
echo "🧼 Cleaning up leftover files and caches..."
brew cleanup --prune=all

# 4. Reinstall everything from the Brewfile
echo "🚀 Reinstalling everything from your Brewfile..."
brew bundle

echo "✅ Homebrew environment successfully restored!"