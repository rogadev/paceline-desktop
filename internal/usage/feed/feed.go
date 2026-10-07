// Package feed reads the usage file paceline writes from Claude Code's
// status payload each time the status line renders. It is the preferred
// source: local, network-free, and as fresh as the last render.
package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/rogadev/paceline-tray/internal/usage"
)

const (
	// version is the feed format this build understands. paceline bumps it
	// when the format changes incompatibly.
	version  = 1
	maxSize  = 4096
	fileName = "paceline-feed.json"

	// maxUnixSeconds (about the year 2242) keeps an absurd timestamp from
	// overflowing the conversion to int64.
	maxUnixSeconds = 1 << 33

	// DefaultMaxAge is how old a feed may be before paceline-tray prefers
	// another source.
	DefaultMaxAge = 10 * time.Minute
)

type window struct {
	UsedPct  float64 `json:"usedPct"`
	ResetsAt float64 `json:"resetsAt"` // Unix seconds
}

type file struct {
	Version   int     `json:"version"`
	FiveHour  *window `json:"fiveHour"`
	SevenDay  *window `json:"sevenDay"`
	WrittenAt float64 `json:"writtenAt"` // Unix seconds
}

// DefaultPath is the feed file in Claude Code's config directory:
// $CLAUDE_CONFIG_DIR when set, otherwise ~/.claude.
func DefaultPath() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, fileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", fileName), nil
}

// Source reads the feed file at Path. A zero MaxAge means DefaultMaxAge, and
// a nil Now means time.Now.
type Source struct {
	Path   string
	MaxAge time.Duration
	Now    func() time.Time
}

// Fetch returns the feed's reading, timed by when paceline wrote it. A
// missing or stale feed is usage.ErrUnavailable, so the next source gets a
// turn; a feed that exists but cannot be read is a real failure.
func (s *Source) Fetch(context.Context) (usage.Usage, error) {
	f, err := os.Open(s.Path) //nolint:gosec // G304: path is paceline's feed file, set by this program.
	if errors.Is(err, fs.ErrNotExist) {
		return usage.Usage{}, fmt.Errorf("feed: no file at %s: %w", s.Path, usage.ErrUnavailable)
	}
	if err != nil {
		return usage.Usage{}, fmt.Errorf("feed: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only: a close error loses nothing
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return usage.Usage{}, fmt.Errorf("feed: %w", err)
	}
	if len(data) > maxSize {
		return usage.Usage{}, fmt.Errorf("feed: file is larger than %d bytes", maxSize)
	}
	u, err := parse(data)
	if err != nil {
		return usage.Usage{}, err
	}
	if !s.fresh(u.FetchedAt) {
		return usage.Usage{}, fmt.Errorf("feed: written at %s, too old to use: %w",
			u.FetchedAt.Format(time.RFC3339), usage.ErrUnavailable)
	}
	return u, nil
}

// fresh reports whether a feed written at t is recent enough to use. A feed
// from the future is not fresh, so a clock change cannot pin a stale value.
func (s *Source) fresh(t time.Time) bool {
	now, maxAge := time.Now, s.MaxAge
	if s.Now != nil {
		now = s.Now
	}
	if maxAge == 0 {
		maxAge = DefaultMaxAge
	}
	age := now().Sub(t)
	return age >= 0 && age <= maxAge
}

func parse(data []byte) (usage.Usage, error) {
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return usage.Usage{}, fmt.Errorf("feed: %w", err)
	}
	if f.Version != version {
		return usage.Usage{}, fmt.Errorf("feed: version %d, want %d", f.Version, version)
	}
	written, ok := unixSeconds(f.WrittenAt)
	if !ok {
		return usage.Usage{}, errors.New("feed: missing or invalid writtenAt")
	}
	u := usage.Usage{Origin: usage.FromFeed, FetchedAt: written}
	if u.FiveHour, ok = convert(f.FiveHour); !ok {
		return usage.Usage{}, errors.New("feed: invalid fiveHour window")
	}
	if u.SevenDay, ok = convert(f.SevenDay); !ok {
		return usage.Usage{}, errors.New("feed: invalid sevenDay window")
	}
	if !u.Valid() {
		return usage.Usage{}, errors.New("feed: invalid reading")
	}
	return u, nil
}

// convert maps a feed window to a usage window. An absent window is fine;
// one with a missing or invalid reset time is not.
func convert(w *window) (*usage.Window, bool) {
	if w == nil {
		return nil, true
	}
	resets, ok := unixSeconds(w.ResetsAt)
	if !ok {
		return nil, false
	}
	return &usage.Window{UsedPct: w.UsedPct, ResetsAt: resets}, true
}

// unixSeconds converts a positive Unix timestamp to a time truncated to the
// whole second. Every source truncates the same way, because a reset time
// that differs by a fraction of a second reads as a new week and re-anchors
// today's budget.
func unixSeconds(sec float64) (time.Time, bool) {
	if sec <= 0 || sec > maxUnixSeconds {
		return time.Time{}, false
	}
	return time.Unix(int64(sec), 0).UTC(), true
}
