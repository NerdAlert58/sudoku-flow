package solver_test

// P-1 hidden_single coverage backfill (test-only). The 25 seed puzzles (D-Q3) all solve by
// naked singles alone, so applyHiddenSingle (solver.go:198) and the AC-3 replay's
// `case "hidden_single"` arm never execute against the seed corpus. These tests construct
// grids that FORCE a hidden single and route it through the SAME no-backtracking replay proof
// TestAC3 uses (reusing the leaf helpers nakedForced/hiddenForced/parseBoard/boardString/legal
// from oracle_test.go), so the hidden-single arm is exercised. The solver is already correct;
// these are pure coverage — an expected PASS.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// hiddenSingleFixture builds an 81-char grid whose ONLY givens are eight 5's placed one per
// row/col/box so that, in row 0, digit 5 is legal in EXACTLY one cell (R0C0) — while R0C0
// still holds every candidate (no given touches row 0, column 0, or box 0). That makes R0C0=5
// a HIDDEN single (unit-forced) and NOT a naked single (it has >1 candidate). Because the only
// givens are 5's, every empty cell has >= 8 candidates, so no naked single exists anywhere and
// the FIRST technique the solver can fire is the hidden single.
//
// The eight givens sit at (row,col): (3,1) (6,2) (1,3) (4,4) (7,5) (2,6) (5,7) (8,8) — rows,
// columns, and boxes all distinct, so no 5 repeats in any unit and sudoku.Parse accepts it.
func hiddenSingleFixture() string {
	var b [81]byte
	for i := range b {
		b[i] = '0'
	}
	for _, rc := range [][2]int{{3, 1}, {6, 2}, {1, 3}, {4, 4}, {7, 5}, {2, 6}, {5, 7}, {8, 8}} {
		b[rc[0]*9+rc[1]] = '5'
	}
	return string(b[:])
}

// replayProvesForced replays res.Events from `input` and asserts the SAME no-backtracking
// invariants TestAC3 asserts: contiguous seqs, every step named + witnessed, every placement
// mechanically FORCED by its named technique given its pre-state (naked_single -> nakedForced,
// hidden_single -> hiddenForced AND not nakedForced, since the solver fires naked first so a
// hidden step is never also a naked single), each GridAfter equal to the replay, a terminal
// grid equal to res.Solution when solved (else to the last event's GridAfter), and no filled
// cell left unexplained. Unlike TestAC3 it accepts a stalled result, so a stalled grid that
// still made a hidden-single placement can be routed through the identical proof. Returns the
// number of naked and hidden placements it verified.
func replayProvesForced(t *testing.T, input string, res solver.SolveResult) (naked, hidden int) {
	t.Helper()
	board := parseBoard(input)
	given := board
	placed := make(map[int]bool)
	wantSeq := 1

	for _, ev := range res.Events {
		if ev.Seq != wantSeq {
			t.Fatalf("event seq gap: got %d want %d", ev.Seq, wantSeq)
		}
		wantSeq++
		if ev.Technique == "" || len(ev.WitnessCells) == 0 {
			t.Fatalf("seq %d: unnamed or unwitnessed step (a hidden guess)", ev.Seq)
		}

		if ev.Placement != nil {
			idx := ev.Placement.Cell.Row*9 + ev.Placement.Cell.Col
			val := ev.Placement.Value
			if idx < 0 || idx >= 81 {
				t.Fatalf("seq %d: placement cell out of range: %+v", ev.Seq, ev.Placement.Cell)
			}
			if board[idx] != 0 {
				t.Fatalf("seq %d: placement into non-empty cell %d", ev.Seq, idx)
			}
			if val < 1 || val > 9 || !legal(board, idx, val) {
				t.Fatalf("seq %d: illegal placement %d at cell %d", ev.Seq, val, idx)
			}
			switch ev.Technique {
			case "naked_single":
				if !nakedForced(board, idx, val) {
					t.Fatalf("seq %d: labeled naked_single but %d is not the sole candidate at cell %d (hidden guess)", ev.Seq, val, idx)
				}
				naked++
			case "hidden_single":
				// This is the arm the 25 naked-single-only seeds never reach.
				if !hiddenForced(board, idx, val) {
					t.Fatalf("seq %d: labeled hidden_single but %d is not a hidden single at cell %d (hidden guess)", ev.Seq, val, idx)
				}
				// Genuinely HIDDEN, not a mislabeled naked single: the solver applies naked
				// singles first, so any hidden step must have >1 candidate at its cell.
				if nakedForced(board, idx, val) {
					t.Fatalf("seq %d: labeled hidden_single but %d is a NAKED single at cell %d (not genuinely hidden)", ev.Seq, val, idx)
				}
				hidden++
			default:
				t.Fatalf("seq %d: unexpected technique %q for the singles tier (want naked_single|hidden_single)", ev.Seq, ev.Technique)
			}
			board[idx] = val
			placed[idx] = true
		}

		if got := boardString(board); got != ev.GridAfter {
			t.Fatalf("seq %d: gridAfter mismatch\n replay=%q\n event =%q", ev.Seq, got, ev.GridAfter)
		}
	}

	if res.Status == solver.StatusSolved {
		if got := boardString(board); got != res.Solution {
			t.Fatalf("replayed grid != returned solution\n replay=%q\n sol   =%q", got, res.Solution)
		}
	} else if n := len(res.Events); n > 0 {
		if got := boardString(board); got != res.Events[n-1].GridAfter {
			t.Fatalf("replayed grid != last event GridAfter\n replay=%q\n last  =%q", got, res.Events[n-1].GridAfter)
		}
	}

	for i := 0; i < 81; i++ {
		blankAtStart, filledAtEnd := given[i] == 0, board[i] != 0
		if blankAtStart && filledAtEnd && !placed[i] {
			t.Fatalf("cell %d is filled but no placement event explains it (hidden guess)", i)
		}
		if !blankAtStart && placed[i] {
			t.Fatalf("given cell %d was re-placed by an event", i)
		}
	}
	return naked, hidden
}

// P-1 BLOCKING FIX (AUDIT §D-Q3): a grid that FORCES a hidden single, routed through the AC-3
// replay proof so the `hidden_single` arm actually executes. The construction, its pre-state
// hidden-vs-naked properties, and the fired event are all asserted explicitly.
func TestP1_HiddenSingle_ForcedAndRoutedThroughReplay(t *testing.T) {
	input := hiddenSingleFixture()

	// (1) Parses: no 5 repeats in any row/col/box.
	grid, err := sudoku.Parse(input)
	if err != nil {
		t.Fatalf("hidden-single fixture must parse (no duplicate given in any unit): %v", err)
	}

	// Precondition: the only givens are 5's, so NO empty cell has a single candidate — the
	// first technique the solver can fire is therefore NOT a naked single.
	b0 := parseBoard(input)
	for i := 0; i < 81; i++ {
		if b0[i] == 0 && len(candidateDigits(b0, i)) < 2 {
			t.Fatalf("fixture precondition broken: empty cell %d has <2 candidates (a naked single)", i)
		}
	}
	// Precondition (the crux): R0C0 is a HIDDEN single for 5 (only legal cell for 5 in row 0)
	// yet NOT a naked single (it still has multiple candidates). This is exactly "the digit is
	// legal in exactly one cell of some unit, but that cell has >1 candidate".
	if !hiddenForced(b0, 0, 5) {
		t.Fatalf("fixture precondition broken: R0C0=5 is not hidden-forced")
	}
	if nakedForced(b0, 0, 5) {
		t.Fatalf("fixture precondition broken: R0C0=5 is a naked single, not a hidden single")
	}
	if got := len(candidateDigits(b0, 0)); got < 2 {
		t.Fatalf("fixture precondition broken: R0C0 has %d candidates, want >1 (genuinely hidden)", got)
	}

	res := solver.Solve(grid)

	// (2) The event log contains >= 1 genuinely hidden-forced placement.
	var hs *solver.Event
	for i := range res.Events {
		if res.Events[i].Technique == "hidden_single" {
			hs = &res.Events[i]
			break
		}
	}
	if hs == nil {
		t.Fatalf("no hidden_single event fired; the hidden-single arm was not exercised. events=%+v", res.Events)
	}
	if hs.Placement == nil {
		t.Fatalf("hidden_single event %d carries no placement", hs.Seq)
	}

	// (3) Route the whole log through the AC-3 no-backtracking replay proof. This is what
	// executes the `case "hidden_single"` arm (hiddenForced) against a hidden single.
	naked, hidden := replayProvesForced(t, input, res)
	if hidden < 1 {
		t.Fatalf("replay verified %d hidden_single placements, want >= 1", hidden)
	}
	_ = naked
}

// (4) The hidden-single fixture is deterministic across two solves (status, counters, and a
// byte-identical event log). It STALLS after the hidden placement (the only givens are 5's, so
// once the 5-layer completes nothing else is forced), so "solved" is not assertable here —
// determinism is (ADR-0012 covers the event log + counter trio, not wall-clock solveTimeMs).
func TestP1_HiddenSingle_Deterministic(t *testing.T) {
	grid, err := sudoku.Parse(hiddenSingleFixture())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := solver.Solve(grid)
	b := solver.Solve(grid)

	if a.Status != b.Status || a.Solved != b.Solved {
		t.Fatalf("status/solved not deterministic: %v/%v vs %v/%v", a.Status, a.Solved, b.Status, b.Solved)
	}
	if a.EventCount != b.EventCount || a.Iterations != b.Iterations || a.CandidateChecks != b.CandidateChecks {
		t.Fatalf("counter metrics not deterministic")
	}
	ja, _ := json.Marshal(a.Events)
	jb, _ := json.Marshal(b.Events)
	if string(ja) != string(jb) {
		t.Fatalf("event log not byte-identical across solves")
	}
}

// AC-5 stalled AFTER progress (ADR-0011), jasnah non-blocking note: distinct from the
// degenerate empty-grid stalled fixture (which places nothing, eventCount 0). Here row 0 holds
// givens 1..8 in columns 0..7, leaving R0C8 forced to 9 (a NAKED single). The solver makes
// that one productive placement, then no single fires on the otherwise-empty grid, so it
// stalls with eventCount > 0. Not solved, not unsolvable.
func TestP1_StalledAfterProductivePlacement(t *testing.T) {
	input := "123456780" + strings.Repeat("0", 72) // row 0 = 1..8 then blank; rest empty
	if len(input) != 81 {
		t.Fatalf("fixture length %d, want 81", len(input))
	}
	grid, err := sudoku.Parse(input)
	if err != nil {
		t.Fatalf("stalled-after-progress fixture must parse: %v", err)
	}
	// Precondition: R0C8 (index 8) is a naked single for 9 (row 0 already carries 1..8).
	if !nakedForced(parseBoard(input), 8, 9) {
		t.Fatalf("fixture precondition broken: R0C8=9 is not a naked single")
	}

	res := solver.Solve(grid)

	if res.Status != solver.StatusStalled {
		t.Fatalf("got status=%q, want stalled", res.Status)
	}
	if res.Solved {
		t.Fatalf("solved=true on a stalled grid")
	}
	if res.EventCount < 1 {
		t.Fatalf("eventCount=%d, want >= 1 (stalled AFTER a productive placement, distinct from the empty-grid stalled fixture)", res.EventCount)
	}
	// The productive placement(s) are still fully forced and replayable.
	if naked, hidden := replayProvesForced(t, input, res); naked+hidden < 1 {
		t.Fatalf("replay verified %d forced placements, want >= 1", naked+hidden)
	}
}
