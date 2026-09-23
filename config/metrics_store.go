package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MetricsStore persists per-minute ops metrics so dashboards survive restarts.
// Rows are opaque JSON blobs keyed by unix minute; the proxy owns the schema.
//   - jsonStore: metrics.json beside config.json (whole-map rewrite, atomic).
//   - sqlStore:  metrics_minutes(minute PRIMARY KEY, data TEXT) upserts.
type MetricsStore interface {
	LoadMetrics(sinceMinute int64) (map[int64]string, error)
	SaveMetrics(rows map[int64]string) error
	PruneMetrics(beforeMinute int64) error
}

// BlobStore persists small runtime documents (JSON) that must survive
// restarts, keyed by name: the auto-router's decisions and learned stats, etc.
//   - jsonStore: runtime-<key>.json beside config.json (atomic write).
//   - sqlStore:  runtime_blobs(key PRIMARY KEY, data TEXT).
type BlobStore interface {
	LoadBlob(key string) (string, error)
	SaveBlob(key, data string) error
}

// Blobs returns the runtime blob persistence for the active backend.
func Blobs() BlobStore {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if bs, ok := store.(BlobStore); ok {
		return bs
	}
	return nil
}

// Metrics returns the metrics persistence for the active backend.
func Metrics() MetricsStore {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if ms, ok := store.(MetricsStore); ok {
		return ms
	}
	return nil
}

// ---- JSON file backend ----

func (s *jsonStore) metricsPath() string {
	return filepath.Join(filepath.Dir(s.path), "metrics.json")
}

func (s *jsonStore) readMetricsFile() (map[int64]string, error) {
	data, err := os.ReadFile(s.metricsPath())
	if os.IsNotExist(err) {
		return map[int64]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(raw))
	for k, v := range raw {
		var minute int64
		if _, err := fmt.Sscan(k, &minute); err == nil {
			out[minute] = string(v)
		}
	}
	return out, nil
}

func (s *jsonStore) writeMetricsFile(rows map[int64]string) error {
	raw := make(map[string]json.RawMessage, len(rows))
	for k, v := range rows {
		raw[fmt.Sprint(k)] = json.RawMessage(v)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.metricsPath(), data)
}

func (s *jsonStore) LoadMetrics(sinceMinute int64) (map[int64]string, error) {
	rows, err := s.readMetricsFile()
	if err != nil {
		return nil, err
	}
	for k := range rows {
		if k < sinceMinute {
			delete(rows, k)
		}
	}
	return rows, nil
}

func (s *jsonStore) SaveMetrics(rows map[int64]string) error {
	if len(rows) == 0 {
		return nil
	}
	all, err := s.readMetricsFile()
	if err != nil {
		all = map[int64]string{}
	}
	for k, v := range rows {
		all[k] = v
	}
	return s.writeMetricsFile(all)
}

func (s *jsonStore) PruneMetrics(beforeMinute int64) error {
	all, err := s.readMetricsFile()
	if err != nil || len(all) == 0 {
		return err
	}
	changed := false
	for k := range all {
		if k < beforeMinute {
			delete(all, k)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.writeMetricsFile(all)
}

// ---- SQL backend ----

func (s *sqlStore) LoadMetrics(sinceMinute int64) (map[int64]string, error) {
	rows, err := s.db.Query(s.rebind(`SELECT minute, data FROM metrics_minutes WHERE minute >= ?`), sinceMinute)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var minute int64
		var data string
		if err := rows.Scan(&minute, &data); err != nil {
			return nil, err
		}
		out[minute] = data
	}
	return out, rows.Err()
}

func (s *sqlStore) SaveMetrics(rowsIn map[int64]string) error {
	if len(rowsIn) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt := s.rebind(`INSERT INTO metrics_minutes (minute, data) VALUES (?, ?)
		ON CONFLICT (minute) DO UPDATE SET data = excluded.data`)
	for minute, data := range rowsIn {
		if _, err := tx.Exec(stmt, minute, data); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("save metrics minute %d: %w", minute, err)
		}
	}
	return tx.Commit()
}

func (s *sqlStore) PruneMetrics(beforeMinute int64) error {
	_, err := s.db.Exec(s.rebind(`DELETE FROM metrics_minutes WHERE minute < ?`), beforeMinute)
	return err
}

// ---- runtime blobs ----

func (s *jsonStore) blobPath(key string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, key)
	return filepath.Join(filepath.Dir(s.path), "runtime-"+safe+".json")
}

func (s *jsonStore) LoadBlob(key string) (string, error) {
	data, err := os.ReadFile(s.blobPath(key))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

func (s *jsonStore) SaveBlob(key, data string) error {
	return writeFileAtomic(s.blobPath(key), []byte(data))
}

func (s *sqlStore) LoadBlob(key string) (string, error) {
	var data string
	err := s.db.QueryRow(s.rebind(`SELECT data FROM runtime_blobs WHERE key = ?`), key).Scan(&data)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return data, err
}

func (s *sqlStore) SaveBlob(key, data string) error {
	_, err := s.db.Exec(s.rebind(`INSERT INTO runtime_blobs (key, data) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET data = excluded.data`), key, data)
	return err
}
