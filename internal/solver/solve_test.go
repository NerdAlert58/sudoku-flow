package solver_test

// P-1 solver-core tests. These drive the test-facing API the builder implements:
//
//	package solver
//
//	type Status string
//	const (
//	    StatusSolved       Status = "solved"
//	    StatusStalled      Status = "stalled"
//	    StatusUnsolvable   Status = "unsolvable"
//	    StatusInvalidInput Status = "invalid_input"
//	)
//	type Cell struct { Row, Col int }
//	type Placement struct { Cell Cell; Value int }
//	type Elimination struct { Cell Cell; Candidate int }
//	type Event struct {
//	    Seq          int
//	    Technique    string          // canonical: "naked_single" | "hidden_single"
//	    WitnessCells []Cell
//	    Placement    *Placement      // set iff the effect is a placement
//	    Eliminations []Elimination   // set iff the effect is candidate eliminations
//	    GridAfter    string          // canonical 81-char grid AFTER this step
//	}
//	type SolveResult struct {
//	    Status          Status
//	    Solved          bool
//	    Solution        string       // canonical 81-char grid when solved
//	    Events          []Event
//	    EventCount      int          // == len(Events)
//	    Iterations      int          // main-loop scan passes (ADR-0007)
//	    CandidateChecks int          // candidate-cell inspections (ADR-0007)
//	    SolveTimeMs     float64      // NOT set by Solve; measured in the handler (P3/ADR-0007)
//	}
//	func Solve(g sudoku.Grid) SolveResult
//
// SolveTimeMs is deliberately handler-measured: ADR-0007 defines it as wall-clock and P3/AUDIT
// requires in-handler measurement, so Solve leaves it zero and the api handler times the call.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// AC-1 (EVAL §Eval matrix → UC-1): every one of the 25 seed puzzles solves to a grid that
// (a) satisfies all 27 row/col/box constraints and (b) equals the brute-force oracle's UNIQUE
// solution. The oracle also confirms exactly one solution exists (D-Q2).
func TestAC1_Solver_SolvesAllSeedPuzzlesToOracleSolution(t *testing.T) {
	puzzles := loadPuzzles(t)
	if len(puzzles) != 25 {
		t.Fatalf("expected 25 seed puzzles, got %d", len(puzzles))
	}
	for n, line := range puzzles {
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("puzzle %d: seed failed to parse: %v", n+1, err)
		}
		res := solver.Solve(grid)

		if res.Status != solver.StatusSolved || !res.Solved {
			t.Fatalf("puzzle %d: got status=%q solved=%v, want solved/true", n+1, res.Status, res.Solved)
		}
		if !constraints27Valid(res.Solution) {
			t.Fatalf("puzzle %d: solution violates the 27 constraints: %q", n+1, res.Solution)
		}
		if !matchesGivens(grid.String(), res.Solution) {
			t.Fatalf("puzzle %d: solution overwrites a given clue", n+1)
		}
		sols := bruteForce(parseBoard(grid.String()), 2)
		if len(sols) != 1 {
			t.Fatalf("puzzle %d: oracle found %d solutions, want exactly 1 (D-Q2)", n+1, len(sols))
		}
		if res.Solution != sols[0] {
			t.Fatalf("puzzle %d: solver != oracle\n solver=%q\n oracle=%q", n+1, res.Solution, sols[0])
		}
	}
}

// AC-3 (EVAL §Eval matrix → UC-2 — THE load-bearing test): replay the event log FROM THE
// ORIGINAL INPUT. Apply each Event to its pre-state, assert each recorded GridAfter, assert
// the final grid is byte-identical to the returned solution, and assert every filled cell was
// placed by a NAMED, WITNESSED technique that is mechanically forced given its pre-state —
// zero cells placed by anything else. Any unforced placement is a hidden guess and fails here.
func TestAC3_Solver_ReplayFromInputProvesNoBacktracking(t *testing.T) {
	for n, line := range loadPuzzles(t) {
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("puzzle %d: parse: %v", n+1, err)
		}
		res := solver.Solve(grid)
		if res.Status != solver.StatusSolved {
			t.Fatalf("puzzle %d: expected solved to run replay, got %q", n+1, res.Status)
		}

		board := parseBoard(grid.String())
		given := board // snapshot of the original givens
		placed := make(map[int]bool)
		wantSeq := 1

		for _, ev := range res.Events {
			if ev.Seq != wantSeq {
				t.Fatalf("puzzle %d: event seq gap: got %d want %d", n+1, ev.Seq, wantSeq)
			}
			wantSeq++
			if ev.Technique == "" || len(ev.WitnessCells) == 0 {
				t.Fatalf("puzzle %d seq %d: unnamed or unwitnessed step (a hidden guess)", n+1, ev.Seq)
			}

			if ev.Placement != nil {
				idx := ev.Placement.Cell.Row*9 + ev.Placement.Cell.Col
				val := ev.Placement.Value
				if idx < 0 || idx >= 81 {
					t.Fatalf("puzzle %d seq %d: placement cell out of range: %+v", n+1, ev.Seq, ev.Placement.Cell)
				}
				if board[idx] != 0 {
					t.Fatalf("puzzle %d seq %d: placement into non-empty cell %d", n+1, ev.Seq, idx)
				}
				if val < 1 || val > 9 || !legal(board, idx, val) {
					t.Fatalf("puzzle %d seq %d: illegal placement %d at cell %d", n+1, ev.Seq, val, idx)
				}
				// The named technique must actually FORCE this placement given the pre-state.
				switch ev.Technique {
				case "naked_single":
					if !nakedForced(board, idx, val) {
						t.Fatalf("puzzle %d seq %d: labeled naked_single but %d is not the sole candidate at cell %d (hidden guess)", n+1, ev.Seq, val, idx)
					}
				case "hidden_single":
					if !hiddenForced(board, idx, val) {
						t.Fatalf("puzzle %d seq %d: labeled hidden_single but %d is not a hidden single at cell %d (hidden guess)", n+1, ev.Seq, val, idx)
					}
				default:
					t.Fatalf("puzzle %d seq %d: unexpected technique %q for the singles tier (want naked_single|hidden_single)", n+1, ev.Seq, ev.Technique)
				}
				board[idx] = val
				placed[idx] = true
			}

			if got := boardString(board); got != ev.GridAfter {
				t.Fatalf("puzzle %d seq %d: gridAfter mismatch\n replay=%q\n event =%q", n+1, ev.Seq, got, ev.GridAfter)
			}
		}

		if got := boardString(board); got != res.Solution {
			t.Fatalf("puzzle %d: replayed grid != returned solution\n replay=%q\n sol   =%q", n+1, got, res.Solution)
		}
		for i := 0; i < 81; i++ {
			blankAtStart, filledAtEnd := given[i] == 0, board[i] != 0
			if blankAtStart && filledAtEnd && !placed[i] {
				t.Fatalf("puzzle %d: cell %d is filled but no placement event explains it (hidden guess)", n+1, i)
			}
			if !blankAtStart && placed[i] {
				t.Fatalf("puzzle %d: given cell %d was re-placed by an event", n+1, i)
			}
		}
	}
}

// AC-4 (ADR-0012): the same puzzle solved twice yields a byte-identical solution, event log,
// and counter metrics.
//
// SPEC RECONCILIATION (surfaced, not averaged): ADR-0012 says "identical input → byte-identical
// event log and metric quartet", but ADR-0007 defines solveTimeMs as WALL CLOCK, which is not
// reproducible run-to-run. The only coherent reading is that byte-identity covers the event log
// plus the three COUNTER metrics (eventCount, iterations, candidateChecks); solveTimeMs is
// excluded. Solve leaves solveTimeMs zero (the handler measures it), so this is consistent.
func TestAC4_Solver_DeterministicAcrossRepeatedSolves(t *testing.T) {
	for n, line := range loadPuzzles(t) {
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("puzzle %d: parse: %v", n+1, err)
		}
		a := solver.Solve(grid)
		b := solver.Solve(grid)

		if a.Status != b.Status || a.Solved != b.Solved {
			t.Fatalf("puzzle %d: status/solved not deterministic: %v/%v vs %v/%v", n+1, a.Status, a.Solved, b.Status, b.Solved)
		}
		if a.Solution != b.Solution {
			t.Fatalf("puzzle %d: solution not deterministic", n+1)
		}
		if a.EventCount != b.EventCount || a.Iterations != b.Iterations || a.CandidateChecks != b.CandidateChecks {
			t.Fatalf("puzzle %d: counter metrics not deterministic: {ec:%d it:%d cc:%d} vs {ec:%d it:%d cc:%d}",
				n+1, a.EventCount, a.Iterations, a.CandidateChecks, b.EventCount, b.Iterations, b.CandidateChecks)
		}
		ja, _ := json.Marshal(a.Events)
		jb, _ := json.Marshal(b.Events)
		if string(ja) != string(jb) {
			t.Fatalf("puzzle %d: event log not byte-identical across solves", n+1)
		}
	}
}

// AC-2 (ADR-0007, solver side): the counter metrics are populated and self-consistent. The
// in-handler solveTimeMs measurement is asserted at the api layer (see internal/api).
func TestAC2_Solver_CounterMetricsPopulated(t *testing.T) {
	grid, err := sudoku.Parse(loadPuzzles(t)[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res := solver.Solve(grid)
	if res.EventCount != len(res.Events) {
		t.Fatalf("eventCount %d != len(events) %d", res.EventCount, len(res.Events))
	}
	if res.EventCount == 0 {
		t.Fatalf("eventCount is zero on a solved puzzle")
	}
	if res.Iterations <= 0 {
		t.Fatalf("iterations must be > 0 (main-loop scan passes)")
	}
	if res.CandidateChecks <= 0 {
		t.Fatalf("candidateChecks must be > 0 (candidate-cell inspections)")
	}
}

// AC-5 stalled (ADR-0011): a valid grid the singles tier cannot finish returns "stalled" —
// not a guess, not unsolvable. Documented fixture: the EMPTY grid (81 zeros). It is valid (no
// givens to violate a unit), singles-INERT (every cell holds all 9 candidates → no naked
// single; every digit is a candidate in every cell of every unit → no hidden single), reaches
// no zero-candidate contradiction, and is incomplete & non-unique. Per ADR-0011 that is the
// textbook stalled outcome.
func TestAC5_Solver_StalledOnValidGridSinglesCannotFinish(t *testing.T) {
	blank := strings.Repeat("0", 81)
	grid, err := sudoku.Parse(blank)
	if err != nil {
		t.Fatalf("empty grid must parse (no givens to violate): %v", err)
	}
	// Precondition: no naked single anywhere (every empty cell has all 9 candidates).
	board := parseBoard(blank)
	for i := 0; i < 81; i++ {
		if got := len(candidateDigits(board, i)); got != 9 {
			t.Fatalf("empty-grid fixture precondition broken: cell %d has %d candidates, want 9", i, got)
		}
	}
	res := solver.Solve(grid)
	if res.Status != solver.StatusStalled {
		t.Fatalf("empty grid: got status=%q, want stalled", res.Status)
	}
	if res.Solved {
		t.Fatalf("empty grid: solved=true on a stalled grid")
	}
}

// AC-5 unsolvable (ADR-0011): a grid where the tier constructively drives a cell to ZERO
// candidates returns "unsolvable". Documented fixture: R0C0 is blank; row 0 carries 1..8
// (C1..C8) and column 0 carries 9 (R1C0). No given repeats within any row/col/box, so
// sudoku.Parse accepts it — yet R0C0 has zero candidates, an in-tier constructive contradiction.
func TestAC5_Solver_UnsolvableOnInTierZeroCandidate(t *testing.T) {
	fixture := "012345678" + "900000000" + strings.Repeat("000000000", 7)
	if len(fixture) != 81 {
		t.Fatalf("fixture length %d, want 81", len(fixture))
	}
	grid, err := sudoku.Parse(fixture)
	if err != nil {
		t.Fatalf("unsolvable fixture must be structurally legal (Parse accepts non-duplicate givens): %v", err)
	}
	// Precondition: R0C0 (index 0) has zero candidates.
	if cands := candidateDigits(parseBoard(fixture), 0); len(cands) != 0 {
		t.Fatalf("unsolvable fixture precondition broken: R0C0 candidates = %v, want none", cands)
	}
	res := solver.Solve(grid)
	if res.Status != solver.StatusUnsolvable {
		t.Fatalf("in-tier zero-candidate grid: got status=%q, want unsolvable", res.Status)
	}
	if res.Solved {
		t.Fatalf("unsolvable grid: solved=true")
	}
}
