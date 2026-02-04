#!/bin/bash
set -euo pipefail

# Release script for dottie
# Usage: ./scripts/release.sh [major|minor|patch]

usage() {
    echo "Usage: $0 [major|minor|patch]"
    echo "  Bumps the version, creates a git tag, and pushes it."
    exit 1
}

if [[ $# -ne 1 ]]; then
    usage
fi

BUMP_TYPE="$1"

if [[ "$BUMP_TYPE" != "major" && "$BUMP_TYPE" != "minor" && "$BUMP_TYPE" != "patch" ]]; then
    echo "Error: Invalid bump type '$BUMP_TYPE'"
    usage
fi

# Get the latest version tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Current version: $LATEST_TAG"

# Strip the 'v' prefix and split into parts
VERSION="${LATEST_TAG#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

# Bump the appropriate part
case "$BUMP_TYPE" in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch)
        PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
echo "New version: $NEW_VERSION"

# Check for uncommitted changes
if [[ -n $(git status --porcelain) ]]; then
    echo "Error: Working directory has uncommitted changes"
    exit 1
fi

# Check we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "main" ]]; then
    echo "Warning: Not on main branch (currently on '$CURRENT_BRANCH')"
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Create and push the tag
echo "Creating tag $NEW_VERSION..."
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

echo "Pushing tag to origin..."
git push origin "$NEW_VERSION"

echo "Release $NEW_VERSION tagged and pushed."
echo "GitHub Actions or goreleaser will handle the release build."
