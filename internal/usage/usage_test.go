package usage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var fetched = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func window(pct float64) *Window {
	return &Window{UsedPct: pct, ResetsAt: fetched.Add(72 * time.Hour)}
}

func good(o Origin) Usage {
	return Usage{FiveHour: window(20), SevenDay: window(40), Origin: o, FetchedAt: fetched}
}

func TestWindowValid(t *testing.T) {
	tests := []struct {
		name string
		w    *Window
		want bool
	}{
		{"nil", nil, false},
		{"zero used", window(0), true},
		{"full", window(100), true},
		{"negative", window(-0.1), false},
		{"over 100", window(100.1), false},
		{"no reset time", &Window{UsedPct: 5}, false},
	}
	for _, tt := range tests {
		if got := tt.w.Valid(); got != tt.want {
			t.Errorf("%s: Valid() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUsageValid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Usage)
		want   bool
	}{
		{"complete", func(*Usage) {}, true},
		{"week only", func(u *Usage) { u.FiveHour = nil }, true},
		{"session only", func(u *Usage) { u.SevenDay = nil }, true},
		{"no windows", func(u *Usage) { u.FiveHour, u.SevenDay = nil, nil }, false},
		{"bad window", func(u *Usage) { u.SevenDay = window(250) }, false},
		{"unknown origin", func(u *Usage) { u.Origin = "scraped" }, false},
		{"no fetch time", func(u *Usage) { u.FetchedAt = time.Time{} }, false},
	}
	for _, tt := range tests {
		u := good(FromFeed)
		tt.mutate(&u)
		if got := u.Valid(); got != tt.want {
			t.Errorf("%s: Valid() = %v, want %v", tt.name, got, tt.want)
		}
	}
	var nilUsage *Usage
	if nilUsage.Valid() {
		t.Error("nil Usage is valid")
	}
}

type fake struct {
	u     Usage
	err   error
	calls int
}

func (f *fake) Fetch(context.Context) (Usage, error) {
	f.calls++
	return f.u, f.err
}

func TestFirstReturnsFirstSuccess(t *testing.T) {
	feed := &fake{err: ErrUnavailable}
	oauth := &fake{u: good(FromOAuth)}
	never := &fake{u: good(FromFeed)}
	u, err := First(context.Background(), feed, oauth, never)
	if err != nil {
		t.Fatal(err)
	}
	if u.Origin != FromOAuth {
		t.Errorf("origin = %q, want oauth", u.Origin)
	}
	if never.calls != 0 {
		t.Error("tried a source after one succeeded")
	}
}

func TestFirstSkipsInvalidReadings(t *testing.T) {
	broken := &fake{u: Usage{Origin: FromFeed, FetchedAt: fetched}}
	oauth := &fake{u: good(FromOAuth)}
	u, err := First(context.Background(), broken, oauth)
	if err != nil || u.Origin != FromOAuth {
		t.Fatalf("First = %+v, %v; want the oauth reading", u, err)
	}
}

func TestFirstAllUnavailable(t *testing.T) {
	_, err := First(context.Background(),
		&fake{err: ErrUnavailable},
		&fake{err: fmt.Errorf("no token: %w", ErrUnavailable)})
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if _, err := First(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no sources: err = %v, want ErrUnavailable", err)
	}
}

func TestFirstReportsRealFailures(t *testing.T) {
	boom := errors.New("HTTP 500")
	_, err := First(context.Background(),
		&fake{err: ErrUnavailable},
		&fake{err: boom},
		&fake{u: Usage{}})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the HTTP failure", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Error("a real failure must not look like an unavailable source")
	}
}
