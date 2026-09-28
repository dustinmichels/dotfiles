# Output directory — must be on PATH (see ~/.zshrc PATH & TOOLS: export PATH="$HOME/dev/bin:$PATH")
BIN := $(HOME)/dev/bin

# Discover all subdirectories as tool names.
# $(wildcard */) finds dirs like "gh-init/", then patsubst strips the trailing slash.
TOOLS := $(patsubst %/,%,$(wildcard */))

# Declare phony targets so make always runs them, even if a file with the same name exists.
.PHONY: all clean $(TOOLS)

# Default target: build/install everything.
all: $(TOOLS)

# One rule handles every tool. $@ expands to the current target name (e.g. "gh-init").
# Usage: make          → builds/installs all
#        make gh-init  → builds one
$(TOOLS):
	@mkdir -p $(BIN)
	@if [ -f "$@/go.mod" ] || [ -f "$@/main.go" ]; then \
		echo "Building $@ (Go)..."; \
		cd $@ && go build -o $(BIN)/$@ .; \
	elif [ -f "$@/main.sh" ]; then \
		echo "Installing $@ (shell)..."; \
		install -m 755 $@/main.sh $(BIN)/$@; \
	elif [ -f "$@/$@" ]; then \
		echo "Installing $@ (shell)..."; \
		install -m 755 $@/$@ $(BIN)/$@; \
	elif [ -f "$@/$@.sh" ]; then \
		echo "Installing $@ (shell)..."; \
		install -m 755 $@/$@.sh $(BIN)/$@; \
	else \
		echo "Error: Unknown tool type in $@ (no main.go, main.sh, $@, or $@.sh found)" >&2; \
		exit 1; \
	fi
# Remove all compiled binaries from dev/bin.
clean:
	rm -f $(addprefix $(BIN)/, $(TOOLS))
