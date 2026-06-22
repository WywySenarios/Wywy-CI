#!/bin/bash
# Wywy-CI test runner script.
# Runs Go unit tests and Astro component tests.
# Compliant with CI runner contract: parses --output-dir= and writes results.jsonl.
set -euo pipefail

# Parse --output-dir= argument (CI runner contract).
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done

# Enable CGO so that -race (race detector) can compile its C++ runtime.
export CGO_ENABLED=1

SCRIPT_DIR="$(dirname "$(realpath "$0")")"
REPO_DIR="$(realpath "$SCRIPT_DIR/../..")"

# ---------------------------------------------------------------------------
# Colors for output
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ---------------------------------------------------------------------------
# Functions
# ---------------------------------------------------------------------------
function print_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

function print_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

function print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

function run_go_tests() {
    print_info "Running Go tests..."

    if ! (cd "$REPO_DIR" && go vet ./...) then
        print_fail "go vet"
        return 1
    fi
    print_pass "go vet"

    if ! (cd "$REPO_DIR" && go test -race -count=1 ./...) then
        print_fail "go test"
        return 1
    fi
    print_pass "go test"
}

function run_astro_tests() {
    print_info "Running Astro tests..."

    if ! (cd "$REPO_DIR/astro" && npm test) then
        print_fail "Astro tests"
        return 1
    fi
    print_pass "Astro tests"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
exit_code=0

run_go_tests || exit_code=1
run_astro_tests || exit_code=1

if [[ "$exit_code" -eq 0 ]]; then
    echo ""
    print_info "All tests passed."
else
    echo ""
    print_fail "Some tests failed."
fi

# Write results.jsonl (CI runner contract).
if [ -n "$output_dir" ]; then
    if [ "$exit_code" -eq 0 ]; then
        echo '{"name":"go-tests","status":"passed"}' > "$output_dir/results.jsonl"
        echo '{"name":"astro-tests","status":"passed"}' >> "$output_dir/results.jsonl"
    else
        echo '{"name":"go-tests","status":"failed"}' > "$output_dir/results.jsonl"
        echo '{"name":"astro-tests","status":"failed"}' >> "$output_dir/results.jsonl"
    fi
fi

exit "$exit_code"
