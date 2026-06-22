package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Run represents a single test run.
type Run struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	Status     string `json:"status"`
}

// CreateRun inserts a new run into the database.
func (s *Store) CreateRun(run *Run) error {
	_, err := s.db.Exec(
		`INSERT INTO runs (id, created_at, finished_at, status) VALUES (?, ?, ?, ?)`,
		run.ID, run.CreatedAt, nilIfEmpty(run.FinishedAt), run.Status,
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

// GetRun retrieves a run by its ID. Returns the error if not found.
func (s *Store) GetRun(id string) (*Run, error) {
	row := s.db.QueryRow(
		`SELECT id, created_at, finished_at, status FROM runs WHERE id = ?`, id,
	)
	r, err := scanRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("run %q not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get run: %w", err)
	}
	return r, nil
}

// ListRuns returns all runs ordered by created_at descending.
func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(`SELECT id, created_at, finished_at, status FROM runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		runs = append(runs, *r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return runs, nil
}

// scanner abstracts sql.Row and sql.Rows for shared scan logic.
type scanner interface {
	Scan(dest ...any) error
}

// scanRun scans a single Run from a row or rows scanner.
func scanRun(s scanner) (*Run, error) {
	var r Run
	var finishedAt sql.NullString
	if err := s.Scan(&r.ID, &r.CreatedAt, &finishedAt, &r.Status); err != nil {
		return nil, err
	}
	r.FinishedAt = finishedAt.String
	return &r, nil
}

// RunService represents a single service's test execution within a run.
type RunService struct {
	RunID       string `json:"run_id"`
	ServiceName string `json:"service_name"`
	Suite       string `json:"suite"`
	Status      string `json:"status"`
	ExitCode    *int   `json:"exit_code"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
}

// CreateRunService inserts a new run_service record.
func (s *Store) CreateRunService(rs *RunService) error {
	_, err := s.db.Exec(
		`INSERT INTO run_services (run_id, service_name, suite, status, exit_code, start_time, end_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rs.RunID, rs.ServiceName, rs.Suite, rs.Status,
		nilIfNil(rs.ExitCode),
		nilIfEmpty(rs.StartTime), nilIfEmpty(rs.EndTime),
	)
	if err != nil {
		return fmt.Errorf("insert run_service: %w", err)
	}
	return nil
}

// GetRunService retrieves a run_service by run_id and service_name.
func (s *Store) GetRunService(runID, serviceName string) (*RunService, error) {
	row := s.db.QueryRow(
		`SELECT run_id, service_name, suite, status, exit_code, start_time, end_time
		 FROM run_services WHERE run_id = ? AND service_name = ?`,
		runID, serviceName,
	)
	rs, err := scanRunService(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("run_service %q/%q not found: %w", runID, serviceName, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get run_service: %w", err)
	}
	return rs, nil
}

// ListActiveRunServices returns distinct service names from runs with status "running".
func (s *Store) ListActiveRunServices() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT rs.service_name
		 FROM runs r
		 JOIN run_services rs ON r.id = rs.run_id
		 WHERE r.status = 'running'`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active run_services: %w", err)
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var svc string
		if err := rows.Scan(&svc); err != nil {
			return nil, fmt.Errorf("scan active service: %w", err)
		}
		services = append(services, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return services, nil
}

// ListRunServices returns all run_service records for a given run.
func (s *Store) ListRunServices(runID string) ([]RunService, error) {
	rows, err := s.db.Query(
		`SELECT run_id, service_name, suite, status, exit_code, start_time, end_time
		 FROM run_services WHERE run_id = ?`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list run_services: %w", err)
	}
	defer rows.Close()

	var svcs []RunService
	for rows.Next() {
		rs, err := scanRunService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan run_service: %w", err)
		}
		svcs = append(svcs, *rs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return svcs, nil
}

// UpdateRunStatus updates the status and finished_at of a run.
func (s *Store) UpdateRunStatus(id, status, finishedAt string) error {
	_, err := s.db.Exec(
		`UPDATE runs SET status = ?, finished_at = ? WHERE id = ?`,
		status, nilIfEmpty(finishedAt), id,
	)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	return nil
}

// LogEntry represents a single line of test output.
type LogEntry struct {
	RunID       string `json:"run_id"`
	ServiceName string `json:"service_name"`
	LineNumber  int    `json:"line_number"`
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	Content     string `json:"content"`
}

// InsertLogEntries inserts multiple log entries in a single transaction.
func (s *Store) InsertLogEntries(entries []LogEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO log_entries (run_id, service_name, line_number, timestamp, level, content)
		 VALUES (?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.RunID, e.ServiceName, e.LineNumber, e.Timestamp, e.Level, e.Content); err != nil {
			return fmt.Errorf("insert log entry: %w", err)
		}
	}
	return tx.Commit()
}

// LogQueryOpts filters for QueryLogEntries.
type LogQueryOpts struct {
	Level  string
	Search string
	Offset int
	Limit  int
}

// QueryLogEntries returns log entries matching the given filters.
func (s *Store) QueryLogEntries(runID, serviceName string, opts LogQueryOpts) ([]LogEntry, error) {
	query := `SELECT run_id, service_name, line_number, timestamp, level, content
		 FROM log_entries WHERE run_id = ? AND service_name = ?`
	args := []any{runID, serviceName}

	if opts.Level != "" {
		query += ` AND level = ?`
		args = append(args, opts.Level)
	}

	if opts.Search != "" {
		query += ` AND content LIKE ?`
		args = append(args, "%"+opts.Search+"%")
	}

	query += ` ORDER BY line_number`

	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, opts.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query log entries: %w", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.RunID, &e.ServiceName, &e.LineNumber, &e.Timestamp, &e.Level, &e.Content); err != nil {
			return nil, fmt.Errorf("scan log entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return entries, nil
}

// QueryAllLogEntries returns log entries for a run across all services.
func (s *Store) QueryAllLogEntries(runID string, opts LogQueryOpts) ([]LogEntry, error) {
	query := `SELECT run_id, service_name, line_number, timestamp, level, content
		 FROM log_entries WHERE run_id = ?`
	args := []any{runID}

	if opts.Level != "" {
		query += ` AND level = ?`
		args = append(args, opts.Level)
	}

	if opts.Search != "" {
		query += ` AND content LIKE ?`
		args = append(args, "%"+opts.Search+"%")
	}

	query += ` ORDER BY line_number`

	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, opts.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query all log entries: %w", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.RunID, &e.ServiceName, &e.LineNumber, &e.Timestamp, &e.Level, &e.Content); err != nil {
			return nil, fmt.Errorf("scan log entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return entries, nil
}

// UpdateRunService updates the exit_code, status, and end_time of a run_service.
func (s *Store) UpdateRunService(runID, serviceName string, exitCode int, status, endTime string) error {
	_, err := s.db.Exec(
		`UPDATE run_services SET exit_code = ?, status = ?, end_time = ?
		 WHERE run_id = ? AND service_name = ?`,
		exitCode, status, nilIfEmpty(endTime), runID, serviceName,
	)
	if err != nil {
		return fmt.Errorf("update run_service: %w", err)
	}
	return nil
}

// scanRunService scans a single RunService from a row scanner.
func scanRunService(s scanner) (*RunService, error) {
	var rs RunService
	var exitCode sql.NullInt64
	var startTime, endTime sql.NullString
	if err := s.Scan(&rs.RunID, &rs.ServiceName, &rs.Suite, &rs.Status,
		&exitCode, &startTime, &endTime); err != nil {
		return nil, err
	}
	if exitCode.Valid {
		v := int(exitCode.Int64)
		rs.ExitCode = &v
	}
	rs.StartTime = startTime.String
	rs.EndTime = endTime.String
	return &rs, nil
}

// nilIfEmpty returns nil if s is empty, otherwise returns s.
// This converts Go empty strings to SQL NULL.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nilIfNil returns nil if p is nil, otherwise returns *p.
// This converts Go nil pointers to SQL NULL.
func nilIfNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
