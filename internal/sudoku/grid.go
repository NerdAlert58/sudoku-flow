// Package sudoku owns the grid model and the parse trust boundary. Exactly one
// untrusted input crosses into this system — the 81-character puzzle string — and it
// is allowlist-validated here (ARCHITECTURE.md §Summary, §Contracts → Grid/Candidates)
// before any solver code runs.
package sudoku

// Cells is the number of cells in a standard 9x9 Sudoku grid.
const Cells = 81

// Grid is a 9x9 Sudoku grid stored row-major as 81 cell values. A cell holds 0 for a
// blank or 1..9 for a filled digit. The field is unexported so the only way to obtain a
// Grid whose values are known-legal is through Parse — the invariant lives at the trust
// boundary, not in every caller.
type Grid struct {
	cells [Cells]uint8
}

// At returns the value (0=blank, 1..9) at row-major index i (0..80). It panics on an
// out-of-range index, which is a programmer error, never user input — Parse never
// produces an out-of-range access.
func (g Grid) At(i int) uint8 {
	return g.cells[i]
}

// String renders the grid in its canonical 81-character form using '0' for blanks (the
// D-Q1 canonical form). A grid parsed from a '.'-blank input and the equivalent
// '0'-blank input therefore produce identical String output.
func (g Grid) String() string {
	b := make([]byte, Cells)
	for i, v := range g.cells {
		b[i] = '0' + v
	}
	return string(b)
}
