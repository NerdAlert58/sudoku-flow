package solver_test

// AC-1 (two-tier per-technique ship gate — ADR-0018) and AC-2 (advanced-tier replay).
// Source: DESIGN_DECISIONS §ADR-0018 (two-tier gate), §ADR-0002 (the ladder), §ADR-0001
// (logic-only); EVAL.md §Datasets and fixtures, §Ground-truth process, §Eval matrix → UC-1/UC-2.
//
// ADR-0018 splits the gate:
//   - ISOLABLE tier (strict): >= 3 fixtures where the technique is the EXACT hardest required step
//     — floor (capping at the predecessor does NOT solve) + ceiling (capping at the technique DOES
//     solve). This is the original per-technique gate, kept for every technique that can meet it.
//   - UN-ISOLABLE tier (fallback): >= 1 fixture on which the technique FIRES (emits a witnessed
//     Event) and every one of its eliminations is replay-sound. Used only where generate-and-grade
//     cannot produce 3 exact-ceiling puzzles within the constructive ladder (see MANIFEST.md).
// AC-2 replay soundness (below) is applied to EVERY fixture regardless of tier.

import (
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// AC-1a — SHIP GATE (two-tier): every shipped technique above naked_single is covered by EITHER
// >= 3 isolable fixtures (exact-hardest) OR >= 1 un-isolable fixture (fires + sound). A technique
// with neither is an unverified capability and FAILS the phase (EVAL §Datasets and fixtures,
// relaxed per ADR-0018).
func TestAC1_ShipGate_TwoTierCoverage(t *testing.T) {
	iso := map[string]int{}
	uni := map[string]int{}
	for _, f := range loadAdvancedFixtures(t) {
		if tierIndex(f.technique) < 0 {
			t.Fatalf("fixture labeled with unknown technique %q", f.technique)
		}
		switch f.tier {
		case tierIsolable:
			iso[f.technique]++
		case tierUnisolable:
			uni[f.technique]++
		default:
			t.Fatalf("[%s] unknown tier %q (want %q or %q)", f.technique, f.tier, tierIsolable, tierUnisolable)
		}
	}
	for _, tech := range advTechniques {
		switch {
		case iso[tech] >= 3:
			// strict tier satisfied
		case uni[tech] >= 1:
			// fallback tier satisfied — MANIFEST.md justifies why strict was infeasible
		default:
			t.Errorf("[%s] SHIP GATE FAIL: %d isolable (want >= 3) and %d un-isolable (want >= 1)",
				tech, iso[tech], uni[tech])
		}
	}
}

// AC-1b — GROUND TRUTH + SOLVE/FIRE. Every fixture (all tiers): 81-char, parses, is UNIQUE
// (brute-force count == 1) and its recorded solution equals the oracle's, is 27-constraint valid,
// and preserves the givens. Isolable fixtures additionally SOLVE to that solution; un-isolable
// fixtures need not fully solve (the technique fires mid-solve — see AC-1d), but must never be
// mislabeled unsolvable on a unique grid (EVAL UC-1, ADR-0011).
func TestAC1_AdvancedFixtures_GroundTruthAndSolve(t *testing.T) {
	for _, f := range loadAdvancedFixtures(t) {
		grid, err := sudoku.Parse(f.puzzle)
		if err != nil {
			t.Fatalf("[%s] fixture failed to parse: %v\n  %s", f.technique, err, f.puzzle)
		}
		// Oracle ground truth — required of EVERY fixture.
		if sols := bruteForce(parseBoard(f.puzzle), 2); len(sols) != 1 {
			t.Errorf("[%s] oracle found %d solutions, want exactly 1\n  %s", f.technique, len(sols), f.puzzle)
		} else if sols[0] != f.solution {
			t.Errorf("[%s] recorded solution != oracle recompute\n rec   =%q\n oracle=%q", f.technique, f.solution, sols[0])
		}
		if !constraints27Valid(f.solution) {
			t.Errorf("[%s] recorded solution violates the 27 constraints: %q", f.technique, f.solution)
		}
		if !matchesGivens(grid.String(), f.solution) {
			t.Errorf("[%s] recorded solution overwrites a given clue", f.technique)
		}

		res := solver.Solve(grid)
		if f.tier == tierIsolable {
			if res.Status != solver.StatusSolved || !res.Solved {
				t.Errorf("[%s isolable] got status=%q solved=%v, want solved\n  %s",
					f.technique, res.Status, res.Solved, f.puzzle)
				continue
			}
			if res.Solution != f.solution {
				t.Errorf("[%s isolable] solver != oracle solution\n solver=%q\n oracle=%q", f.technique, res.Solution, f.solution)
			}
		} else {
			// Un-isolable: solved OR stalled are both honest; only unsolvable is wrong here.
			if res.Status == solver.StatusUnsolvable {
				t.Errorf("[%s unisolable] solver returned unsolvable on a UNIQUE puzzle (ADR-0011)\n  %s", f.technique, f.puzzle)
			}
			if res.Status == solver.StatusSolved && res.Solution != f.solution {
				t.Errorf("[%s unisolable] solved but solver != oracle solution", f.technique)
			}
		}
	}
}

// AC-1c — FLOOR / NECESSITY (isolable tier only). Capping the ladder at the fixture's technique
// PREDECESSOR must NOT solve — proving no cheaper tier suffices, so the fixture genuinely REQUIRES
// its technique as the exact hardest step (EVAL §Datasets and fixtures — the floor test).
func TestAC1_Isolable_FloorRequiresTechnique(t *testing.T) {
	for _, f := range loadAdvancedFixtures(t) {
		if f.tier != tierIsolable {
			continue
		}
		k := tierIndex(f.technique)
		if k < 1 {
			t.Fatalf("[%s] technique has no cheaper predecessor to cap at", f.technique)
		}
		predecessor := solver.Technique(expectedLadder[k-1])
		grid, err := sudoku.Parse(f.puzzle)
		if err != nil {
			t.Fatalf("[%s] parse: %v", f.technique, err)
		}
		res := solver.SolveWithMaxTechnique(grid, predecessor)
		if res.Status == solver.StatusSolved {
			t.Errorf("[%s isolable] solved with the ladder capped at %q (cheaper than %q) — the fixture does NOT require %q\n  %s",
				f.technique, predecessor, f.technique, f.technique, f.puzzle)
		}
	}
}

// AC-1d — FIRES (un-isolable tier only). The full Solve(g) event log must contain an Event whose
// Technique is the fixture's label — proving the implementation is exercised (non-vacuous), even
// when the technique is not the puzzle's unique bottleneck (ADR-0018). Soundness of those firings
// is proven by AC-2.
func TestAC1_Unisolable_TechniqueFires(t *testing.T) {
	for _, f := range loadAdvancedFixtures(t) {
		if f.tier != tierUnisolable {
			continue
		}
		grid, err := sudoku.Parse(f.puzzle)
		if err != nil {
			t.Fatalf("[%s] parse: %v", f.technique, err)
		}
		res := solver.Solve(grid)
		fired := false
		for _, ev := range res.Events {
			if ev.Technique == f.technique {
				fired = true
				break
			}
		}
		if !fired {
			t.Errorf("[%s unisolable] technique never fired in the Solve log (status %q, %d events) — fixture does not exercise it\n  %s",
				f.technique, res.Status, len(res.Events), f.puzzle)
		}
	}
}

// AC-2 — REPLAY AT THE ADVANCED TIER (EVERY fixture, all tiers). Replays input → events against a
// candidate model, asserting every placement is a naked/hidden single under the exact candidate
// state the recorded eliminations produce, every elimination is SOUND (never removes the unique
// solution's true digit), each GridAfter matches, and — for a solved fixture — the final grid ==
// solution == oracle. This is the logic-only (no-backtracking) guarantee (EVAL UC-2). It works for
// a stalled un-isolable fixture too: the solution-equality checks are guarded on solved status, so
// a stalled fixture still has every recorded elimination proven sound against its oracle solution.
func TestAC2_AdvancedFixtures_ReplayProvesForced(t *testing.T) {
	for _, f := range loadAdvancedFixtures(t) {
		grid, err := sudoku.Parse(f.puzzle)
		if err != nil {
			t.Fatalf("[%s] parse: %v", f.technique, err)
		}
		res := solver.Solve(grid)
		placements, _ := replayAdvancedProvesForced(t, f.puzzle, f.solution, res)
		if f.tier == tierIsolable && placements == 0 {
			t.Errorf("[%s] replay verified zero placements on a solved fixture", f.technique)
		}
	}
}
