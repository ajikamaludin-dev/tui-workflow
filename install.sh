#!/usr/bin/env bash
# install.sh — Install/upgrade tui-workflow (binary: workflow) from GitHub Releases.
# Usage:
#   wget -qO- https://raw.githubusercontent.com/ajikamaludin-dev/tui-workflow/main/install.sh | sh
#   ./install.sh            # latest release
#   ./install.sh v0.3.0     # specific tag
set -euo pipefail

REPO="ajikamaludin-dev/tui-workflow"
INSTALL_DIR="${HOME}/.local/bin"
BIN_NAME="workflow"

# --- Detect architecture ---
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture '${ARCH_RAW}'. Only linux/amd64 and linux/arm64 are published." >&2
    exit 1
    ;;
esac

if [ "$(uname -s)" != "Linux" ]; then
  echo "Error: this installer supports Linux only." >&2
  exit 1
fi

# --- Resolve tag ---
TAG="${1:-}"
if [ -z "${TAG}" ]; then
  if command -v wget >/dev/null 2>&1; then
    TAG="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep -oP '"tag_name":\s*"\K[^"]+' || true)"
  elif command -v curl >/dev/null 2>&1; then
    TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -oP '"tag_name":\s*"\K[^"]+' || true)"
  fi
  if [ -z "${TAG}" ]; then
    echo "Error: could not resolve the latest release tag from GitHub. Pass a tag explicitly: ./install.sh v0.3.0" >&2
    exit 1
  fi
fi

if ! printf '%s' "${TAG}" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Error: tag '${TAG}' does not match vX.Y.Z." >&2
  exit 1
fi

VER="${TAG#v}"
ASSET="workflow_${VER}_linux_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT

# --- Download ---
echo "Downloading ${ASSET} (${TAG})..."
if command -v wget >/dev/null 2>&1; then
  wget -q "${BASE_URL}/${ASSET}" -O "${TMPDIR}/${ASSET}"
  wget -q "${BASE_URL}/checksums.txt" -O "${TMPDIR}/checksums.txt"
else
  curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMPDIR}/${ASSET}"
  curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMPDIR}/checksums.txt"
fi

# --- Verify SHA-256 ---
echo "Verifying SHA-256..."
( cd "${TMPDIR}" && grep " ${ASSET}\$" checksums.txt | sha256sum -c - )

# --- Install ---
mkdir -p "${INSTALL_DIR}"
tar -xzf "${TMPDIR}/${ASSET}" -C "${TMPDIR}"
install -m 0755 "${TMPDIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"

echo "Installed: ${INSTALL_DIR}/${BIN_NAME}"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Note: add ${INSTALL_DIR} to your PATH: export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac

"${INSTALL_DIR}/${BIN_NAME}" --version || true
