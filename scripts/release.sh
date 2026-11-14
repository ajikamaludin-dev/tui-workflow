#!/usr/bin/env bash
# release.sh — One-command release: bump version, commit, tag, push.
# The push of the tag triggers .github/workflows/release.yml, which builds
# the Linux binaries and publishes the GitHub Release.
#
# Usage: ./scripts/release.sh vX.Y.Z
#   - run from branch `main` with a clean working tree
#   - the matching revision-history row must exist in PRD.md first:
#       | vX.Y.Z | YYYY-MM-DD | ... |
set -euo pipefail

TAG="${1:-}"

if ! printf '%s' "${TAG}" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Usage: $(basename "$0") vX.Y.Z" >&2
  exit 1
fi

VER="${TAG#v}"

# --- Preflight checks ---
BRANCH="$(git branch --show-current)"
if [ "${BRANCH}" != "main" ]; then
  echo "Error: releases must be cut from branch 'main' (current: '${BRANCH}')." >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "Error: working tree is not clean. Commit or stash changes first." >&2
  exit 1
fi

if git rev-parse "${TAG}" >/dev/null 2>&1 || [ -n "$(git ls-remote --tags origin "refs/tags/${TAG}")" ]; then
  echo "Error: tag '${TAG}' already exists." >&2
  exit 1
fi

if ! grep -q "| ${TAG} |" PRD.md; then
  echo "Error: PRD.md has no revision-history row for ${TAG}." >&2
  echo "Add one first, e.g.: | ${TAG} | $(date +%F) | ... |" >&2
  exit 1
fi

# --- Verify ---
echo "Running go vet..."
go vet ./...
echo "Running go test -race..."
go test -race ./...

# --- Bump version files ---
sed -i -E "s/^(var Version = )\"[^\"]+\"/\1\"${VER}\"/" internal/buildinfo/version.go
sed -i -E "s/^\*\*Version:\*\* v[0-9.]+/**Version:** ${TAG}/" PRD.md

git add internal/buildinfo/version.go PRD.md
git commit -m "chore(release): ${TAG}"
git tag -a "${TAG}" -m "Release ${TAG}"
git push origin main --follow-tags

echo ""
echo "Tag ${TAG} pushed. Release workflow is running:"
echo "  https://github.com/ajikamaludin-dev/tui-workflow/releases/tag/${TAG}"
