#!/bin/bash
# Wywy-CI test runner script.
# Runs Go unit tests, Astro component tests, and Playwright E2E tests.
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

    # Run with -json reporter and parse counts via Python.
    local go_output_file
    go_output_file="$(mktemp)"
    local go_exit=0
    (cd "$REPO_DIR" && go test -json -race -count=1 ./...) 2>&1 | tee "$go_output_file" || go_exit=$?

    local go_jsonl
    go_jsonl="$(python3 -c '
import json, sys

counts = {}
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    pkg = event.get("Package", "")
    action = event.get("Action", "")
    test = event.get("Test", "")
    if pkg and test and action in ("pass", "fail", "skip"):
        if pkg not in counts:
            counts[pkg] = {"passed": 0, "failed": 0, "skipped": 0}
        if action == "skip":
            counts[pkg]["skipped"] += 1
        elif action == "pass":
            counts[pkg]["passed"] += 1
        elif action == "fail":
            counts[pkg]["failed"] += 1

total_passed = sum(c["passed"] for c in counts.values())
total_failed = sum(c["failed"] for c in counts.values())
total_skipped = sum(c["skipped"] for c in counts.values())
total_tests = total_passed + total_failed + total_skipped
overall_failed = any(c["failed"] > 0 for c in counts.values())
status = "failed" if overall_failed else "passed"
print(json.dumps({
    "name": "go-tests",
    "status": status,
    "passed": total_passed,
    "failed": total_failed,
    "skipped": total_skipped,
    "total": total_tests,
}))
' < "$go_output_file")"
    [ -n "$output_dir" ] && echo "$go_jsonl" >> "$output_dir/results.jsonl"

    rm -f "$go_output_file"

    if [ "$go_exit" -ne 0 ]; then
        print_fail "go test"
        return 1
    fi
    print_pass "go test"
}

function run_astro_tests() {
    print_info "Running Astro tests..."

    local astro_output_file
    astro_output_file="$(mktemp)"
    local astro_exit=0
    (cd "$REPO_DIR/astro" && npx vitest run --reporter=json) > "$astro_output_file" 2>&1 || astro_exit=$?

    local astro_jsonl
    astro_jsonl="$(python3 -c '
import json, sys

data = json.load(sys.stdin)
result = data.get("result", data)
passed = result.get("numPassedTests", 0)
failed = result.get("numFailedTests", 0)
skipped = result.get("numPendingTests", 0) + result.get("numTodoTests", 0)
total = result.get("numTotalTests", 0)
if total == 0:
    total = passed + failed + skipped
status = "failed" if failed > 0 or not result.get("success", True) else "passed"
print(json.dumps({
    "name": "astro-tests",
    "status": status,
    "passed": passed,
    "failed": failed,
    "skipped": skipped,
    "total": total,
}))
' < "$astro_output_file")"
    [ -n "$output_dir" ] && echo "$astro_jsonl" >> "$output_dir/results.jsonl"

    rm -f "$astro_output_file"

    if [ "$astro_exit" -ne 0 ]; then
        print_fail "Astro tests"
        return 1
    fi
    print_pass "Astro tests"
}

function run_playwright_e2e() {
    print_info "Running Playwright E2E tests..."

    local pw_output_file
    pw_output_file="$(mktemp)"
    local pw_exit=0
    (cd "$REPO_DIR/astro" && npx playwright test --reporter=json) > "$pw_output_file" 2>&1 || pw_exit=$?

    local pw_jsonl
    pw_jsonl="$(python3 -c '
import json, sys

data = json.load(sys.stdin)
stats = data.get("stats", data)
expected = stats.get("expected", 0)
unexpected = stats.get("unexpected", 0)
skipped = stats.get("skipped", 0)
flaky = stats.get("flaky", 0)
total = expected + unexpected + skipped + flaky
passed = expected + flaky
failed = unexpected
status = "failed" if failed > 0 else "passed"
print(json.dumps({
    "name": "playwright-e2e",
    "status": status,
    "passed": passed,
    "failed": failed,
    "skipped": skipped,
    "total": total,
}))
' < "$pw_output_file")"
    [ -n "$output_dir" ] && echo "$pw_jsonl" >> "$output_dir/results.jsonl"

    rm -f "$pw_output_file"

    if [ "$pw_exit" -ne 0 ]; then
        print_fail "Playwright E2E tests"
        return 1
    fi
    print_pass "Playwright E2E tests"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
exit_code=0

# Initialize results.jsonl (truncate from previous runs) before any suite writes.
if [ -n "$output_dir" ]; then
    mkdir -p "$output_dir"
    > "$output_dir/results.jsonl"
fi

run_go_tests || exit_code=1
run_astro_tests || exit_code=1
run_playwright_e2e || exit_code=1

if [[ "$exit_code" -eq 0 ]]; then
    echo ""
    print_info "All tests passed."
else
    echo ""
    print_fail "Some tests failed."
fi

exit "$exit_code"
