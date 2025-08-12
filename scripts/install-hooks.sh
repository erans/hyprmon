#!/bin/bash

# Install git hooks for HyprMon project

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
GIT_HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "Installing git hooks for HyprMon..."

# Check if we're in a git repository
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo -e "${RED}Error: Not in a git repository${NC}"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p "$GIT_HOOKS_DIR"

# Create pre-commit hook
cat > "$GIT_HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash

# HyprMon pre-commit hook
# Runs gofmt check before allowing commit

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Running pre-commit checks..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Warning: Go is not installed, skipping Go checks${NC}"
    exit 0
fi

# Run gofmt check
echo "Checking code formatting with gofmt..."
GOFMT_OUTPUT=$(go fmt ./... 2>&1)

if [ -n "$GOFMT_OUTPUT" ]; then
    echo -e "${RED}❌ Code formatting issues found!${NC}"
    echo -e "${RED}The following files need formatting:${NC}"
    echo "$GOFMT_OUTPUT"
    echo ""
    echo -e "${YELLOW}To fix: run 'make fmt' or 'go fmt ./...'${NC}"
    echo -e "${YELLOW}To bypass: use 'git commit --no-verify'${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Code formatting check passed${NC}"

# Run go vet
echo "Running go vet..."
if ! go vet ./... > /dev/null 2>&1; then
    echo -e "${RED}❌ go vet found issues!${NC}"
    echo "Run 'go vet ./...' to see details"
    echo -e "${YELLOW}To bypass: use 'git commit --no-verify'${NC}"
    exit 1
fi

echo -e "${GREEN}✓ go vet check passed${NC}"

# Check for binary files in bin/
if [ -f "bin/hyprmon" ]; then
    echo -e "${YELLOW}⚠ Warning: Binary file found in bin/hyprmon${NC}"
    echo "Consider removing it before committing: rm bin/hyprmon"
fi

# Check for large files (over 1MB)
LARGE_FILES=$(find . -type f -size +1M -not -path "./.git/*" -not -path "./bin/*" -not -path "./vendor/*" 2>/dev/null)
if [ -n "$LARGE_FILES" ]; then
    echo -e "${YELLOW}⚠ Warning: Large files detected (>1MB):${NC}"
    echo "$LARGE_FILES"
fi

echo -e "${GREEN}✓ All pre-commit checks passed!${NC}"
EOF

# Make pre-commit hook executable
chmod +x "$GIT_HOOKS_DIR/pre-commit"

# Create pre-push hook (optional - runs tests)
cat > "$GIT_HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash

# HyprMon pre-push hook
# Runs tests before pushing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Running pre-push checks..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Warning: Go is not installed, skipping Go checks${NC}"
    exit 0
fi

# Run tests if they exist
if ls *_test.go &> /dev/null; then
    echo "Running tests..."
    if ! go test -short ./... > /dev/null 2>&1; then
        echo -e "${RED}❌ Tests failed!${NC}"
        echo "Run 'go test ./...' to see details"
        echo -e "${YELLOW}To bypass: use 'git push --no-verify'${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Tests passed${NC}"
fi

# Build check
echo "Checking if project builds..."
if ! go build -o /tmp/hyprmon-test > /dev/null 2>&1; then
    echo -e "${RED}❌ Build failed!${NC}"
    echo "Run 'go build' to see details"
    exit 1
fi
rm -f /tmp/hyprmon-test
echo -e "${GREEN}✓ Build check passed${NC}"

echo -e "${GREEN}✓ All pre-push checks passed!${NC}"
EOF

# Make pre-push hook executable
chmod +x "$GIT_HOOKS_DIR/pre-push"

echo -e "${GREEN}✓ Git hooks installed successfully!${NC}"
echo ""
echo "Installed hooks:"
echo "  - pre-commit: Runs gofmt and go vet"
echo "  - pre-push: Runs tests and build check"
echo ""
echo "To skip hooks temporarily, use --no-verify flag:"
echo "  git commit --no-verify"
echo "  git push --no-verify"
echo ""
echo "To uninstall hooks:"
echo "  rm .git/hooks/pre-commit .git/hooks/pre-push"