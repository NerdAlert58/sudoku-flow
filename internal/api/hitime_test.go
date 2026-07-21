package api

import (
	"testing"
	"time"
)

// TestHiElapsedMs_Sane proves the platform timer measures real elapsed time: over a 2ms sleep
// it returns a value strictly greater than 0 (the whole point of the QPC fix on Windows) and,
// with loose upper bounds to avoid CI flakiness, well under a second.
func TestHiElapsedMs_Sane(t *testing.T) {
	start := hiNow()
	time.Sleep(2 * time.Millisecond)
	ms := hiElapsedMs(start)

	if ms <= 0 {
		t.Errorf("hiElapsedMs over a 2ms sleep = %v, want > 0", ms)
	}
	if ms >= 1000 {
		t.Errorf("hiElapsedMs over a 2ms sleep = %v, want < 1000", ms)
	}
}

// TestHiNow_Monotonic proves two hiNow() reads do not go backwards. hiElapsedMs reads hiNow()
// internally, so a non-negative result means the later read is not before the earlier one —
// a platform-agnostic monotonicity check (hiInstant is int64 on Windows, time.Time elsewhere).
func TestHiNow_Monotonic(t *testing.T) {
	a := hiNow()
	if d := hiElapsedMs(a); d < 0 {
		t.Errorf("hiNow() went backwards: elapsed = %v ms, want >= 0", d)
	}
}
