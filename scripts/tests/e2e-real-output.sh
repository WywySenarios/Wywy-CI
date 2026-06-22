#!/bin/bash
# e2e-real-output.sh — Test runner script for E2E real-output test.
#
# Produces structured JSON log lines that are parsed by the CI log parser
# and stored in the database. This exercises the real script execution path
# of the CI runner to confirm non-placeholder output is captured correctly.
#
# Usage: bash scripts/tests/e2e-real-output.sh

echo '{"msg":"[e2e] Build started","level":"INFO","service":"ci"}'
echo '{"msg":"[e2e] Running unit tests...","level":"INFO","service":"ci"}'
echo '{"msg":"[e2e] 42 passed, 0 failed","level":"INFO","service":"ci"}'
echo '{"msg":"[e2e] Build completed","level":"INFO","service":"ci"}'
