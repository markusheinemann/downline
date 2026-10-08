package main

import (
	"fmt"
	"time"
)

// collectorStalled reports whether the collector has not written to the
// current archive for longer than staleAfter.
func collectorStalled(lastWrite, now time.Time, staleAfter time.Duration) (bool, string) {
	if lastWrite.IsZero() {
		return true, "collector stalled: no archive for the current hour"
	}
	if since := now.Sub(lastWrite); since > staleAfter {
		return true, fmt.Sprintf("collector stalled: last write %s ago", since.Truncate(time.Second))
	}
	return false, ""
}
