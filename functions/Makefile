# Output directory — must be on PATH (see ~/.zshrc: export PATH="$PATH:$HOME/dev/bin")
BIN := $(HOME)/dev/bin

# Discover all subdirectories as tool names.
# $(wildcard */) finds dirs like "gh-init/", then patsubst strips the trailing slash.
TOOLS := $(patsubst %/,%,$(wildcard */))

# Declare phony targets so make always runs them, even if a file with the same name exists.
.PHONY: all clean $(TOOLS)

# Default target: build everything.
all: $(TOOLS)

# One rule handles every tool. $@ expands to the current target name (e.g. "gh-init").
# Usage: make          → builds all
#        make gh-init  → builds one
$(TOOLS):
	@echo "Building $@..."
	cd $@ && go build -o $(BIN)/$@ .

# Remove all compiled binaries from dev/bin.
clean:
	rm -f $(addprefix $(BIN)/, $(TOOLS))
