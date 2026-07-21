// Package solver is the constructive singles-tier Sudoku solver. It solves logic-only —
// naked single then hidden single, cheapest-first, scanned row-major — and NEVER guesses or
// backtracks (ADR-0001). Every placement it makes is mechanically forced by its named
// technique given the current candidates, and it emits a replayable, technique-tagged event
// log (EVAL UC-2) plus the frozen metric quartet (ADR-0007). Identical input yields a
// byte-identical event log and counter set (ADR-0012); the whole path is single-threaded
// with no shared mutable state (AUDIT P2).
package solver

import (
	"math/bits"

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

// Elimination records a single candidate removed from a cell. The singles tier only ever
// places, so P-1 emits no eliminations; the field exists for the frozen event shape.
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

// SolveResult is the full outcome of Solve: the status/solution, the event log, and the
// counter metrics. SolveTimeMs is deliberately left zero — the handler measures wall-clock
// around the Solve call (P3/ADR-0007), so it is excluded from the solver's deterministic set.
type SolveResult struct {
	Status          Status
	Solved          bool
	Solution        string
	Events          []Event
	EventCount      int
	Iterations      int
	CandidateChecks int
	SolveTimeMs     float64
}

// allDigits is the bitset with digits 1..9 set (bit d ⇔ digit d); bit 0 is unused.
const allDigits uint16 = 0b0000_0011_1111_1110

// units holds every row (0..8), then every column, then every box, each as its 9 cell
// indices. Fixed row-major order gives the hidden-single scan a deterministic tie-break
// (ADR-0012). Built once; read-only thereafter.
var units = buildUnits()

func buildUnits() [27][9]int {
	var u [27][9]int
	for i := 0; i < 9; i++ {
		for k := 0; k < 9; k++ {
			u[i][k] = i*9 + k         // row i
			u[9+i][k] = k*9 + i       // column i
			br, bc := (i/3)*3, (i%3)*3 // box i
			u[18+i][k] = (br+k/3)*9 + (bc + k%3)
		}
	}
	return u
}

// Solve runs the singles tier to fixpoint. Each main-loop pass (an Iteration) sweeps the
// ladder cheapest-first: it recomputes candidates from the current board, aborts to
// unsolvable if any empty cell has zero candidates, then applies the first naked single, or
// failing that the first hidden single, found in row-major order. A pass that places nothing
// on an incomplete grid is stalled; a full grid is solved.
func Solve(g sudoku.Grid) SolveResult {
	var b [81]uint8
	for i := 0; i < 81; i++ {
		b[i] = g.At(i)
	}

	var events []Event
	iterations, checks := 0, 0
	status := StatusStalled

	for {
		if full(&b) {
			status = StatusSolved
			break
		}
		iterations++

		cand, zero := computeCandidates(&b, &checks)
		if zero {
			status = StatusUnsolvable
			break
		}

		if ev, ok := applyNakedSingle(&b, &cand, len(events)); ok {
			events = append(events, ev)
			continue
		}
		if ev, ok := applyHiddenSingle(&b, &cand, len(events)); ok {
			events = append(events, ev)
			continue
		}

		status = StatusStalled
		break
	}

	var solution string
	if status == StatusSolved {
		solution = render(&b)
	}
	return SolveResult{
		Status:          status,
		Solved:          status == StatusSolved,
		Solution:        solution,
		Events:          events,
		EventCount:      len(events),
		Iterations:      iterations,
		CandidateChecks: checks,
	}
}

// computeCandidates returns the candidate bitset of every empty cell (bit d ⇔ digit d is
// legal there — absent from the cell's row, column, and box) and whether any empty cell has
// zero candidates (an in-tier constructive contradiction ⇒ unsolvable). Each empty-cell
// inspection is counted into checks (ADR-0007 candidateChecks).
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

// applyNakedSingle places the first empty cell (row-major) whose candidate set holds exactly
// one digit — that digit is forced, its own cell is the witness. Returns false if none fires.
func applyNakedSingle(b *[81]uint8, cand *[81]uint16, seq int) (Event, bool) {
	for i := 0; i < 81; i++ {
		if b[i] != 0 {
			continue
		}
		if bits.OnesCount16(cand[i]) == 1 {
			val := bits.TrailingZeros16(cand[i])
			cell := Cell{Row: i / 9, Col: i % 9}
			return place(b, cell, val, "naked_single", []Cell{cell}, seq), true
		}
	}
	return Event{}, false
}

// applyHiddenSingle places the first (unit, digit) in row-major unit order — rows, then
// columns, then boxes, digits ascending — for which exactly one empty cell in the unit can
// legally take the digit. That cell is the placement; its own cell is the witness. Returns
// false if none fires.
func applyHiddenSingle(b *[81]uint8, cand *[81]uint16, seq int) (Event, bool) {
	for u := range units {
		for d := 1; d <= 9; d++ {
			bit := uint16(1) << d
			only, count := -1, 0
			for _, idx := range units[u] {
				if b[idx] == 0 && cand[idx]&bit != 0 {
					only, count = idx, count+1
				}
			}
			if count == 1 {
				cell := Cell{Row: only / 9, Col: only % 9}
				return place(b, cell, d, "hidden_single", []Cell{cell}, seq), true
			}
		}
	}
	return Event{}, false
}

// place mutates b with the forced digit and returns the event describing the step, including
// the canonical grid AFTER the placement.
func place(b *[81]uint8, cell Cell, val int, technique string, witness []Cell, seq int) Event {
	b[cell.Row*9+cell.Col] = uint8(val)
	return Event{
		Seq:          seq + 1,
		Technique:    technique,
		WitnessCells: witness,
		Placement:    &Placement{Cell: cell, Value: val},
		GridAfter:    render(b),
	}
}

// full reports whether every cell is filled.
func full(b *[81]uint8) bool {
	for _, v := range b {
		if v == 0 {
			return false
		}
	}
	return true
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
