package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"loadtester/backend/internal/model"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS proxies (
  address TEXT PRIMARY KEY,
  active INTEGER NOT NULL DEFAULT 1,
  last_checked TEXT,
  last_latency INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  success_count INTEGER NOT NULL DEFAULT 0,
  failure_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS uploaded_files (
  id TEXT PRIMARY KEY,
  file_name TEXT NOT NULL,
  disk_path TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS runs (
  run_id TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  config_json TEXT NOT NULL,
  summary_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS run_samples (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  timestamp TEXT NOT NULL,
  latency_ms INTEGER NOT NULL,
  status_code INTEGER NOT NULL,
  success INTEGER NOT NULL,
  error_category TEXT NOT NULL,
  error_message TEXT NOT NULL,
  proxy_address TEXT NOT NULL,
  bytes_sent INTEGER NOT NULL,
  bytes_received INTEGER NOT NULL
);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) SaveUploadedFile(ctx context.Context, id, fileName, diskPath string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO uploaded_files (id, file_name, disk_path, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET file_name=excluded.file_name, disk_path=excluded.disk_path
`, id, fileName, diskPath, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) UploadedFilePath(ctx context.Context, id string) (string, string, error) {
	var name, path string
	err := s.db.QueryRowContext(ctx, `SELECT file_name, disk_path FROM uploaded_files WHERE id = ?`, id).Scan(&name, &path)
	return name, path, err
}

func (s *Store) UpsertProxies(ctx context.Context, addresses []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, addr := range addresses {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO proxies (address, active, last_checked, last_latency, last_error, success_count, failure_count)
VALUES (?, 1, ?, 0, '', 0, 0)
ON CONFLICT(address) DO NOTHING
`, addr, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListProxies(ctx context.Context) ([]model.ProxyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT address, active, last_checked, last_latency, last_error, success_count, failure_count FROM proxies ORDER BY address`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ProxyRecord, 0)
	for rows.Next() {
		var p model.ProxyRecord
		var active int
		var checked sql.NullString
		if err := rows.Scan(&p.Address, &active, &checked, &p.LastLatency, &p.LastError, &p.SuccessCount, &p.FailureCount); err != nil {
			return nil, err
		}
		p.Active = active == 1
		if checked.Valid {
			if t, err := time.Parse(time.RFC3339, checked.String); err == nil {
				p.LastChecked = &t
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) ListActiveProxies(ctx context.Context) ([]model.ProxyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT address, active, last_checked, last_latency, last_error, success_count, failure_count FROM proxies WHERE active = 1 ORDER BY address`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ProxyRecord, 0)
	for rows.Next() {
		var p model.ProxyRecord
		var active int
		var checked sql.NullString
		if err := rows.Scan(&p.Address, &active, &checked, &p.LastLatency, &p.LastError, &p.SuccessCount, &p.FailureCount); err != nil {
			return nil, err
		}
		p.Active = active == 1
		if checked.Valid {
			if t, err := time.Parse(time.RFC3339, checked.String); err == nil {
				p.LastChecked = &t
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateProxyHealth(ctx context.Context, address string, active bool, latency int64, lastError string) error {
	activeInt := 0
	if active {
		activeInt = 1
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE proxies
SET active = ?, last_checked = ?, last_latency = ?, last_error = ?,
	success_count = success_count + CASE WHEN ? = 1 THEN 1 ELSE 0 END,
	failure_count = failure_count + CASE WHEN ? = 0 THEN 1 ELSE 0 END
WHERE address = ?
`, activeInt, time.Now().UTC().Format(time.RFC3339), latency, lastError, activeInt, activeInt, address)
	return err
}

func (s *Store) DeleteInactiveProxies(ctx context.Context) (int64, error) {
	r, err := s.db.ExecContext(ctx, `DELETE FROM proxies WHERE active = 0`)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

func (s *Store) SetAllProxiesEnabled(ctx context.Context, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE proxies SET active = ?`, v)
	return err
}

func (s *Store) CreateRun(ctx context.Context, runID string, cfg model.LoadTestConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO runs (run_id, state, started_at, config_json, summary_json)
VALUES (?, ?, ?, ?, '{}')
`, runID, model.RunStateRunning, time.Now().UTC().Format(time.RFC3339), string(data))
	return err
}

func (s *Store) SaveRunSample(ctx context.Context, sample model.RunSample) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO run_samples (run_id, timestamp, latency_ms, status_code, success, error_category, error_message, proxy_address, bytes_sent, bytes_received)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, sample.RunID, sample.Timestamp.UTC().Format(time.RFC3339), sample.LatencyMS, sample.StatusCode, boolToInt(sample.Success), sample.ErrorCategory, sample.ErrorMessage, sample.ProxyAddress, sample.BytesSent, sample.BytesReceived)
	return err
}

func (s *Store) UpdateRunSummary(ctx context.Context, runID string, summary model.RunSummary) error {
	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	completedAt := ""
	if summary.CompletedAt != nil {
		completedAt = summary.CompletedAt.UTC().Format(time.RFC3339)
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE runs SET state = ?, completed_at = ?, summary_json = ? WHERE run_id = ?
`, summary.State, completedAt, string(data), runID)
	return err
}

func (s *Store) ListRunSummaries(ctx context.Context) ([]model.RunSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT summary_json FROM runs ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.RunSummary, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if raw == "" || raw == "{}" {
			continue
		}
		var s model.RunSummary
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("decode summary: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *Store) RunSummary(ctx context.Context, runID string) (model.RunSummary, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT summary_json FROM runs WHERE run_id = ?`, runID).Scan(&raw)
	if err != nil {
		return model.RunSummary{}, err
	}
	var out model.RunSummary
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return model.RunSummary{}, err
	}
	return out, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
