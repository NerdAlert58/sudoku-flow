package solver_test

// P-4 intra-puzzle scan-parallel correctness test (RED until the builder implements
// solver.SolveParallel).
//
// Test-defined SOURCE SURFACE the builder implements:
//
//	// SolveParallel runs the SAME deterministic constructive solve as Solve, but parallelises the
//	// READ-ONLY per-pass technique SCAN within a pass. The productive-step selection stays
//	// deterministic (cheapest-first, row-major tie-break, ADR-0012), so SolveParallel(g) returns a
//	// SolveResult IDENTICAL to Solve(g): same Status/Solved/Solution, byte-identical event log, and
//	// identical counter quartet (eventCount, iterations, candidateChecks). It is the flagged
//	// intra-puzzle variant of ADR-0006 — shipped only to be benchmarked as a measured negative
//	// result, never as a speed feature.
//	func SolveParallel(g sudoku.Grid) SolveResult
//
// This is the ASSERTABLE half of ADR-0006 / USERS §UC-5: a parallel variant must not change the
// answer. The (non-)speedup half is measured by the benchmarks in solve_parallel_bench_test.go —
// this test deliberately does NOT assert "parallel is slower" (that would be flaky).
//
// Reuses loadPuzzles / parseBoard etc. from oracle_test.go (same package solver_test).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// assertParallelEqualsSequential fails unless SolveParallel(g) is identical to Solve(g) on every
// field that is part of the deterministic set (ADR-0012). SolveTimeMs is excluded (Solve leaves
// it zero and it is wall-clock per ADR-0007).
func assertParallelEqualsSequential(t *testing.T, label string, g sudoku.Grid) {
	t.Helper()
	seq := solver.Solve(g)
	par := solver.SolveParallel(g)

	if par.Status != seq.Status || par.Solved != seq.Solved {
		t.Fatalf("%s: status/solved differ: parallel=%q/%v sequential=%q/%v",
			label, par.Status, par.Solved, seq.Status, seq.Solved)
	}
	if par.Solution != seq.Solution {
		t.Fatalf("%s: solution differs\n parallel  =%q\n sequential=%q", label, par.Solution, seq.Solution)
	}
	if par.EventCount != seq.EventCount || par.Iterations != seq.Iterations || par.CandidateChecks != seq.CandidateChecks {
		t.Fatalf("%s: counter quartet differs: parallel{ec:%d it:%d cc:%d} sequential{ec:%d it:%d cc:%d}",
			label, par.EventCount, par.Iterations, par.CandidateChecks,
			seq.EventCount, seq.Iterations, seq.CandidateChecks)
	}
	if par.HardestTechnique != seq.HardestTechnique {
		t.Fatalf("%s: hardestTechnique differs: parallel=%q sequential=%q", label, par.HardestTechnique, seq.HardestTechnique)
	}
	pj, _ := json.Marshal(par.Events)
	sj, _ := json.Marshal(seq.Events)
	if string(pj) != string(sj) {
		t.Fatalf("%s: event log not byte-identical between SolveParallel and Solve", label)
	}
}

// AC-6a (ADR-0006, USERS §UC-5): on every seed puzzle, the intra-puzzle scan-parallel variant
// produces IDENTICAL solutions and event logs to the sequential Solve.
func TestAC6_SolveParallel_IdenticalToSequentialOnSeeds(t *testing.T) {
	puzzles := loadPuzzles(t)
	if len(puzzles) != 25 {
		t.Fatalf("expected 25 seed puzzles, got %d", len(puzzles))
	}
	for n, line := range puzzles {
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("puzzle %d: parse: %v", n+1, err)
		}
		assertParallelEqualsSequential(t, "seed puzzle "+line, grid)
	}
}

// AC-6a (terminal-status coverage): the parallel variant must also agree with Solve on the
// non-solved terminal outcomes — the empty grid (stalled) and an in-tier zero-candidate grid
// (unsolvable) — so the equivalence is not solved-path-only.
func TestAC6_SolveParallel_IdenticalOnStalledAndUnsolvable(t *testing.T) {
	stalled, err := sudoku.Parse(strings.Repeat("0", 81))
	if err != nil {
		t.Fatalf("empty grid must parse: %v", err)
	}
	assertParallelEqualsSequential(t, "empty/stalled grid", stalled)

	unsolvable, err := sudoku.Parse("012345678" + "900000000" + strings.Repeat("000000000", 7))
	if err != nil {
		t.Fatalf("unsolvable fixture must parse: %v", err)
	}
	assertParallelEqualsSequential(t, "in-tier zero-candidate/unsolvable grid", unsolvable)
}

// AC-6a (determinism, ADR-0012): SolveParallel is itself deterministic across repeated runs —
// a parallel scan must not introduce run-to-run variance in the event log or counters.
func TestAC6_SolveParallel_DeterministicAcrossRuns(t *testing.T) {
	grid, err := sudoku.Parse(loadPuzzles(t)[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := solver.SolveParallel(grid)
	b := solver.SolveParallel(grid)
	if a.Solution != b.Solution || a.EventCount != b.EventCount || a.Iterations != b.Iterations || a.CandidateChecks != b.CandidateChecks {
		t.Fatalf("SolveParallel not deterministic across runs")
	}
	aj, _ := json.Marshal(a.Events)
	bj, _ := json.Marshal(b.Events)
	if string(aj) != string(bj) {
		t.Fatalf("SolveParallel event log not byte-identical across runs")
	}
}
