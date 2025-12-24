# diff-viz - Git diff visualization tool

# Run tests (default)
default: test

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Build the binary
build:
    go build -o git-diff-tree ./cmd/git-diff-tree

# Install to ~/.local/bin
install: build
    mkdir -p ~/.local/bin
    cp git-diff-tree ~/.local/bin/

# Uninstall
uninstall:
    rm -f ~/.local/bin/git-diff-tree

# Clean build artifacts
clean:
    rm -f git-diff-tree

# Format code
fmt:
    go fmt ./...

# Vet and build check
check:
    go vet ./...
    go build ./...

# List available visualization modes
modes:
    go run ./cmd/git-diff-tree --list-modes

# Demo all modes (compares HEAD against root commit)
demo:
    #!/usr/bin/env bash
    ROOT=$(git rev-list --max-parents=0 HEAD)
    echo "=== tree (default) ==="
    go run ./cmd/git-diff-tree "$ROOT..HEAD"
    echo -e "\n=== collapsed ==="
    go run ./cmd/git-diff-tree -m collapsed "$ROOT..HEAD"
    echo -e "\n=== smart ==="
    go run ./cmd/git-diff-tree -m smart "$ROOT..HEAD"
    echo -e "\n=== topn ==="
    go run ./cmd/git-diff-tree -m topn "$ROOT..HEAD"
    echo -e "\n=== icicle ==="
    go run ./cmd/git-diff-tree -m icicle "$ROOT..HEAD"
    echo -e "\n=== brackets ==="
    go run ./cmd/git-diff-tree -m brackets "$ROOT..HEAD"
