package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// newTestStore opens an in-memory SQLite store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenDBCreatesSchema(t *testing.T) {
	store := newTestStore(t)

	db := store.DB()
	if db == nil {
		t.Fatal("store.DB() returned nil")
	}

	// Verify tables.
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	expectedTables := []string{"log_entries", "run_services", "runs"}
	if len(tables) != len(expectedTables) {
		t.Fatalf("expected %d tables, got %d: %v", len(expectedTables), len(tables), tables)
	}
	for i, want := range expectedTables {
		if tables[i] != want {
			t.Errorf("table[%d]: want %q, got %q", i, want, tables[i])
		}
	}

	// Verify indexes.
	idxRows, err := db.Query("SELECT name FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	defer idxRows.Close()

	var indexes []string
	for idxRows.Next() {
		var name string
		if err := idxRows.Scan(&name); err != nil {
			t.Fatalf("scan index name: %v", err)
		}
		indexes = append(indexes, name)
	}

	expectedIndexes := []string{"idx_log_entries_level", "idx_log_entries_run_service"}
	if len(indexes) != len(expectedIndexes) {
		t.Fatalf("expected %d indexes, got %d: %v", len(expectedIndexes), len(indexes), indexes)
	}
	for i, want := range expectedIndexes {
		if indexes[i] != want {
			t.Errorf("index[%d]: want %q, got %q", i, want, indexes[i])
		}
	}
}

func TestOpenFileBackedDBSchemaPersists(t *testing.T) {
	f, err := os.CreateTemp("", "wywy-ci-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	// First open applies schema.
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Re-open — schema must persist.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()

	db := s2.DB()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 tables after re-open, got %d", count)
	}

	// Verify a specific table is usable.
	if _, err := db.Exec("INSERT INTO runs (id, created_at, status) VALUES ('r1', datetime('now'), 'pending')"); err != nil {
		t.Fatalf("insert into runs: %v", err)
	}
}

func TestCreateAndGetRun(t *testing.T) {
	s := newTestStore(t)

	run := &Run{
		ID:        "r1",
		CreatedAt: "2026-06-13T10:00:00Z",
		Status:    "pending",
	}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun("r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got == nil {
		t.Fatal("GetRun returned nil")
	}

	if got.ID != run.ID {
		t.Errorf("ID: want %q, got %q", run.ID, got.ID)
	}
	if got.CreatedAt != run.CreatedAt {
		t.Errorf("CreatedAt: want %q, got %q", run.CreatedAt, got.CreatedAt)
	}
	if got.Status != run.Status {
		t.Errorf("Status: want %q, got %q", run.Status, got.Status)
	}
}

func TestListRunsSorted(t *testing.T) {
	s := newTestStore(t)

	// Create 3 runs with distinct timestamps — T3 newest.
	runs := []*Run{
		{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "passed"},
		{ID: "r2", CreatedAt: "2026-06-13T11:00:00Z", Status: "failed"},
		{ID: "r3", CreatedAt: "2026-06-13T12:00:00Z", Status: "running"},
	}
	for _, r := range runs {
		if err := s.CreateRun(r); err != nil {
			t.Fatalf("CreateRun(%s): %v", r.ID, err)
		}
	}

	results, err := s.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(results))
	}

	// Newest first: r3, r2, r1.
	expected := []string{"r3", "r2", "r1"}
	for i, want := range expected {
		if results[i].ID != want {
			t.Errorf("position %d: want %q, got %q", i, want, results[i].ID)
		}
	}
}

func TestUpdateRunStatus(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateRun(&Run{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "pending"}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	if err := s.UpdateRunStatus("r1", "passed", "2026-06-13T10:05:00Z"); err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}

	got, err := s.GetRun("r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	if got.Status != "passed" {
		t.Errorf("Status: want %q, got %q", "passed", got.Status)
	}
	if got.FinishedAt != "2026-06-13T10:05:00Z" {
		t.Errorf("FinishedAt: want %q, got %q", "2026-06-13T10:05:00Z", got.FinishedAt)
	}
}

func TestCreateAndGetRunService(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateRun(&Run{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "running"}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	rs := &RunService{
		RunID:       "r1",
		ServiceName: "agentic",
		Suite:       "test",
		Status:      "pending",
	}
	if err := s.CreateRunService(rs); err != nil {
		t.Fatalf("CreateRunService: %v", err)
	}

	got, err := s.GetRunService("r1", "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}
	if got == nil {
		t.Fatal("GetRunService returned nil")
	}

	if got.RunID != "r1" {
		t.Errorf("RunID: want %q, got %q", "r1", got.RunID)
	}
	if got.ServiceName != "agentic" {
		t.Errorf("ServiceName: want %q, got %q", "agentic", got.ServiceName)
	}
	if got.Suite != "test" {
		t.Errorf("Suite: want %q, got %q", "test", got.Suite)
	}
	if got.Status != "pending" {
		t.Errorf("Status: want %q, got %q", "pending", got.Status)
	}
}

func TestUpdateRunServiceExitCode(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateRun(&Run{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "running"}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := s.CreateRunService(&RunService{
		RunID: "r1", ServiceName: "agentic", Suite: "test", Status: "pending",
	}); err != nil {
		t.Fatalf("CreateRunService: %v", err)
	}

	if err := s.UpdateRunService("r1", "agentic", 1, "failed", "2026-06-13T10:05:00Z"); err != nil {
		t.Fatalf("UpdateRunService: %v", err)
	}

	got, err := s.GetRunService("r1", "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}

	if got.Status != "failed" {
		t.Errorf("Status: want %q, got %q", "failed", got.Status)
	}
	if got.ExitCode == nil || *got.ExitCode != 1 {
		t.Errorf("ExitCode: want 1, got %v", got.ExitCode)
	}
	if got.EndTime != "2026-06-13T10:05:00Z" {
		t.Errorf("EndTime: want %q, got %q", "2026-06-13T10:05:00Z", got.EndTime)
	}
}

func TestInsertLogEntries(t *testing.T) {
	s := newTestStore(t)

	entries := make([]LogEntry, 50)
	for i := range entries {
		entries[i] = LogEntry{
			RunID:       "r1",
			ServiceName: "agentic",
			LineNumber:  i + 1,
			Timestamp:   "2026-06-13T10:00:00Z",
			Level:       "INFO",
			Content:     "test output line",
		}
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	var count int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM log_entries WHERE run_id = 'r1' AND service_name = 'agentic'`,
	).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 50 {
		t.Errorf("expected 50 log entries, got %d", count)
	}
}

func TestQueryLogEntriesBasic(t *testing.T) {
	s := newTestStore(t)

	entries := make([]LogEntry, 0, 15)
	for i := 1; i <= 10; i++ {
		entries = append(entries, LogEntry{
			RunID: "r1", ServiceName: "agentic", LineNumber: i,
			Timestamp: "2026-06-13T10:00:00Z", Level: "INFO", Content: "agentic log",
		})
	}
	for i := 1; i <= 5; i++ {
		entries = append(entries, LogEntry{
			RunID: "r1", ServiceName: "cache", LineNumber: i,
			Timestamp: "2026-06-13T10:00:00Z", Level: "INFO", Content: "cache log",
		})
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	results, err := s.QueryLogEntries("r1", "agentic", LogQueryOpts{})
	if err != nil {
		t.Fatalf("QueryLogEntries: %v", err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 entries, got %d", len(results))
	}
	for i, e := range results {
		if e.ServiceName != "agentic" {
			t.Errorf("entry %d: ServiceName want %q, got %q", i, "agentic", e.ServiceName)
		}
		if e.LineNumber != i+1 {
			t.Errorf("entry %d: LineNumber want %d, got %d", i, i+1, e.LineNumber)
		}
	}
}

func TestQueryLogEntriesByLevel(t *testing.T) {
	s := newTestStore(t)

	levels := []string{"ERROR", "ERROR", "ERROR", "WARN", "WARN", "WARN", "WARN", "WARN", "INFO", "INFO"}
	entries := make([]LogEntry, len(levels))
	for i, lvl := range levels {
		entries[i] = LogEntry{
			RunID: "r1", ServiceName: "agentic", LineNumber: i + 1,
			Timestamp: "2026-06-13T10:00:00Z", Level: lvl, Content: "log",
		}
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	results, err := s.QueryLogEntries("r1", "agentic", LogQueryOpts{Level: "ERROR"})
	if err != nil {
		t.Fatalf("QueryLogEntries: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 ERROR entries, got %d", len(results))
	}
	for i, e := range results {
		if e.Level != "ERROR" {
			t.Errorf("entry %d: Level want %q, got %q", i, "ERROR", e.Level)
		}
	}
}

func TestQueryLogEntriesBySearch(t *testing.T) {
	s := newTestStore(t)

	contents := []string{"timeout error", "connection refused", "all good"}
	entries := make([]LogEntry, len(contents))
	for i, c := range contents {
		entries[i] = LogEntry{
			RunID: "r1", ServiceName: "agentic", LineNumber: i + 1,
			Timestamp: "2026-06-13T10:00:00Z", Level: "INFO", Content: c,
		}
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	results, err := s.QueryLogEntries("r1", "agentic", LogQueryOpts{Search: "timeout"})
	if err != nil {
		t.Fatalf("QueryLogEntries: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 entry matching 'timeout', got %d", len(results))
	}
	if results[0].Content != "timeout error" {
		t.Errorf("Content: want %q, got %q", "timeout error", results[0].Content)
	}
}

func TestQueryLogEntriesPaginate(t *testing.T) {
	s := newTestStore(t)

	entries := make([]LogEntry, 100)
	for i := range entries {
		entries[i] = LogEntry{
			RunID: "r1", ServiceName: "agentic", LineNumber: i + 1,
			Timestamp: "2026-06-13T10:00:00Z", Level: "INFO", Content: "log",
		}
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	results, err := s.QueryLogEntries("r1", "agentic", LogQueryOpts{Offset: 50, Limit: 20})
	if err != nil {
		t.Fatalf("QueryLogEntries: %v", err)
	}

	if len(results) != 20 {
		t.Fatalf("expected 20 entries, got %d", len(results))
	}
	if results[0].LineNumber != 51 {
		t.Errorf("first entry LineNumber: want 51, got %d", results[0].LineNumber)
	}
	if results[19].LineNumber != 70 {
		t.Errorf("last entry LineNumber: want 70, got %d", results[19].LineNumber)
	}
}

// TestOpenCreatesParentDirectories verifies that Open creates the parent
// directories of the database path when they don't exist.
// Without this, `run.sh ci dev` silently falls back to an in-memory database
// because /var/lib/Wywy-Website/ci/ doesn't exist.
func TestOpenCreatesParentDirectories(t *testing.T) {
	// Use a path where the parent directory does not exist.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent", "subdir", "ci.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open should create parent directories, got error: %v", err)
	}
	defer s.Close()

	// Verify the database file was actually created.
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file not created at %s: %v", dbPath, err)
	}

	// Verify the schema was applied (tables exist).
	db := s.DB()
	var tableCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tableCount != 3 {
		t.Errorf("expected 3 tables, got %d", tableCount)
	}
}

// TestRunServiceSchemaHasCountColumns verifies that the run_services table
// has passed, failed, and skipped columns. This test will fail until the
// migration in sqlite.go adds these columns.
func TestRunServiceSchemaHasCountColumns(t *testing.T) {
	s := newTestStore(t)

	rows, err := s.DB().Query("PRAGMA table_info(run_services)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	for _, col := range []string{"passed", "failed", "skipped"} {
		if !columns[col] {
			t.Errorf("run_services table is missing column %q — add it to the migration in sqlite.go", col)
		}
	}
}

// TestCreateAndGetRunServiceWithCounts verifies that RunService count fields
// (passed, failed, skipped) survive a round-trip through SQLite. This test
// will fail until the run_services table has count columns and the
// Store methods INSERT/SELECT them.
func TestCreateAndGetRunServiceWithCounts(t *testing.T) {
	s := newTestStore(t)

	// Create the parent run first.
	if err := s.CreateRun(&Run{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "running"}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Attempt to INSERT with count columns via raw SQL.
	_, err := s.DB().Exec(`
		INSERT INTO run_services (run_id, service_name, suite, status, passed, failed, skipped)
		VALUES ('r1', 'agentic', 'test', 'passed', 5, 2, 1)
	`)
	if err != nil {
		t.Fatalf("expected INSERT with count columns to succeed, but got: %v", err)
	}

	// Verify the values are stored correctly.
	var passed, failed, skipped int
	err = s.DB().QueryRow(
		`SELECT passed, failed, skipped FROM run_services WHERE run_id = 'r1' AND service_name = 'agentic'`,
	).Scan(&passed, &failed, &skipped)
	if err != nil {
		t.Fatalf("expected SELECT of count columns to succeed, but got: %v", err)
	}

	if passed != 5 {
		t.Errorf("passed: want 5, got %d", passed)
	}
	if failed != 2 {
		t.Errorf("failed: want 2, got %d", failed)
	}
	if skipped != 1 {
		t.Errorf("skipped: want 1, got %d", skipped)
	}
}

// TestGetRunServiceReturnsCountFields verifies that GetRunService returns
// count fields (passed, failed, skipped) by inserting enriched data via raw
// SQL, calling GetRunService, and checking JSON output contains the values.
func TestGetRunServiceReturnsCountFields(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateRun(&Run{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "running"}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Insert enriched row via raw SQL (count columns exist from A-I).
	_, err := s.DB().Exec(`
		INSERT INTO run_services (run_id, service_name, suite, status, passed, failed, skipped)
		VALUES ('r1', 'agentic', 'test', 'passed', 5, 2, 1)
	`)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	// GetRunService should return count fields, but currently doesn't.
	rs, err := s.GetRunService("r1", "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}

	// Marshal to JSON and verify count fields appear in the output.
	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := string(data)

	for _, want := range []string{`"passed":5`, `"failed":2`, `"skipped":1`} {
		if !strings.Contains(got, want) {
			t.Errorf("GetRunService JSON should contain %s; got: %s", want, got)
		}
	}
}

// TestListRunServicesReturnsCountFields verifies that ListRunServices returns
// count fields (passed, failed, skipped). Inserts enriched data via raw SQL,
// then checks JSON output.
func TestListRunServicesReturnsCountFields(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateRun(&Run{ID: "r1", CreatedAt: "2026-06-13T10:00:00Z", Status: "running"}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Insert enriched row via raw SQL.
	_, err := s.DB().Exec(`
		INSERT INTO run_services (run_id, service_name, suite, status, passed, failed, skipped)
		VALUES ('r1', 'agentic', 'test', 'passed', 5, 2, 1)
	`)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	svcs, err := s.ListRunServices("r1")
	if err != nil {
		t.Fatalf("ListRunServices: %v", err)
	}
	if len(svcs) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcs))
	}

	// Marshal to JSON and verify count fields appear in the output.
	data, err := json.Marshal(svcs[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := string(data)

	for _, want := range []string{`"passed":5`, `"failed":2`, `"skipped":1`} {
		if !strings.Contains(got, want) {
			t.Errorf("ListRunServices JSON should contain %s; got: %s", want, got)
		}
	}
}
