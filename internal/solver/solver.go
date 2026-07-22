// Package solver is the constructive Sudoku solver. It solves logic-only — walking the
// ADR-0002 technique ladder cheapest-first, scanning row-major — and NEVER guesses or
// backtracks (ADR-0001). Singles PLACE a forced digit; every advanced technique (ladder
// index >= 2) only ELIMINATES candidates that provably cannot be the solution, after which a
// single may fire. Each step is mechanically forced by its named technique given the current
// candidates, and the solver emits a replayable, technique-tagged event log (EVAL UC-2) plus
// the frozen metric quartet (ADR-0007). Identical input yields a byte-identical event log and
// counter set (ADR-0012); the whole path is single-threaded with no shared mutable state.
package solver

import (
	"math/bits"
	"slices"

	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// Status is the terminal outcome of a solve (ADR-0011). invalid_input is decided upstream at
// sudoku.Parse and never originates here; Solve only ever returns solved/stalled/unsolvable.
type Status string

const (
	StatusSolved       Status = "solved"
	StatusStalled      Status = "stalled"
	StatusUnsolvable   Status = "unsolvable"
	StatusInvalidInput Status = "invalid_input"
)

// Cell is a 0-based row/column coordinate.
type Cell struct {
	Row, Col int
}

// Placement records a digit forced into a cell by a technique.
type Placement struct {
	Cell  Cell
	Value int
}

// Elimination records a single candidate removed from a cell by an advanced technique.
type Elimination struct {
	Cell      Cell
	Candidate int
}

// Event is one replayable step: the technique that fired, the witnessing cell(s), its effect
// (a placement, or eliminations), and the canonical grid AFTER the step. Replaying the log
// from the original input reproduces each GridAfter and the final solution (the mechanical
// proof that no step was a hidden guess).
type Event struct {
	Seq          int
	Technique    string
	WitnessCells []Cell
	Placement    *Placement
	Eliminations []Elimination
	GridAfter    string
}

// SolveResult is the full outcome of Solve: the status/solution, the event log, the counter
// metrics, and the hardest technique the solve was forced to use. SolveTimeMs is deliberately
// left zero — the handler measures wall-clock around the Solve call (P3/ADR-0007), so it is
// excluded from the solver's deterministic set.
type SolveResult struct {
	Status           Status
	Solved           bool
	Solution         string
	Events           []Event
	EventCount       int
	Iterations       int
	CandidateChecks  int
	SolveTimeMs      float64
	HardestTechnique Technique
}

// allDigits is the bitset with digits 1..9 set (bit d ⇔ digit d); bit 0 is unused.
const allDigits uint16 = 0b0000_0011_1111_1110

// units holds every row (0..8), then every column, then every box, each as its 9 cell
// indices. Fixed row-major order gives every scan a deterministic tie-break (ADR-0012).
// Built once; read-only thereafter.
var units = buildUnits()

func buildUnits() [27][9]int {
	var u [27][9]int
	for i := 0; i < 9; i++ {
		for k := 0; k < 9; k++ {
			u[i][k] = i*9 + k          // row i
			u[9+i][k] = k*9 + i        // column i
			br, bc := (i/3)*3, (i%3)*3 // box i
			u[18+i][k] = (br+k/3)*9 + (bc + k%3)
		}
	}
	return u
}

// engine carries the mutable state of one solve: the board, the cumulative technique
// eliminations, the derived candidate set for the current pass, the event log, and the
// counter metrics. The candidate model is deliberately reconstructed each pass as
// (basic candidates of the current board) &^ (recorded eliminations) so it stays byte-for-byte
// identical to the replay model the tests rebuild — placements are singles under exactly that
// set, and every advanced technique reads the reduced set including prior eliminations.
type engine struct {
	board      [81]uint8
	elim       [81]uint16 // cumulative advanced-technique eliminations (bit d)
	cand       [81]uint16 // basic candidates of board &^ elim, recomputed each pass
	events     []Event
	iterations int
	checks     int
	hardestIdx int
}

// runEngine drives the solve loop with the ladder enabled up to (and including) maxIdx.
// Each pass recomputes the candidate model via compute (the sequential computeCandidates for
// Solve, the concurrent computeCandidatesParallel for SolveParallel — the ONLY difference
// between the two entry points), aborts to unsolvable on a zero-candidate cell, then applies
// the FIRST enabled technique (cheapest-first) that makes a productive step — a single
// placement, or an advanced elimination of at least one live candidate. A pass that makes no
// progress on an incomplete grid is stalled; a full grid is solved. compute must be a pure,
// byte-identical candidate derivation so the event log and counters stay deterministic
// (ADR-0012) regardless of which strategy is passed.
func runEngine(g sudoku.Grid, compute func(b *[81]uint8, checks *int) (cand [81]uint16, zero bool), maxIdx int) SolveResult {
	e := &engine{hardestIdx: -1}
	for i := 0; i < 81; i++ {
		e.board[i] = g.At(i)
	}

	status := StatusStalled
	for {
		if !slices.Contains(e.board[:], 0) {
			status = StatusSolved
			break
		}
		e.iterations++

		basic, _ := compute(&e.board, &e.checks)
		zero := false
		for i := 0; i < 81; i++ {
			if e.board[i] != 0 {
				e.cand[i] = 0
				continue
			}
			e.cand[i] = basic[i] &^ e.elim[i]
			if e.cand[i] == 0 {
				zero = true
			}
		}
		if zero {
			status = StatusUnsolvable
			break
		}

		progressed := false
		for ti := 0; ti <= maxIdx; ti++ {
			if ev, ok := ladderTechniques[ti].run(e); ok {
				e.events = append(e.events, ev)
				if ti > e.hardestIdx {
					e.hardestIdx = ti
				}
				progressed = true
				break
			}
		}
		if !progressed {
			status = StatusStalled
			break
		}
	}

	var solution string
	if status == StatusSolved {
		solution = render(&e.board)
	}
	var hardest Technique
	if e.hardestIdx >= 0 {
		hardest = ladderTechniques[e.hardestIdx].name
	}
	return SolveResult{
		Status:           status,
		Solved:           status == StatusSolved,
		Solution:         solution,
		Events:           e.events,
		EventCount:       len(e.events),
		Iterations:       e.iterations,
		CandidateChecks:  e.checks,
		HardestTechnique: hardest,
	}
}

// Solve runs the full ladder to fixpoint (ADR-0002). It is the maximal SolveWithMaxTechnique.
func Solve(g sudoku.Grid) SolveResult {
	return runEngine(g, computeCandidates, len(ladderTechniques)-1)
}

// computeCandidates returns the basic candidate bitset of every empty cell (bit d ⇔ digit d is
// legal there — absent from the cell's row, column, and box) and whether any empty cell has
// zero basic candidates. Each empty-cell inspection is counted into checks (ADR-0007
// candidateChecks). Advanced-technique eliminations are layered on top by the caller.
func computeCandidates(b *[81]uint8, checks *int) (cand [81]uint16, zero bool) {
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
	for i := 0; i < 81; i++ {
		if b[i] != 0 {
			continue
		}
		r, c := i/9, i%9
		cand[i] = allDigits &^ (rows[r] | cols[c] | boxes[(r/3)*3+c/3])
		*checks++
		if cand[i] == 0 {
			zero = true
		}
	}
	return cand, zero
}

// --- singles (ladder index 0, 1) — the only techniques that PLACE -------------------------

// nakedSingle places the first empty cell (row-major) whose candidate set holds exactly one
// digit — that digit is forced, its own cell is the witness.
func nakedSingle(e *engine) (Event, bool) {
	for i := 0; i < 81; i++ {
		if e.board[i] != 0 {
			continue
		}
		if bits.OnesCount16(e.cand[i]) == 1 {
			val := bits.TrailingZeros16(e.cand[i])
			cell := Cell{Row: i / 9, Col: i % 9}
			return e.placeEvent(cell, val, "naked_single", []Cell{cell}), true
		}
	}
	return Event{}, false
}

// hiddenSingle places the first (unit, digit) in row-major unit order — rows, then columns,
// then boxes, digits ascending — for which exactly one empty cell in the unit can take the
// digit. That cell is the placement; its own cell is the witness.
func hiddenSingle(e *engine) (Event, bool) {
	for u := range units {
		for d := 1; d <= 9; d++ {
			bit := uint16(1) << d
			only, count := -1, 0
			for _, idx := range units[u] {
				if e.board[idx] == 0 && e.cand[idx]&bit != 0 {
					only, count = idx, count+1
				}
			}
			if count == 1 {
				cell := Cell{Row: only / 9, Col: only % 9}
				return e.placeEvent(cell, d, "hidden_single", []Cell{cell}), true
			}
		}
	}
	return Event{}, false
}

// --- engine step helpers ------------------------------------------------------------------

// placeEvent mutates the board with a forced digit and returns the placement event, including
// the canonical grid AFTER the placement.
func (e *engine) placeEvent(cell Cell, val int, technique string, witness []Cell) Event {
	e.board[cell.Row*9+cell.Col] = uint8(val)
	return Event{
		Seq:          len(e.events) + 1,
		Technique:    technique,
		WitnessCells: witness,
		Placement:    &Placement{Cell: cell, Value: val},
		GridAfter:    render(&e.board),
	}
}

// elimEvent records the technique's eliminations into the cumulative set and returns the
// elimination event. The board is unchanged, so GridAfter is the current grid — a later single
// converts the reduced candidates into a placement.
func (e *engine) elimEvent(technique string, witness []Cell, elims []Elimination) Event {
	for _, el := range elims {
		e.elim[el.Cell.Row*9+el.Cell.Col] |= uint16(1) << el.Candidate
	}
	return Event{
		Seq:          len(e.events) + 1,
		Technique:    technique,
		WitnessCells: witness,
		Eliminations: elims,
		GridAfter:    render(&e.board),
	}
}

// render produces the canonical 81-char grid ('0' = blank), matching sudoku.Grid.String() so
// GridAfter compares byte-for-byte against a replay.
func render(b *[81]uint8) string {
	var out [81]byte
	for i, v := range b {
		out[i] = '0' + v
	}
	return string(out[:])
}
