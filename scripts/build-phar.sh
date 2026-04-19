#!/usr/bin/env bash
# Build the embedded leaf.phar from a pinned zephyrus-leaf-core tag.
# Output: internal/embed/phar/leaf.phar
#
# Skeleton. Real implementation will:
#   1. Clone zephyrus-leaf-core at $LEAF_CORE_TAG into a tempdir
#   2. composer install --no-dev --optimize-autoloader
#   3. Box the result with humbug/box into leaf.phar
#   4. Copy to internal/embed/phar/

set -euo pipefail

echo "build-phar.sh: not yet implemented" >&2
exit 1
