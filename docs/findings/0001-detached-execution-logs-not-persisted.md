# Finding 0001: Detached execution logs not persisted to database

**Date:** 2026-06-21
**Author:** Scribe Agent
**Status:** Open

## Summary

When a run executes via the detached execution path (`Runner.executeDetached`),
the script's stdout and stderr are written to disk files (`stdout.log`,
`stderr.log`) but are **never parsed and inserted into the `log_entries`
database table**. This means the API returns zero logs for any run that
completes via the detached path, regardless of whether the script produced
output.

## Evidence

- `run-1782089712877694313` exists in the database with status `"failed"` and
  exit code `1`, but `log_entries` contains **0 rows** for it.
- The on-disk output directory
  `/var/lib/Wywy-Website/ci/runs/run-1782089712877694313/ci/` is empty — no
  `stdout.log`, `stderr.log`, or `results.jsonl` were written.

## Root cause

In `server/orchestrator/runner.go:156`, the call to `MonitorScriptOutput`
discards its stdout and stderr return values:

```go
results, _, _, err := MonitorScriptOutput(ctx, outputDir, 30*time.Minute)
//       ^  ^  — stdout and stderr thrown away
```

The only code path that inserts into `log_entries` is the fallback at line 256,
which only fires when `status == ""` (i.e., the detached path was never taken).

## Impact

- Users see no logs in the API for any run.
- Debugging failed runs is impossible from the API alone.
- The on-disk output files (`stdout.log`, `stderr.log`) are the only record,
  but the runner discards them without ingestion.

## Suggested fix

Insert a step in `executeDetached` (or in `executeServices` after
`executeDetached` returns) that reads the on-disk `stdout.log` / `stderr.log`,
parses the content (using `server/log/parser.go`), and inserts entries into
the `log_entries` table via `store.InsertLogEntries`.

This is a behavioral change and requires a full TDD cycle (RED → GREEN →
REFACTOR).
