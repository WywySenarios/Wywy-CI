#!/bin/bash
# tsc typecheck with timeout
# Resolve paths relative to this script's location.
SCRIPT_DIR="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
REPO_DIR="$(realpath "$SCRIPT_DIR/..")"

cd "$REPO_DIR/astro" && node ./node_modules/.bin/tsc --noEmit > /tmp/tsc-out.txt 2>&1
STATUS=$?
echo "tsc exit=$STATUS" >> /tmp/tsc-out.txt
exit $STATUS
