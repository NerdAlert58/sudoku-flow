package solver

import (
	"sync"

	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// SolveParallel runs the SAME deterministic constructive solve as Solve, parallelising ONLY the
// read-only per-pass candidate scan. It MUST return a SolveResult byte-identical to Solve(g):
// same Status/Solved/Solution, identical Events (order + content), identical counter quartet
// (EventCount/Iterations/CandidateChecks) and HardestTechnique. The productive-step selection
// stays the identical sequential cheapest-first, row-major logic (ADR-0012); only the candidate
// derivation fans out. This is the ADR-0006 flagged intra-puzzle variant — it exists to be
// benchmarked as a MEASURED NEGATIVE RESULT: a sub-millisecond 9x9 solve cannot amortise
// goroutine overhead, so the parallel scan buys no speedup (AUDIT §P2). Correctness,
// race-freedom, and the benchmark number are the whole point; speed is not.
func SolveParallel(g sudoku.Grid) SolveResult {
	return runEngine(g, computeCandidatesParallel, len(ladderTechniques)-1)
}

// scanBands is the goroutine fan-out width for the parallel candidate scan: three bands of 27
// cells (rows 0-2, 3-5, 6-8). Small and fixed — this is the ADR-0006 variant that demonstrates
// goroutine overhead swamps a 9x9 solve, not a tuned worker pool.
const scanBands = 3

// computeCandidatesParallel is the read-only, concurrent twin of computeCandidates. It produces a
// byte-identical cand set and zero flag. The peer-mask accumulation (rows/cols/boxes) runs once,
// sequentially, into fully-populated read-only arrays. Then the per-cell candidate derivation is
// fanned out across scanBands goroutines, each writing ONLY its own disjoint cand indices while
// reading the shared masks read-only — disjoint writes to distinct array elements are race-free
// (verified under `go test -race`). The checks counter and zero flag are folded AFTER the join,
// in fixed row-major order, so CandidateChecks is deterministic and identical to the sequential
// path (ADR-0007/0012) rather than depending on goroutine scheduling.
func computeCandidatesParallel(b *[81]uint8, checks *int) (cand [81]uint16, zero bool) {
	var rows, cols, boxes [9]uint16
	for i := 0; i < 81; i++ {
		if v := b[i]; v != 0 {
			bit := uint16(1) << v
			r, c := i/9, i%9
			rows[r] |= bit
			cols[c] |= bit
			boxes[(r/3)*3+c/3] |= bit
		}
	}

	var wg sync.WaitGroup
	wg.Add(scanBands)
	for band := 0; band < scanBands; band++ {
		go func(band int) {
			defer wg.Done()
			for i := band * 27; i < band*27+27; i++ {
				if b[i] != 0 {
					continue // filled cell: cand[i] stays the zero value, as in the sequential path
				}
				r, c := i/9, i%9
				cand[i] = allDigits &^ (rows[r] | cols[c] | boxes[(r/3)*3+c/3])
			}
		}(band)
	}
	wg.Wait()

	// Deterministic fold: count one check per empty cell in row-major order and detect a
	// zero-candidate cell, exactly as computeCandidates does, independent of scan scheduling.
	for i := 0; i < 81; i++ {
		if b[i] != 0 {
			continue
		}
		*checks++
		if cand[i] == 0 {
			zero = true
		}
	}
	return cand, zero
}
