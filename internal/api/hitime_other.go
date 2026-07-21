//go:build !windows

package api

// High-resolution monotonic timer for non-Windows platforms. Here time.Now() already carries a
// monotonic reading at nanosecond precision, so the same unexported API is a thin wrapper over
// the stdlib — no syscall needed.

import "time"

// hiInstant is an opaque wall/monotonic instant.
type hiInstant time.Time

// hiNow returns the current high-resolution instant.
func hiNow() hiInstant { return hiInstant(time.Now()) }

// hiElapsedMs returns milliseconds elapsed since start as a float64 (sub-ms precision). Uses
// the monotonic clock reading captured by time.Now via time.Since.
func hiElapsedMs(start hiInstant) float64 {
	return float64(time.Since(time.Time(start)).Nanoseconds()) / 1e6
}
