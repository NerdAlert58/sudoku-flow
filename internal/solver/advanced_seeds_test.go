package solver_test

// ADR-0019 advanced-tier no-backtracking proof over the 30 generator-produced seeds (MEDIUM +
// HARD + VERY HARD sections of puzzles.txt). This is the advanced-tier sibling of AC-3's
// singles-only TestAC3_Solver_ReplayFromInputProvesNoBacktracking (which stays frozen over the 25
// ORIGINAL seeds): AC-3 proves the singles tier guesses nothing; this proves the FULL ladder
// guesses nothing on real graded puzzles.
//
// Each seed is solved by the shipped solver, its unique solution taken from the brute-force oracle
// (bruteForce, oracle_test.go — D-Q2 uniqueness), and its event log routed through the existing
// advanced replay proof replayAdvancedProvesForced (advanced_ladder_test.go). That helper asserts
// every placement is a forced naked/hidden single under the exact candidate state the recorded
// eliminations produce, every elimination is SOUND against the oracle (never removes a true digit),
// each GridAfter matches the replay, and the final grid == returned solution == oracle. Together
// that is the logic-only guarantee: no backtracking, full ladder (EVAL UC-2).

import (
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// TestAdvancedSeeds_ReplayProvesNoBacktracking proves all 30 advanced seeds solve by sound,
// forced logic. It reuses bruteForce/parseBoard/replayAdvancedProvesForced from the solver_test
// package (no reimplementation). A total elimination count > 0 across the set is the non-vacuity
// witness: replayAdvancedProvesForced only accrues an elimination for a technique at ladder index
// >= 2 (it hard-fails if a singles technique records one), so eliminations > 0 proves at least one
// ADVANCED technique actually fired — otherwise the advanced arms would be untested.
func TestAdvancedSeeds_ReplayProvesNoBacktracking(t *testing.T) {
	seeds := loadAdvancedSeeds(t)
	if len(seeds) != 30 {
		t.Fatalf("expected 30 advanced seeds (MEDIUM+HARD+VERY HARD), got %d", len(seeds))
	}

	totalEliminations := 0
	for n, line := range seeds {
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("advanced seed %d: parse: %v", n+1, err)
		}
		res := solver.Solve(grid)
		if res.Status != solver.StatusSolved || !res.Solved {
			t.Fatalf("advanced seed %d: got status=%q solved=%v, want solved/true\n  %s",
				n+1, res.Status, res.Solved, line)
		}

		// Oracle: the puzzle must have exactly one solution (D-Q2) before we assert the solver's
		// solution equals it inside the replay proof.
		sols := bruteForce(parseBoard(grid.String()), 2)
		if len(sols) != 1 {
			t.Fatalf("advanced seed %d: oracle found %d solutions, want exactly 1 (D-Q2)\n  %s",
				n+1, len(sols), line)
		}

		placements, eliminations := replayAdvancedProvesForced(t, grid.String(), sols[0], res)
		if placements == 0 {
			t.Fatalf("advanced seed %d: replay verified zero placements on a solved seed", n+1)
		}
		totalEliminations += eliminations
	}

	if totalEliminations == 0 {
		t.Fatalf("no advanced-technique elimination fired across any of the 30 seeds — the advanced arms are untested (vacuous)")
	}
}
