package sudoku

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for the parse trust boundary. Callers classify with errors.Is; the
// wrapped detail (index, character, unit) is for logs and the {error, code} envelope,
// never for control flow. Row, column, and box duplicate givens all map to
// ErrDuplicateValue (ADR-0011: these are one class of malformed input — a rule-violating
// given — surfaced upstream of the solver as invalid_input).
var (
	// ErrInvalidLength is returned when the puzzle is not exactly 81 characters.
	ErrInvalidLength = errors.New("sudoku: puzzle must be exactly 81 characters")
	// ErrInvalidCharacter is returned for any character other than '0'..'9' or '.'.
	ErrInvalidCharacter = errors.New("sudoku: puzzle contains an invalid character")
	// ErrDuplicateValue is returned when a given digit repeats within a row, column, or box.
	ErrDuplicateValue = errors.New("sudoku: duplicate given digit in a row, column, or box")
)

// Parse allowlist-validates an 81-character puzzle string and returns a populated Grid.
// Blanks are '0' or '.' (an accepted alias); filled cells are '1'..'9'. A trailing CRLF
// or LF is tolerated so a line read from a puzzles.txt source (D-Q1: CRLF line endings)
// parses without a caller-side trim; interior whitespace is not stripped and is rejected
// as an invalid character.
//
// Parse never panics on user input. Every malformed case returns a typed error:
//   - length != 81                              -> ErrInvalidLength
//   - a character outside '0'..'9' / '.'        -> ErrInvalidCharacter
//   - a given digit repeated in a row/col/box   -> ErrDuplicateValue
func Parse(s string) (Grid, error) {
	s = strings.TrimRight(s, "\r\n")

	if len(s) != Cells {
		return Grid{}, fmt.Errorf("%w: got %d", ErrInvalidLength, len(s))
	}

	var g Grid
	for i := 0; i < Cells; i++ {
		c := s[i]
		switch {
		case c == '0' || c == '.':
			g.cells[i] = 0
		case c >= '1' && c <= '9':
			g.cells[i] = c - '0'
		default:
			return Grid{}, fmt.Errorf("%w: byte %#x at index %d", ErrInvalidCharacter, c, i)
		}
	}

	if err := validateGivens(g); err != nil {
		return Grid{}, err
	}
	return g, nil
}

// validateGivens rejects a grid whose givens violate the one-per-unit rule. It walks each
// cell once, tracking seen digits per row, column, and box as 9-bit masks — O(81), no
// allocation. The first violating unit wins; the wrapped message names which unit.
func validateGivens(g Grid) error {
	var rows, cols, boxes [9]uint16
	for i := 0; i < Cells; i++ {
		v := g.cells[i]
		if v == 0 {
			continue // blanks are not givens
		}
		bit := uint16(1) << v
		r, c := i/9, i%9
		b := (r/3)*3 + c/3

		switch {
		case rows[r]&bit != 0:
			return fmt.Errorf("%w: digit %d twice in row %d", ErrDuplicateValue, v, r)
		case cols[c]&bit != 0:
			return fmt.Errorf("%w: digit %d twice in column %d", ErrDuplicateValue, v, c)
		case boxes[b]&bit != 0:
			return fmt.Errorf("%w: digit %d twice in box %d", ErrDuplicateValue, v, b)
		}
		rows[r] |= bit
		cols[c] |= bit
		boxes[b] |= bit
	}
	return nil
}
