#!/usr/bin/env bash
# Cross-compile the leaf binary for every supported target.
# Output: bin/leaf-<os>-<arch>[.exe]
#
# Skeleton. Real implementation will:
#   1. Ensure internal/embed/phar/leaf.phar exists (build-phar.sh first)
#   2. For each target in (darwin-arm64 darwin-amd64 linux-arm64 linux-amd64 windows-amd64):
#        - Set GOOS/GOARCH
#        - Build via FrankenPHP's static build (needs libphp cross-compile toolchain)
#        - Output into bin/
#   3. Generate checksums

set -euo pipefail

echo "build-matrix.sh: not yet implemented" >&2
exit 1
