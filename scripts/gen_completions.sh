#!/bin/sh
set -e

# Generate shell completion files for packaging
# These files are created at build time and included in packages

echo "Generating shell completion files..."

# Create completions directory if it doesn't exist
rm -rf completions
mkdir -p completions

# Generate bash completion
echo "  - bash completion"
go run . completion bash > completions/cloudfuse.bash

# Generate zsh completion
echo "  - zsh completion"
go run . completion zsh > completions/_cloudfuse

# Generate fish completion
echo "  - fish completion"
go run . completion fish > completions/cloudfuse.fish

echo "Shell completion files generated in completions/"
