package solver

// Basic fish (ADR-0002 ladder index 6/7/8): X-wing (size 2), swordfish (size 3), jellyfish
// (size 4). A fish on digit d picks `size` base lines whose d-positions are confined to exactly
// `size` cover lines; d must then sit on the intersections, so it is eliminated from those cover
// lines in every non-base line. This is SOUND and uniqueness-free. Cheapest-first ordering means
// a genuine size-k fish only fires when no smaller fish (or cheaper technique) can act.

// fish runs one fish size for one canonical name, trying rows-as-base then columns-as-base,
// digits ascending. Witness = the fish corner cells (the base-line d-positions).
func fish(e *engine, size int, name string) (Event, bool) {
	for orient := 0; orient < 2; orient++ { // 0: base=rows/cover=cols, 1: base=cols/cover=rows
		for d := 1; d <= 9; d++ {
			bit := uint16(1) << d
			var posByLine [9][]int
			var lines []int
			for line := 0; line < 9; line++ {
				for cross := 0; cross < 9; cross++ {
					if e.board[cellAt(orient, line, cross)] == 0 &&
						e.cand[cellAt(orient, line, cross)]&bit != 0 {
						posByLine[line] = append(posByLine[line], cross)
					}
				}
				if n := len(posByLine[line]); n >= 2 && n <= size {
					lines = append(lines, line)
				}
			}
			if len(lines) < size {
				continue
			}
			var result Event
			found := combinations(len(lines), size, func(sel []int) bool {
				cover := [9]bool{}
				coverCount := 0
				base := [9]bool{}
				for _, si := range sel {
					base[lines[si]] = true
					for _, cross := range posByLine[lines[si]] {
						if !cover[cross] {
							cover[cross] = true
							coverCount++
						}
					}
				}
				if coverCount != size {
					return false
				}
				var elims []Elimination
				for line := 0; line < 9; line++ {
					if base[line] {
						continue
					}
					for cross := 0; cross < 9; cross++ {
						if !cover[cross] {
							continue
						}
						idx := cellAt(orient, line, cross)
						if e.board[idx] == 0 && e.cand[idx]&bit != 0 {
							elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
						}
					}
				}
				if len(elims) == 0 {
					return false
				}
				var witness []int
				for _, si := range sel {
					line := lines[si]
					for _, cross := range posByLine[line] {
						witness = append(witness, cellAt(orient, line, cross))
					}
				}
				result = e.elimEvent(name, cellsOf(witness), elims)
				return true
			})
			if found {
				return result, true
			}
		}
	}
	return Event{}, false
}

// cellAt maps (line, cross) to a cell index for the given fish orientation.
func cellAt(orient, line, cross int) int {
	if orient == 0 {
		return line*9 + cross // base = row `line`, cover = column `cross`
	}
	return cross*9 + line // base = column `line`, cover = row `cross`
}
