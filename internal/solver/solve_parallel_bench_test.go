package solver_test

// P-4 serial-vs-parallel benchmark (RED until the builder implements solver.SolveParallel).
//
// ADR-0006 / AUDIT §P2 / USERS §UC-5: intra-puzzle scan parallelism is published as a MEASURED
// NEGATIVE RESULT, not a speed feature. These two benchmarks run the SAME puzzles through the
// sequential Solve and the intra-puzzle-parallel SolveParallel so the coordinator can run
//
//	go test -bench=. -benchmem ./internal/solver/
//
// and record the (non-)speedup. The tests deliberately assert NOTHING about relative speed —
// "parallel is slower" is flaky wall-clock and would gate CI on a race; the assertable
// correctness claim lives in solve_parallel_test.go (SolveParallel == Solve). This file exists so
// the negative result is a reproducible number in the build log, per ADR-0006.
//
// benchGrids is a bench-scoped loader (oracle_test.go's loadPuzzles takes *testing.T; a benchmark
// only has *testing.B), CRLF-safe like the shipped loaders (D-Q1). It loads the FULL graded corpus
// (all 55 data lines across ORIGINAL + MEDIUM + HARD + VERY HARD, ADR-0019) — benchmarking the
// whole difficulty range is the ADR-0006 intent — and skips '#' section headers and blank lines.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

func benchGrids(b *testing.B) []sudoku.Grid {
	b.Helper()
	path := filepath.Join("..", "..", "puzzles.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("reading seed puzzles at %s: %v", path, err)
	}
	var grids []sudoku.Grid
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		// Skip blank lines and '#' lines (tier-section headers + comments, ADR-0019) so a header
		// never parses as a grid; the remaining data lines are the FULL graded corpus (all tiers).
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		g, err := sudoku.Parse(line)
		if err != nil {
			b.Fatalf("seed failed to parse: %v", err)
		}
		grids = append(grids, g)
	}
	if len(grids) == 0 {
		b.Fatal("no seed puzzles loaded")
	}
	return grids
}

// BenchmarkSolveSequential measures the shipped single-threaded solver over the full graded corpus.
func BenchmarkSolveSequential(b *testing.B) {
	grids := benchGrids(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, g := range grids {
			_ = solver.Solve(g)
		}
	}
}

// BenchmarkSolveIntraParallel measures the flagged intra-puzzle scan-parallel variant over the
// SAME full graded corpus. Compared against BenchmarkSolveSequential this is the ADR-0006 negative
// result: a sub-millisecond 9x9 solve cannot amortise goroutine overhead (AUDIT §P2).
func BenchmarkSolveIntraParallel(b *testing.B) {
	grids := benchGrids(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, g := range grids {
			_ = solver.SolveParallel(g)
		}
	}
}
