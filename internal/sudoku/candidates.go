package sudoku

import "math/bits"

// Candidates is a per-cell candidate set for the digits 1..9, stored as a bitset: bit d
// (1 <= d <= 9) is set when digit d is still a candidate for the cell. Bit 0 is unused so
// a digit maps to its own bit index with no offset arithmetic at the call sites. P-0 only
// needs the type to exist and be sound; the solver (P-1) drives the deduction with it.
type Candidates uint16

// allDigits is the bitset with every digit 1..9 present (bits 1..9 set).
const allDigits Candidates = 0b0000_0011_1111_1110

// Full returns a candidate set containing every digit 1..9 — the starting state for a
// blank cell before any elimination.
func Full() Candidates { return allDigits }

// valid reports whether d is a real Sudoku digit. Methods treat an out-of-range digit as
// absent / a no-op rather than panicking, so the type stays sound under any input.
func valid(d uint8) bool { return d >= 1 && d <= 9 }

// Has reports whether digit d is a candidate.
func (c Candidates) Has(d uint8) bool {
	if !valid(d) {
		return false
	}
	return c&(1<<d) != 0
}

// Add returns c with digit d added as a candidate. An out-of-range digit is ignored.
func (c Candidates) Add(d uint8) Candidates {
	if !valid(d) {
		return c
	}
	return c | (1 << d)
}

// Remove returns c with digit d eliminated. An out-of-range digit is ignored.
func (c Candidates) Remove(d uint8) Candidates {
	if !valid(d) {
		return c
	}
	return c &^ (1 << d)
}

// Count returns the number of candidate digits remaining in the set.
func (c Candidates) Count() int {
	return bits.OnesCount16(uint16(c & allDigits))
}
