package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// jsonStore persists the config to a single JSON file. It is the default backend
// and the historical format; an existing config.json keeps working untouched.
type jsonStore struct {
	path string
}

// Load reads and parses the JSON file. A missing file returns (nil, nil) so the
// caller seeds a default; any other read/parse error is surfaced.
func (s *jsonStore) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config atomically: marshal, write to a temp file in the same
// directory, fsync, then rename over the target. os.WriteFile truncates-then-
// writes, so a crash mid-write could leave a zero-byte or truncated config and
// destroy every account token; the write-then-rename makes a partial write
// impossible — readers see either the old file or the complete new one.
func (s *jsonStore) Save(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, data)
}

// writeFileAtomic writes data to a temp file in the target's directory, fsyncs,
// then renames over the target so readers see either the old or the complete
// new file, never a partial one.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func (s *jsonStore) Close() error { return nil }

func (s *jsonStore) Backend() string { return "json:" + s.path }
