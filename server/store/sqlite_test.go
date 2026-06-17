package store

import (
	"os"
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
