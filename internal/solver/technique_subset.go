package solver

import "math/bits"

// Subsets (ADR-0002 ladder index 4, 5), sizes 2/3/4 (pairs, triples, quads). Both are SOUND:
// a naked subset locks k digits into k cells, freeing the rest of the unit of those digits; a
// hidden subset locks k digits into k cells, freeing those cells of every other digit. Sizes
// are tried ascending so the smallest (cheapest) pattern fires first.

// nakedSubset: k empty cells in a unit whose candidates union to exactly k digits — those k
// digits are removed from the unit's other cells. Witness = the k subset cells.
func nakedSubset(e *engine) (Event, bool) {
	for size := 2; size <= 4; size++ {
		for u := 0; u < 27; u++ {
			var cells []int
			for _, idx := range units[u] {
				if e.board[idx] == 0 {
					if n := bits.OnesCount16(e.cand[idx]); n >= 2 && n <= size {
						cells = append(cells, idx)
					}
				}
			}
			if len(cells) < size {
				continue
			}
			var result Event
			found := combinations(len(cells), size, func(sel []int) bool {
				var union uint16
				for _, si := range sel {
					union |= e.cand[cells[si]]
				}
				if bits.OnesCount16(union) != size {
					return false
				}
				inSub := [81]bool{}
				chosen := make([]int, size)
				for i, si := range sel {
					inSub[cells[si]] = true
					chosen[i] = cells[si]
				}
				var elims []Elimination
				for _, idx := range units[u] {
					if e.board[idx] != 0 || inSub[idx] {
						continue
					}
					common := e.cand[idx] & union
					for _, d := range digitsOf(common) {
						elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
					}
				}
				if len(elims) == 0 {
					return false
				}
				result = e.elimEvent("naked_subset", cellsOf(chosen), elims)
				return true
			})
			if found {
				return result, true
			}
		}
	}
	return Event{}, false
}

// hiddenSubset: k digits whose candidate positions in a unit union to exactly k cells — in
// those k cells every OTHER digit is removed. Witness = the k cells.
func hiddenSubset(e *engine) (Event, bool) {
	for size := 2; size <= 4; size++ {
		for u := 0; u < 27; u++ {
			var digitPos [10][]int
			var digitList []int
			for d := 1; d <= 9; d++ {
				for _, idx := range units[u] {
					if e.board[idx] == 0 && e.cand[idx]&(uint16(1)<<d) != 0 {
						digitPos[d] = append(digitPos[d], idx)
					}
				}
				if n := len(digitPos[d]); n >= 2 && n <= size {
					digitList = append(digitList, d)
				}
			}
			if len(digitList) < size {
				continue
			}
			var result Event
			found := combinations(len(digitList), size, func(sel []int) bool {
				posSet := [81]bool{}
				count := 0
				var digMask uint16
				for _, si := range sel {
					d := digitList[si]
					digMask |= uint16(1) << d
					for _, idx := range digitPos[d] {
						if !posSet[idx] {
							posSet[idx] = true
							count++
						}
					}
				}
				if count != size {
					return false
				}
				var elims []Elimination
				var cells []int
				for _, idx := range units[u] {
					if !posSet[idx] {
						continue
					}
					cells = append(cells, idx)
					extra := e.cand[idx] &^ digMask
					for _, d := range digitsOf(extra) {
						elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
					}
				}
				if len(elims) == 0 {
					return false
				}
				result = e.elimEvent("hidden_subset", cellsOf(cells), elims)
				return true
			})
			if found {
				return result, true
			}
		}
	}
	return Event{}, false
}
