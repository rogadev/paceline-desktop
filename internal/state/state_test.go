package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogadev/paceline-tray/internal/usage"
)

var now = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func reading() usage.Usage {
	return usage.Usage{
		FiveHour:  &usage.Window{UsedPct: 21, ResetsAt: now.Add(3 * time.Hour)},
		SevenDay:  &usage.Window{UsedPct: 46, ResetsAt: now.Add(80 * time.Hour)},
		Origin:    usage.FromOAuth,
		FetchedAt: now,
	}
}

func TestWriteThenRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "usage.json")
	if err := Write(path, reading()); err != nil {
		t.Fatal(err)
	}
	got := Read(path)
	if got == nil {
		t.Fatal("Read returned nil after Write")
	}
	if got.SevenDay.UsedPct != 46 || !got.SevenDay.ResetsAt.Equal(now.Add(80*time.Hour)) {
		t.Errorf("SevenDay = %+v", got.SevenDay)
	}
	if got.Origin != usage.FromOAuth || !got.FetchedAt.Equal(now) {
		t.Errorf("Origin, FetchedAt = %q, %v", got.Origin, got.FetchedAt)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestWriteReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	first := reading()
	if err := Write(path, first); err != nil {
		t.Fatal(err)
	}
	second := reading()
	second.SevenDay.UsedPct = 60
	if err := Write(path, second); err != nil {
		t.Fatal(err)
	}
	if got := Read(path); got == nil || got.SevenDay.UsedPct != 60 {
		t.Errorf("Read = %+v, want the second reading", got)
	}
}

func TestWriteRejectsInvalidReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := Write(path, usage.Usage{}); err == nil {
		t.Fatal("Write accepted an empty reading")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Write created a file for an invalid reading")
	}
}

func TestWriteFailsWhenDirectoryIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(blocker, "usage.json"), reading()); err == nil {
		t.Error("Write succeeded under a regular file")
	}
}

func TestWriteFailsWhenTargetIsADirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")
	if err := os.MkdirAll(filepath.Join(path, "occupied"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, reading()); err == nil {
		t.Error("Write replaced a directory")
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestReadRejectsBadFiles(t *testing.T) {
	tests := map[string]string{
		"corrupt":       `{"version": 1, "usage": `,
		"wrong version": `{"version": 2, "usage": {"sevenDay": {"usedPct": 5, "resetsAt": "2026-10-04T00:00:00Z"}, "origin": "feed", "fetchedAt": "2026-10-01T09:00:00Z"}}`,
		"invalid usage": `{"version": 1, "usage": {"sevenDay": {"usedPct": 500, "resetsAt": "2026-10-04T00:00:00Z"}, "origin": "feed", "fetchedAt": "2026-10-01T09:00:00Z"}}`,
		"wrong types":   `{"version": "1", "usage": []}`,
		"oversized":     `{"pad": "` + strings.Repeat("x", maxSize) + `"}`,
	}
	dir := t.TempDir()
	for name, body := range tests {
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := Read(path); got != nil {
			t.Errorf("%s: Read = %+v, want nil", name, got)
		}
	}
	if Read(filepath.Join(dir, "missing.json")) != nil {
		t.Error("missing file: Read returned a reading")
	}
}

func TestFresh(t *testing.T) {
	u := reading()
	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"just fetched", now, true},
		{"at the limit", now.Add(5 * time.Minute), true},
		{"too old", now.Add(5*time.Minute + time.Second), false},
		{"from the future", now.Add(-time.Second), false},
	}
	for _, tt := range tests {
		if got := Fresh(&u, tt.at, 5*time.Minute); got != tt.want {
			t.Errorf("%s: Fresh = %v, want %v", tt.name, got, tt.want)
		}
	}
	if Fresh(nil, now, time.Hour) {
		t.Error("nil reading is fresh")
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Skipf("no cache dir on this machine: %v", err)
	}
	if filepath.Base(path) != "usage.json" || filepath.Base(filepath.Dir(path)) != "paceline-tray" {
		t.Errorf("DefaultPath = %q", path)
	}
}
