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

# Demo all modes against recent commits
demo:
    @echo "=== tree (default) ==="
    go run ./cmd/git-diff-tree HEAD~3..HEAD
    @echo "\n=== collapsed ==="
    go run ./cmd/git-diff-tree -m collapsed HEAD~3..HEAD
    @echo "\n=== smart ==="
    go run ./cmd/git-diff-tree -m smart HEAD~3..HEAD
    @echo "\n=== topn ==="
    go run ./cmd/git-diff-tree -m topn HEAD~3..HEAD
    @echo "\n=== icicle ==="
    go run ./cmd/git-diff-tree -m icicle HEAD~3..HEAD
    @echo "\n=== brackets ==="
    go run ./cmd/git-diff-tree -m brackets HEAD~3..HEAD
