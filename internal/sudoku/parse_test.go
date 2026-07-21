package sudoku_test

// Tests for the trust-boundary parser (ARCHITECTURE.md §Contracts → Grid/Candidates;
// AUDIT §S1, §D-Q1; DESIGN_DECISIONS ADR-0011). These encode P-0 acceptance criteria
// AC-3 (valid parse) and AC-4 (typed errors, never panics).
//
// Test-defined source surface the builder implements to:
//   sudoku.Parse(string) (sudoku.Grid, error)
//   sudoku.Grid.String() string  -- canonical 81-char, '0' for blanks (D-Q1 canonical form)
//   sentinel errors: sudoku.ErrInvalidLength, sudoku.ErrInvalidCharacter, sudoku.ErrDuplicateValue

import (
	"errors"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// A real, well-formed, uniquely-solvable seed puzzle (puzzles.txt line 1). Uses '0' for
// blanks per D-Q1. All 25 seed lines are exactly 81 chars with no rule-violating givens.
const validPuzzle = "700605000000000030509300024002000000401907052000501000004050000310492000007003000"

// withDigit returns an 81-char all-blank ('0') grid with the given (index, digit) pairs set.
func withDigit(pairs ...int) string {
	b := []byte(strings.Repeat("0", 81))
	for i := 0; i+1 < len(pairs); i += 2 {
		b[pairs[i]] = byte('0' + pairs[i+1])
	}
	return string(b)
}

// withByte returns an 81-char all-blank ('0') grid with a single raw byte set at idx.
func withByte(idx int, c byte) string {
	b := []byte(strings.Repeat("0", 81))
	b[idx] = c
	return string(b)
}

// AC-3: Parse accepts a valid 81-char string using '0' for blanks and returns a populated
// Grid; '.' is accepted as an alias for '0'.
func TestParse_ValidZeroBlanks_ReturnsPopulatedGrid(t *testing.T) {
	if len(validPuzzle) != 81 {
		t.Fatalf("test-data error: validPuzzle is %d chars, want 81", len(validPuzzle))
	}

	g, err := sudoku.Parse(validPuzzle)
	if err != nil {
		t.Fatalf("Parse(valid 81-char '0'-blank puzzle) returned error: %v", err)
	}
	// A populated Grid round-trips to its canonical 81-char '0'-blank form, proving every
	// given landed in the right cell.
	if got := g.String(); got != validPuzzle {
		t.Fatalf("Grid.String() = %q, want %q", got, validPuzzle)
	}
}

// AC-3: '.' is accepted as an alias for '0' and yields the identical Grid.
func TestParse_DotAliasEqualsZeroForm(t *testing.T) {
	dotForm := strings.ReplaceAll(validPuzzle, "0", ".")

	gZero, err := sudoku.Parse(validPuzzle)
	if err != nil {
		t.Fatalf("Parse('0'-blank form) returned error: %v", err)
	}
	gDot, err := sudoku.Parse(dotForm)
	if err != nil {
		t.Fatalf("Parse('.'-alias form) returned error: %v", err)
	}
	if gZero.String() != gDot.String() {
		t.Fatalf("'.' alias not treated as '0': zero-form=%q dot-form=%q",
			gZero.String(), gDot.String())
	}
}

// AC-3: an all-blank 81-char grid (no givens) is valid input and returns without error.
func TestParse_AllBlank_IsValid(t *testing.T) {
	allBlank := strings.Repeat("0", 81)
	g, err := sudoku.Parse(allBlank)
	if err != nil {
		t.Fatalf("Parse(all-blank grid) returned error: %v", err)
	}
	if got := g.String(); got != allBlank {
		t.Fatalf("Grid.String() = %q, want %q", got, allBlank)
	}
}

// AC-4: length != 81 returns a typed error (sudoku.ErrInvalidLength).
func TestParse_InvalidLength_TypedError(t *testing.T) {
	cases := map[string]string{
		"empty":    "",
		"tooShort": strings.Repeat("0", 80),
		"tooLong":  strings.Repeat("0", 82),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := sudoku.Parse(in)
			if err == nil {
				t.Fatalf("Parse(len=%d) returned nil error, want ErrInvalidLength", len(in))
			}
			if !errors.Is(err, sudoku.ErrInvalidLength) {
				t.Fatalf("Parse(len=%d) error = %v, want errors.Is ErrInvalidLength", len(in), err)
			}
		})
	}
}

// AC-4: an illegal character (length is exactly 81) returns sudoku.ErrInvalidCharacter.
func TestParse_IllegalCharacter_TypedError(t *testing.T) {
	cases := map[string]string{
		"letter":     withByte(0, 'x'),
		"upper":      withByte(40, 'A'),
		"slash":      withByte(80, '/'),
		"space":      withByte(10, ' '),
		"underscore": withByte(5, '_'),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if len(in) != 81 {
				t.Fatalf("test-data error: input is %d chars, want 81", len(in))
			}
			_, err := sudoku.Parse(in)
			if err == nil {
				t.Fatalf("Parse(illegal char) returned nil error, want ErrInvalidCharacter")
			}
			if !errors.Is(err, sudoku.ErrInvalidCharacter) {
				t.Fatalf("Parse(illegal char) error = %v, want errors.Is ErrInvalidCharacter", err)
			}
		})
	}
}

// AC-4: a duplicate digit among the givens in a row, column, or box returns
// sudoku.ErrDuplicateValue. Each case isolates one unit type:
//
//	row  : indices 0 (r0c0) and 3 (r0c3)  -- same row, different box
//	col  : indices 0 (r0c0) and 27 (r3c0) -- same column, different box
//	box  : indices 0 (r0c0) and 10 (r1c1) -- same box, different row & column
func TestParse_DuplicateGiven_TypedError(t *testing.T) {
	cases := map[string]string{
		"row":    withDigit(0, 1, 3, 1),
		"column": withDigit(0, 1, 27, 1),
		"box":    withDigit(0, 1, 10, 1),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if len(in) != 81 {
				t.Fatalf("test-data error: input is %d chars, want 81", len(in))
			}
			_, err := sudoku.Parse(in)
			if err == nil {
				t.Fatalf("Parse(duplicate given in %s) returned nil error, want ErrDuplicateValue", name)
			}
			if !errors.Is(err, sudoku.ErrDuplicateValue) {
				t.Fatalf("Parse(duplicate given in %s) error = %v, want errors.Is ErrDuplicateValue", name, err)
			}
		})
	}
}

// AC-4: Parse never panics on user input. Feeds a wide range of malformed and adversarial
// inputs; a panic here fails the test. Valid inputs must not error, malformed must error.
func TestParse_NeverPanicsOnUserInput(t *testing.T) {
	valid := map[string]bool{
		validPuzzle:             true, // valid givens
		strings.Repeat("0", 81): true, // valid all-blank
		strings.Repeat(".", 81): true, // valid all-blank via alias
	}
	inputs := []string{
		"",
		"0",
		strings.Repeat("0", 80),
		strings.Repeat("0", 82),
		strings.Repeat("0", 1000),
		"abcdefghij" + strings.Repeat("0", 71),
		withByte(0, '\n'),
		withByte(0, '\t'),
		"日本語" + strings.Repeat("0", 78), // multibyte runes
		validPuzzle,
		strings.Repeat("0", 81),
		strings.Repeat(".", 81),
		withDigit(0, 1, 3, 1), // duplicate
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(%q...) panicked: %v", firstRunes(in, 12), r)
				}
			}()
			_, err := sudoku.Parse(in)
			if valid[in] && err != nil {
				t.Fatalf("Parse(valid input) errored: %v", err)
			}
			if !valid[in] && err == nil {
				t.Fatalf("Parse(malformed input %q...) returned nil error", firstRunes(in, 12))
			}
		}()
	}
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
