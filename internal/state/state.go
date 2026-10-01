// Package state stores the latest usage reading in a file shared between
// processes: the tray app writes it after each poll, and the MCP server reads
// it instead of polling on its own.
package state

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rogadev/paceline-desktop/internal/usage"
)

const (
	// version is bumped when the file format changes incompatibly, so an
	// older or newer build ignores a file it cannot read.
	version = 1
	maxSize = 4096
)

type file struct {
	Version int         `json:"version"`
	Usage   usage.Usage `json:"usage"`
}

// DefaultPath is the state file in the user's cache directory.
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "paceline-desktop", "usage.json"), nil
}

// Read loads the stored reading. It returns nil for a missing, oversized,
// corrupt, wrong-version, or invalid file, since every one of those means
// "fetch a fresh reading".
func Read(path string) *usage.Usage {
	f, err := os.Open(path) //nolint:gosec // G304: path is paceline-desktop's own state file.
	if err != nil {
		return nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil || len(data) > maxSize {
		return nil
	}
	var s file
	if json.Unmarshal(data, &s) != nil || s.Version != version || !s.Usage.Valid() {
		return nil
	}
	return &s.Usage
}

// Write saves u through a temp file and a rename, so a reader never sees a
// half-written file. It creates the directory if needed.
func Write(path string, u usage.Usage) error {
	if !u.Valid() {
		return errors.New("state: refusing to store an invalid reading")
	}
	data, err := json.Marshal(file{Version: version, Usage: u})
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Fresh reports whether u was fetched within maxAge of now. A reading from
// the future is not fresh, so a clock change cannot pin a stale value.
func Fresh(u *usage.Usage, now time.Time, maxAge time.Duration) bool {
	if u == nil {
		return false
	}
	age := now.Sub(u.FetchedAt)
	return age >= 0 && age <= maxAge
}
