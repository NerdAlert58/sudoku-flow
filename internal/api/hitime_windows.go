//go:build windows

package api

// High-resolution monotonic timer for Windows (Option A / ADR-0007, P3). time.Now() on Windows
// has ~0.5ms resolution, which reads a single ~18µs solve as 0. QueryPerformanceCounter (QPC)
// is a monotonic high-frequency counter; dividing the tick delta by the (fixed-per-boot)
// QueryPerformanceFrequency yields real sub-millisecond elapsed values. Backed by the stdlib
// syscall + unsafe only — no external dependency.

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procQueryPerformanceCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")

	// qpcFreq is the counter frequency in ticks/second, read once (it is fixed per boot).
	qpcFreq = readQPCFrequency()
)

// hiInstant is an opaque QPC tick count.
type hiInstant int64

func readQPCFrequency() int64 {
	var freq int64
	// The BOOL return is ignored: QPC/QPF are documented to always succeed on Windows XP+.
	procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&freq)))
	if freq <= 0 {
		return 1 // defensive: never divide by zero.
	}
	return freq
}

func qpcCount() int64 {
	var count int64
	procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&count)))
	return count
}

// hiNow returns the current high-resolution instant.
func hiNow() hiInstant { return hiInstant(qpcCount()) }

// hiElapsedMs returns milliseconds elapsed since start as a float64 (sub-ms precision).
func hiElapsedMs(start hiInstant) float64 {
	return float64(int64(hiNow())-int64(start)) / float64(qpcFreq) * 1000.0
}
