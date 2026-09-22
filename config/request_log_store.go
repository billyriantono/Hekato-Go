package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// RequestLogStore persists the per-request telemetry lines (success/failure
// records surfaced via GET /logs and the dashboard telemetry page) so they
// survive process restarts. Backends:
//   - jsonStore: request_logs.json beside config.json (whole-array rewrite, atomic).
//   - sqlStore:  request_logs(time BIGINT PRIMARY KEY, data TEXT) — the time
//     key matches RequestLog.Time so ON CONFLICT keeps the most recent write.
type RequestLogStore interface {
	// Load returns up to `limit` entries, ordered by time ascending (oldest first).
	// Returns an empty slice (not nil) when the backend has no data yet.
	LoadRecent(limit int) ([]PersistedRequestLog, error)
	// Append merges entries (newest first); backend dedupes on Time to handle
	// retries without growing duplicates.
	Append(entries []PersistedRequestLog) error
	// Clear wipes every persisted entry (used by DELETE /logs).
	Clear() error
}

// PersistedRequestLog is the wire shape stored on disk. Field tags match the
// proxy's RequestLog exactly so JSON round-trips byte-for-byte and the SQL TEXT
// column can hold the verbatim JSON of an entry produced by the proxy.
type PersistedRequestLog struct {
	Time      int64   `json:"time"`
	Endpoint  string  `json:"endpoint"`
	Model     string  `json:"model"`
	AccountID string  `json:"accountId"`
	Status    string  `json:"status"`
	Error     string  `json:"error"`
	ErrorType string  `json:"errorType"`
	Tokens    int     `json:"tokens"`
	Credits   float64 `json:"credits"`
	Duration  int64   `json:"duration"`
}

// RequestLogs returns the request-log persistence for the active backend, or
// nil when the backend cannot host it (the proxy treats nil as "in-memory only",
// preserving the previous volatile behaviour).
func RequestLogs() RequestLogStore {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if rs, ok := store.(RequestLogStore); ok {
		return rs
	}
	return nil
}

// ---- JSON file backend ----

func (s *jsonStore) requestLogsPath() string {
	return filepath.Join(filepath.Dir(s.path), "request_logs.json")
}

func (s *jsonStore) readRequestLogsFile() ([]PersistedRequestLog, error) {
	data, err := os.ReadFile(s.requestLogsPath())
	if os.IsNotExist(err) {
		return []PersistedRequestLog{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []PersistedRequestLog
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *jsonStore) writeRequestLogsFile(rows []PersistedRequestLog) error {
	// Cap on disk to limit growth: keep at most 4× the in-memory ring so a
	// restart that imports stale lines doesn't immediately overflow memory.
	const diskCap = requestLogsDiskCap
	if len(rows) > diskCap {
		rows = rows[len(rows)-diskCap:]
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.requestLogsPath(), data)
}

// LoadRecent returns up to `limit` entries, oldest first.
func (s *jsonStore) LoadRecent(limit int) ([]PersistedRequestLog, error) {
	rows, err := s.readRequestLogsFile()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Time < rows[j].Time })
	if limit > 0 && len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows, nil
}

// Append merges new entries (assumed newest-first by the caller). Returns a
// short error tag rather than aborting the request path — telemetry write
// failures must never break a chat completion.
func (s *jsonStore) Append(entries []PersistedRequestLog) error {
	if len(entries) == 0 {
		return nil
	}
	existing, err := s.readRequestLogsFile()
	if err != nil {
		return err
	}
	// Dedupe by Time: when two loggers race, the second write loses silently
	// — the in-memory ring is the source of truth for ordering, disk is for
	// restart survival only.
	byTime := make(map[int64]PersistedRequestLog, len(existing)+len(entries))
	for _, e := range existing {
		byTime[e.Time] = e
	}
	for _, e := range entries {
		byTime[e.Time] = e
	}
	merged := make([]PersistedRequestLog, 0, len(byTime))
	for _, e := range byTime {
		merged = append(merged, e)
	}
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Time < merged[j].Time })
	return s.writeRequestLogsFile(merged)
}

// Clear wipes the on-disk log file.
func (s *jsonStore) Clear() error {
	data, err := os.ReadFile(s.requestLogsPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_ = data
	return os.Remove(s.requestLogsPath())
}

// requestLogsDiskCap is the on-disk ceiling; 4× the in-memory ring. The cap
// exists so a long-running host doesn't grow request_logs.json without bound
// even if the in-memory ring was somehow bypassed.
const requestLogsDiskCap = 2000

// ---- SQL backend ----

// AppendRequestLogs inserts/updates a batch of entries, deduplicating by
// the Time PK. Newest-first by caller.
func (s *sqlStore) AppendRequestLogs(entries []PersistedRequestLog) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt := s.rebind(`INSERT INTO request_logs (ts, data) VALUES (?, ?)
		ON CONFLICT (ts) DO UPDATE SET data = excluded.data`)
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("marshal request log: %w", err)
		}
		if _, err := tx.Exec(stmt, e.Time, string(data)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert request log %d: %w", e.Time, err)
		}
	}
	return tx.Commit()
}

func (s *sqlStore) LoadRecentRequestLogs(limit int) ([]PersistedRequestLog, error) {
	q := `SELECT data FROM request_logs ORDER BY ts ASC`
	if limit > 0 {
		// Subquery to avoid parameterising LIMIT, which differs across drivers.
		q = `SELECT data FROM (
			SELECT data, ts FROM request_logs ORDER BY ts DESC LIMIT ?
		) ORDER BY ts ASC`
	}
	rows, err := s.db.Query(s.rebind(q), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PersistedRequestLog, 0, 64)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var e PersistedRequestLog
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			// Skip a corrupted row rather than failing the whole load — the
			// in-memory buffer is the source of truth for the current session.
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *sqlStore) ClearRequestLogs() error {
	_, err := s.db.Exec(`DELETE FROM request_logs`)
	return err
}

// LoadRecent satisfies RequestLogStore. Limit 0 returns everything.
func (s *sqlStore) LoadRecent(limit int) ([]PersistedRequestLog, error) {
	return s.LoadRecentRequestLogs(limit)
}

// Append satisfies RequestLogStore.
func (s *sqlStore) Append(entries []PersistedRequestLog) error {
	return s.AppendRequestLogs(entries)
}

// Clear satisfies RequestLogStore.
func (s *sqlStore) Clear() error {
	return s.ClearRequestLogs()
}
