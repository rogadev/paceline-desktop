package feed

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogadev/paceline-tray/internal/usage"
)

// written is the writtenAt of the sample feed; now is two minutes later.
var (
	written = time.Unix(1790277013, 0).UTC()
	now     = written.Add(2 * time.Minute)
)

const sample = `{
  "version": 1,
  "fiveHour": { "usedPct": 21, "resetsAt": 1790283000 },
  "sevenDay": { "usedPct": 46, "resetsAt": 1790838000 },
  "writtenAt": 1790277013
}`

func source(t *testing.T, content string) *Source {
	t.Helper()
	path := filepath.Join(t.TempDir(), fileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Source{Path: path, Now: func() time.Time { return now }}
}

func TestFetchReadsTheFeed(t *testing.T) {
	u, err := source(t, sample).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Origin != usage.FromFeed || !u.FetchedAt.Equal(written) {
		t.Errorf("Origin, FetchedAt = %q, %v", u.Origin, u.FetchedAt)
	}
	if u.FiveHour.UsedPct != 21 || !u.FiveHour.ResetsAt.Equal(time.Unix(1790283000, 0)) {
		t.Errorf("FiveHour = %+v", u.FiveHour)
	}
	if u.SevenDay.UsedPct != 46 || !u.SevenDay.ResetsAt.Equal(time.Unix(1790838000, 0)) {
		t.Errorf("SevenDay = %+v", u.SevenDay)
	}
}

func TestFetchAllowsOneMissingWindow(t *testing.T) {
	u, err := source(t, `{"version":1,"sevenDay":{"usedPct":46,"resetsAt":1790838000},"writtenAt":1790277013}`).
		Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.FiveHour != nil || u.SevenDay == nil {
		t.Errorf("FiveHour, SevenDay = %+v, %+v", u.FiveHour, u.SevenDay)
	}
}

// A reset time that differs by a fraction of a second would read as a new
// week and re-anchor today's budget, so every source truncates to the second.
func TestFetchTruncatesResetsAtToWholeSeconds(t *testing.T) {
	u, err := source(t, strings.Replace(sample, "1790838000", "1790838000.75", 1)).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Unix(1790838000, 0); !u.SevenDay.ResetsAt.Equal(want) {
		t.Errorf("SevenDay.ResetsAt = %v, want %v", u.SevenDay.ResetsAt, want)
	}
	if u.SevenDay.ResetsAt.Nanosecond() != 0 {
		t.Errorf("ResetsAt keeps %dns", u.SevenDay.ResetsAt.Nanosecond())
	}
}

func TestFetchMissingFileIsUnavailable(t *testing.T) {
	s := &Source{Path: filepath.Join(t.TempDir(), fileName)}
	if _, err := s.Fetch(context.Background()); !errors.Is(err, usage.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestFetchFreshness(t *testing.T) {
	tests := []struct {
		name   string
		now    time.Time
		maxAge time.Duration
		ok     bool
	}{
		{"just written", written, 0, true},
		{"at the default limit", written.Add(DefaultMaxAge), 0, true},
		{"past the default limit", written.Add(DefaultMaxAge + time.Second), 0, false},
		{"within a custom limit", written.Add(time.Hour), 2 * time.Hour, true},
		{"from the future", written.Add(-time.Second), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := source(t, sample)
			s.Now = func() time.Time { return tt.now }
			s.MaxAge = tt.maxAge
			_, err := s.Fetch(context.Background())
			if tt.ok && err != nil {
				t.Errorf("err = %v, want a reading", err)
			}
			if !tt.ok && !errors.Is(err, usage.ErrUnavailable) {
				t.Errorf("err = %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestFetchRejectsBadFiles(t *testing.T) {
	tests := map[string]string{
		"corrupt":             `{"version":`,
		"oversized":           sample + strings.Repeat(" ", maxSize),
		"wrong version":       strings.Replace(sample, `"version": 1`, `"version": 2`, 1),
		"no windows":          `{"version":1,"writtenAt":1790277013}`,
		"no writtenAt":        strings.Replace(sample, `"writtenAt": 1790277013`, `"writtenAt": 0`, 1),
		"zero resetsAt":       strings.Replace(sample, "1790283000", "0", 1),
		"negative resetsAt":   strings.Replace(sample, "1790283000", "-5", 1),
		"absurd resetsAt":     strings.Replace(sample, "1790283000", "1e300", 1),
		"usedPct over 100":    strings.Replace(sample, `"usedPct": 21`, `"usedPct": 101`, 1),
		"negative usedPct":    strings.Replace(sample, `"usedPct": 46`, `"usedPct": -1`, 1),
		"usedPct as a string": strings.Replace(sample, `"usedPct": 21`, `"usedPct": "21"`, 1),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := source(t, content).Fetch(context.Background())
			if err == nil {
				t.Fatal("Fetch succeeded")
			}
			if errors.Is(err, usage.ErrUnavailable) {
				t.Errorf("err = %v; a broken feed is a failure, not unavailable", err)
			}
		})
	}
}

func TestDefaultPath(t *testing.T) {
	t.Run("CLAUDE_CONFIG_DIR", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", dir)
		got, err := DefaultPath()
		if err != nil || got != filepath.Join(dir, fileName) {
			t.Errorf("DefaultPath() = %q, %v", got, err)
		}
	})
	t.Run("home", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip(err)
		}
		got, err := DefaultPath()
		if err != nil || got != filepath.Join(home, ".claude", fileName) {
			t.Errorf("DefaultPath() = %q, %v", got, err)
		}
	})
}
