package generator

import "math/bits"

// solutionCount is the generator's backtracking uniqueness counter (ADR-0003 / ADR-0004): a
// plain depth-first search with MRV (minimum-remaining-values) cell selection that counts up to
// cap solutions. cap == 2 is all the generator needs — it distinguishes "exactly one solution"
// from "more than one" while never enumerating the full solution space of a sparse grid.
//
// This is the one deliberately-backtracking code path in the system. It lives ENTIRELY inside
// internal/generator and is never imported by internal/solver, so no counting/guessing path is
// reachable from POST /v1/solve (ADR-0003, asserted mechanically by TestAC3). cells holds 0 for
// a blank or 1..9 for a filled cell; the array is restored to its input state on return.
func solutionCount(cells *[81]uint8, cap int) int {
	count := 0
	var rec func()
	rec = func() {
		if count >= cap {
			return
		}
		// MRV: pick the empty cell with the fewest legal candidates — the branch that prunes
		// the search hardest. A cell with zero candidates is an immediate dead end.
		best := -1
		var bestCands uint16
		bestLen := 10
		for i := 0; i < 81; i++ {
			if cells[i] != 0 {
				continue
			}
			cands := legalMask(cells, i)
			n := bits.OnesCount16(cands)
			if n == 0 {
				return // dead end: this branch has no solution
			}
			if n < bestLen {
				best, bestCands, bestLen = i, cands, n
				if n == 1 {
					break // cannot do better than a forced cell
				}
			}
		}
		if best == -1 {
			count++ // no empty cell remains — the grid is full and legal: one solution
			return
		}
		for d := uint8(1); d <= 9; d++ {
			if bestCands&(1<<d) == 0 {
				continue
			}
			cells[best] = d
			rec()
			cells[best] = 0
			if count >= cap {
				return
			}
		}
	}
	rec()
	return count
}

// legalMask returns the bitset (bit d ⇔ digit d is legal) of digits that may occupy cell idx
// given its row, column, and box peers in cells.
func legalMask(cells *[81]uint8, idx int) uint16 {
	var used uint16
	r, c := idx/9, idx%9
	for k := 0; k < 9; k++ {
		used |= 1 << cells[r*9+k]
		used |= 1 << cells[k*9+c]
	}
	br, bc := (r/3)*3, (c/3)*3
	for dr := 0; dr < 3; dr++ {
		for dc := 0; dc < 3; dc++ {
			used |= 1 << cells[(br+dr)*9+(bc+dc)]
		}
	}
	// Digits 1..9 that are not used. Bit 0 (the blank marker) is masked off.
	return ^used & 0b0000_0011_1111_1110
}
