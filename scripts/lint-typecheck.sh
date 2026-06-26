#!/bin/bash
# Resolve paths relative to this script's location.
SCRIPT_DIR="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
REPO_DIR="$(realpath "$SCRIPT_DIR/..")"

cd "$REPO_DIR/astro" && npx tsc --noEmit > /tmp/tsc-out.txt 2>&1
echo "tsc exit=$?" >> /tmp/tsc-out.txt
cd "$REPO_DIR" && go vet ./server/api/ ./server/store/ > /tmp/govet-out.txt 2>&1
echo "vet exit=$?" >> /tmp/govet-out.txt
