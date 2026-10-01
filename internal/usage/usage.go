// Package usage describes the Claude plan limits paceline-desktop tracks and
// the sources that report them.
//
// Every source normalizes to the same Usage value, so the tray, the MCP
// server, and the pace math never care where a reading came from.
package usage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Window is one rate-limit window: the five-hour session or the seven-day week.
type Window struct {
	UsedPct  float64   `json:"usedPct"`
	ResetsAt time.Time `json:"resetsAt"`
}

// Valid reports whether w is a usable reading. A nil window is not valid.
func (w *Window) Valid() bool {
	return w != nil && w.UsedPct >= 0 && w.UsedPct <= 100 && !w.ResetsAt.IsZero()
}

// Origin names the source a reading came from.
type Origin string

// Known origins.
const (
	FromFeed  Origin = "feed"  // the file paceline writes from Claude Code's status payload
	FromOAuth Origin = "oauth" // the usage endpoint, polled with Claude Code's stored token
)

// Usage is one reading of both windows. Either window may be nil when the
// source did not report it, but not both.
type Usage struct {
	FiveHour  *Window   `json:"fiveHour,omitempty"`
	SevenDay  *Window   `json:"sevenDay,omitempty"`
	Origin    Origin    `json:"origin"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// Valid reports whether u has a known origin, a fetch time, at least one
// window, and no malformed window.
func (u *Usage) Valid() bool {
	if u == nil || (u.Origin != FromFeed && u.Origin != FromOAuth) || u.FetchedAt.IsZero() {
		return false
	}
	if u.FiveHour == nil && u.SevenDay == nil {
		return false
	}
	return (u.FiveHour == nil || u.FiveHour.Valid()) && (u.SevenDay == nil || u.SevenDay.Valid())
}

// ErrUnavailable means a source has nothing to offer on this machine, such as
// no feed file or no stored token. It is expected, unlike a failed fetch.
var ErrUnavailable = errors.New("usage source unavailable")

// Source reports current usage.
type Source interface {
	Fetch(ctx context.Context) (Usage, error)
}

// First returns the first valid reading from sources, tried in order. When
// none succeed, it returns ErrUnavailable if no source had anything to offer,
// and otherwise joins the real failures, leaving out unavailable sources.
func First(ctx context.Context, sources ...Source) (Usage, error) {
	var failures []error
	for i, s := range sources {
		u, err := s.Fetch(ctx)
		if err == nil && !u.Valid() {
			err = fmt.Errorf("source %d returned an invalid reading", i)
		}
		switch {
		case err == nil:
			return u, nil
		case !errors.Is(err, ErrUnavailable):
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		return Usage{}, ErrUnavailable
	}
	return Usage{}, errors.Join(failures...)
}
