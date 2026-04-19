#!/usr/bin/env bash
# First hook: writes a sentinel file the test can find.
set -eu
echo "FIRST_HOOK_RAN" > hook-marker.txt
