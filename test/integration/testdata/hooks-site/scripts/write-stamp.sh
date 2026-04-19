#!/usr/bin/env bash
# Second hook: receives an argument and writes it out to prove argv form works.
set -eu
if [ $# -lt 1 ]; then
  echo "usage: write-stamp.sh <label>" >&2
  exit 2
fi
echo "SECOND_HOOK_ARG=$1" > hook-stamp.txt
