#!/bin/bash

# HyprMon Release Script
# This script helps create a new release by:
# 1. Showing the current version
# 2. Suggesting the next version
# 3. Creating changelog
# 4. Creating and pushing tags
# 5. Triggering GitHub release

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    color=$1
    shift
    echo -e "${color}$@${NC}"
}

# Function to get the latest tag
get_latest_tag() {
    git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"
}

# Function to increment version
increment_version() {
    local version=$1
    local part=$2
    
    # Remove 'v' prefix if present
    version="${version#v}"
    
    # Split version into parts
    IFS='.' read -r major minor patch <<< "$version"
    
    case $part in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac
    
    echo "v${major}.${minor}.${patch}"
}

# Function to generate changelog using Claude Code
generate_changelog() {
    local from_tag=$1
    local to_ref=${2:-HEAD}
    
    echo "## Analyzing changes since ${from_tag}..."
    echo ""
    
    # Check if npx is available
    if ! command -v npx &> /dev/null; then
        print_color $YELLOW "npx not found, falling back to git log"
        echo "### All Changes"
        git log ${from_tag}..${to_ref} --pretty=format:"- %s (%an)" --no-merges
        return
    fi
    
    # Get commit messages
    local commits=$(git log ${from_tag}..${to_ref} --pretty=format:"- %s" --no-merges)
    
    # Get list of changed files
    local changed_files=$(git diff --name-only ${from_tag}..${to_ref})
    
    # Get summary statistics
    local stats=$(git diff --shortstat ${from_tag}..${to_ref})
    
    # Create prompt for Claude Code
    local prompt="Generate a user-friendly changelog for HyprMon release. Analyze these changes and create a concise changelog:

STATISTICS:
${stats}

CHANGED FILES:
${changed_files}

COMMITS:
${commits}

Create a changelog with these sections (only include sections that apply):
- Brief summary (1-2 sentences about this release)
- New Features (user-facing features)
- Improvements (enhancements to existing features)  
- Bug Fixes (if any)
- Breaking Changes (if any)

Keep it concise and focus on what users care about. Don't mention internal refactoring unless it affects users.
Output markdown format only, no extra explanation."

    # Use Claude Code to analyze changes
    print_color $BLUE "Using Claude Code to generate intelligent changelog..."
    
    # Run Claude Code with the prompt
    local changelog=$(echo "$prompt" | npx -y @anthropic-ai/claude-code@latest 2>/dev/null || echo "")
    
    if [ -z "$changelog" ]; then
        # Try alternative: just show git diff summary and commits
        print_color $YELLOW "Claude Code not available, generating basic changelog"
        echo "### Changes in this release"
        echo ""
        echo "${stats}"
        echo ""
        echo "### Commits"
        git log ${from_tag}..${to_ref} --pretty=format:"- %s" --no-merges
    else
        echo "$changelog"
    fi
}

# Main script
print_color $BLUE "==================================="
print_color $BLUE "    HyprMon Release Tool"
print_color $BLUE "==================================="
echo ""

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_color $RED "Error: Not in a git repository"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    print_color $YELLOW "Warning: You have uncommitted changes"
    read -p "Do you want to continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_color $RED "Aborted"
        exit 1
    fi
fi

# Get current version
LATEST_TAG=$(get_latest_tag)
print_color $GREEN "Current version: ${LATEST_TAG}"

# Check if on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
    print_color $YELLOW "Warning: You are on branch '${CURRENT_BRANCH}', not 'main' or 'master'"
    read -p "Do you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_color $RED "Aborted"
        exit 1
    fi
fi

# Show recent commits since last tag
echo ""
print_color $BLUE "Recent commits since ${LATEST_TAG}:"
git log ${LATEST_TAG}..HEAD --pretty=format:"  - %s" --no-merges | head -10
echo ""

# Count commits since last release
COMMIT_COUNT=$(git rev-list ${LATEST_TAG}..HEAD --count)
print_color $GREEN "Total commits since last release: ${COMMIT_COUNT}"
echo ""

# Suggest next version
PATCH_VERSION=$(increment_version $LATEST_TAG patch)
MINOR_VERSION=$(increment_version $LATEST_TAG minor)
MAJOR_VERSION=$(increment_version $LATEST_TAG major)

print_color $BLUE "Suggested versions:"
echo "  1) Patch release: ${PATCH_VERSION} (bug fixes, small changes)"
echo "  2) Minor release: ${MINOR_VERSION} (new features, backwards compatible)"
echo "  3) Major release: ${MAJOR_VERSION} (breaking changes)"
echo "  4) Custom version"
echo ""

read -p "Select version type (1-4): " VERSION_CHOICE

case $VERSION_CHOICE in
    1)
        NEW_VERSION=$PATCH_VERSION
        ;;
    2)
        NEW_VERSION=$MINOR_VERSION
        ;;
    3)
        NEW_VERSION=$MAJOR_VERSION
        ;;
    4)
        read -p "Enter custom version (e.g., v1.2.3): " NEW_VERSION
        if [[ ! $NEW_VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            print_color $RED "Error: Invalid version format. Use vX.Y.Z"
            exit 1
        fi
        ;;
    *)
        print_color $RED "Invalid selection"
        exit 1
        ;;
esac

# Confirm version
echo ""
print_color $YELLOW "New version will be: ${NEW_VERSION}"
read -p "Is this correct? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_color $RED "Aborted"
    exit 1
fi

# Check if tag already exists
if git rev-parse "$NEW_VERSION" >/dev/null 2>&1; then
    print_color $RED "Error: Tag ${NEW_VERSION} already exists"
    exit 1
fi

# Generate changelog
echo ""
print_color $BLUE "Generating changelog..."
CHANGELOG=$(generate_changelog $LATEST_TAG)
echo "$CHANGELOG"
echo ""

# Ask for release notes
print_color $BLUE "Would you like to add custom release notes?"
read -p "Enter notes (or press Enter to skip): " CUSTOM_NOTES

# Create release notes file
RELEASE_NOTES="# Release ${NEW_VERSION}

$(date +"%Y-%m-%d")

${CHANGELOG}
"

if [ -n "$CUSTOM_NOTES" ]; then
    RELEASE_NOTES="${RELEASE_NOTES}

## Release Notes
${CUSTOM_NOTES}"
fi

# Save release notes to temporary file
TEMP_NOTES=$(mktemp)
echo "$RELEASE_NOTES" > "$TEMP_NOTES"

# Show final release notes
echo ""
print_color $BLUE "Final release notes:"
cat "$TEMP_NOTES"
echo ""

# Confirm and create tag
print_color $YELLOW "Ready to create tag ${NEW_VERSION}"
read -p "Proceed with creating and pushing tag? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    rm "$TEMP_NOTES"
    print_color $RED "Aborted"
    exit 1
fi

# Create and push tag
print_color $GREEN "Creating tag ${NEW_VERSION}..."
git tag -a "$NEW_VERSION" -F "$TEMP_NOTES"

print_color $GREEN "Pushing tag to origin..."
git push origin "$NEW_VERSION"

# Clean up
rm "$TEMP_NOTES"

# Ask about GitHub release
echo ""
print_color $BLUE "Tag created and pushed successfully!"
print_color $YELLOW "GitHub Actions will automatically create a release from this tag."
echo ""
print_color $GREEN "You can monitor the release at:"
echo "  https://github.com/$(git remote get-url origin | sed 's/.*github.com[:\/]\(.*\)\.git/\1/')/actions"
echo ""

# Optionally trigger manual workflow
print_color $BLUE "Alternatively, you can trigger a manual release workflow."
read -p "Would you like instructions for manual release? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    print_color $BLUE "To trigger manual release:"
    echo "1. Go to: https://github.com/$(git remote get-url origin | sed 's/.*github.com[:\/]\(.*\)\.git/\1/')/actions/workflows/release.yml"
    echo "2. Click 'Run workflow'"
    echo "3. Enter version: ${NEW_VERSION}"
    echo "4. Click 'Run workflow' button"
fi

print_color $GREEN ""
print_color $GREEN "Release ${NEW_VERSION} preparation complete! 🎉"