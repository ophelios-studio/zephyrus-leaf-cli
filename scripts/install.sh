#!/usr/bin/env bash
# Install the latest (or pinned) leaf CLI binary.
#
#   curl -fsSL https://leaf.ophelios.com/install.sh | sh
#
# Environment overrides:
#   LEAF_VERSION   specific release tag (default: latest)
#   LEAF_PREFIX    install prefix (default: /usr/local/bin)

set -eu

REPO="ophelios-studio/zephyrus-leaf-cli"
PREFIX="${LEAF_PREFIX:-/usr/local/bin}"
VERSION="${LEAF_VERSION:-latest}"

err() { printf "install: %s\n" "$1" >&2; exit 1; }

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$os" in
    linux)  os=linux ;;
    darwin) os=darwin ;;
    mingw*|msys*|cygwin*) os=windows ;;
    *) err "unsupported OS: $os" ;;
  esac
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) err "unsupported architecture: $arch" ;;
  esac
  printf '%s-%s' "$os" "$arch"
}

download() {
  local url=$1 out=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
  else
    err "neither curl nor wget available"
  fi
}

main() {
  local platform asset url ext=""
  platform=$(detect_platform)
  case "$platform" in windows-*) ext=".exe" ;; esac

  if [ "$VERSION" = "latest" ]; then
    url_base="https://github.com/$REPO/releases/latest/download"
  else
    url_base="https://github.com/$REPO/releases/download/$VERSION"
  fi

  asset="leaf-${platform}${ext}"
  url="$url_base/$asset"

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  printf "install: downloading %s\n" "$asset"
  download "$url" "$tmp/$asset"

  # Optional checksum verification if checksums.txt is fetchable.
  if download "$url_base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
    if command -v sha256sum >/dev/null 2>&1; then
      (cd "$tmp" && grep " $asset\$" checksums.txt | sha256sum -c -) \
        || err "checksum mismatch for $asset"
    elif command -v shasum >/dev/null 2>&1; then
      (cd "$tmp" && grep " $asset\$" checksums.txt | shasum -a 256 -c -) \
        || err "checksum mismatch for $asset"
    else
      printf "install: no sha256 tool available, skipping verification\n" >&2
    fi
  else
    printf "install: checksums.txt not published, skipping verification\n" >&2
  fi

  chmod +x "$tmp/$asset"
  mkdir -p "$PREFIX"

  if [ -w "$PREFIX" ]; then
    mv "$tmp/$asset" "$PREFIX/leaf${ext}"
  else
    printf "install: writing to %s requires sudo\n" "$PREFIX"
    sudo mv "$tmp/$asset" "$PREFIX/leaf${ext}"
  fi

  printf "install: done. Run 'leaf version' to verify.\n"
}

main "$@"
