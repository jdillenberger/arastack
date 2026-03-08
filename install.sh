#!/usr/bin/env bash
set -euo pipefail

REPO="jdillenberger/arastack"
INSTALL_DIR="/usr/local/bin"

# Ensure Linux
if [ "$(uname -s)" != "Linux" ]; then
  echo "Error: arastack only supports Linux." >&2
  exit 1
fi

# Detect architecture
case "$(uname -m)" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  armv7l)  ARCH="armv7" ;;
  *)
    echo "Error: unsupported architecture $(uname -m)" >&2
    exit 1
    ;;
esac

echo "Detected architecture: ${ARCH}"

# Fetch latest release tag
echo "Fetching latest release..."
RELEASE_JSON=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")
TAG=$(echo "${RELEASE_JSON}" | grep -m1 '"tag_name"' | cut -d'"' -f4)

if [ -z "${TAG}" ]; then
  echo "Error: could not determine latest release." >&2
  exit 1
fi

echo "Latest release: ${TAG}"

# Find the aramanager asset URL
ASSET_PATTERN="aramanager_linux_${ARCH}"
DOWNLOAD_URL=$(echo "${RELEASE_JSON}" | grep -o "\"browser_download_url\": *\"[^\"]*${ASSET_PATTERN}[^\"]*\\.tar\\.gz\"" | head -1 | cut -d'"' -f4)

if [ -z "${DOWNLOAD_URL}" ]; then
  echo "Error: no aramanager release asset found for linux/${ARCH}." >&2
  exit 1
fi

# Download and extract
echo "Downloading aramanager..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSL "${DOWNLOAD_URL}" | tar -xz -C "${TMP_DIR}"

# Install
install -m 755 "${TMP_DIR}/aramanager" "${INSTALL_DIR}/aramanager"

echo ""
echo "aramanager ${TAG} installed to ${INSTALL_DIR}/aramanager"
echo ""
echo "Next step:"
echo "  sudo aramanager setup"
