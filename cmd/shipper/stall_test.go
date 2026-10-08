package main

import (
	"testing"
	"time"
)

func TestCollectorStalled(t *testing.T) {
	now := time.Date(2026, time.March, 8, 14, 15, 0, 0, time.UTC)
	staleAfter := 5 * time.Minute

	cases := map[string]struct {
		lastWrite   time.Time
		wantStalled bool
		wantReason  string
	}{
		"written just now": {
			lastWrite: now,
		},
		"written exactly at the limit": {
			lastWrite: now.Add(-staleAfter),
		},
		"written just over the limit": {
			lastWrite:   now.Add(-staleAfter - time.Second),
			wantStalled: true,
			wantReason:  "collector stalled: last write 5m1s ago",
		},
		"no archive found for the current hour": {
			lastWrite:   time.Time{},
			wantStalled: true,
			wantReason:  "collector stalled: no archive for the current hour",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stalled, reason := collectorStalled(tc.lastWrite, now, staleAfter)

			if stalled != tc.wantStalled {
				t.Errorf("stalled: got %v, want %v", stalled, tc.wantStalled)
			}
			if reason != tc.wantReason {
				t.Errorf("reason: got %q want %v", reason, tc.wantReason)
			}
		})
	}
}
