package solver

// Locked candidates (ADR-0002 ladder index 2, 3). Both are SOUND intersection removals: a
// digit confined to the intersection of a box and a line lets us delete it from the rest of the
// other unit, because the digit must sit in the intersection.

// lockedPointing: within a box, if every cell that can hold digit d lies on a single row (or
// column), d is eliminated from that row (or column) outside the box. Witness = the box cells
// that hold d.
func lockedPointing(e *engine) (Event, bool) {
	for box := 0; box < 9; box++ {
		for d := 1; d <= 9; d++ {
			bit := uint16(1) << d
			var pos []int
			for _, idx := range units[18+box] {
				if e.board[idx] == 0 && e.cand[idx]&bit != 0 {
					pos = append(pos, idx)
				}
			}
			if len(pos) < 2 { // 0 or 1 (a lone cell is a hidden single, handled earlier)
				continue
			}
			sameRow, sameCol := true, true
			r0, c0 := pos[0]/9, pos[0]%9
			for _, idx := range pos {
				if idx/9 != r0 {
					sameRow = false
				}
				if idx%9 != c0 {
					sameCol = false
				}
			}
			if sameRow {
				if ev, ok := pointElim(e, d, r0, box, pos); ok {
					return ev, true
				}
			}
			if sameCol {
				if ev, ok := pointElim(e, d, 9+c0, box, pos); ok {
					return ev, true
				}
			}
		}
	}
	return Event{}, false
}

// pointElim removes d from the cells of line unit lineU that lie outside box, using pos as the
// witness. Returns false when nothing live is removed.
func pointElim(e *engine, d, lineU, box int, pos []int) (Event, bool) {
	bit := uint16(1) << d
	var elims []Elimination
	for _, idx := range units[lineU] {
		if e.board[idx] == 0 && boxOf(idx) != box && e.cand[idx]&bit != 0 {
			elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
		}
	}
	if len(elims) == 0 {
		return Event{}, false
	}
	return e.elimEvent("locked_candidates_pointing", cellsOf(pos), elims), true
}

// lockedClaiming: within a line (row or column), if every cell that can hold digit d lies in a
// single box, d is eliminated from the rest of that box. Witness = the line cells that hold d.
func lockedClaiming(e *engine) (Event, bool) {
	for u := 0; u < 18; u++ { // rows 0..8, then columns 9..17
		for d := 1; d <= 9; d++ {
			bit := uint16(1) << d
			var pos []int
			for _, idx := range units[u] {
				if e.board[idx] == 0 && e.cand[idx]&bit != 0 {
					pos = append(pos, idx)
				}
			}
			if len(pos) < 2 {
				continue
			}
			b0 := boxOf(pos[0])
			same := true
			for _, idx := range pos {
				if boxOf(idx) != b0 {
					same = false
				}
			}
			if !same {
				continue
			}
			var elims []Elimination
			for _, idx := range units[18+b0] {
				if e.board[idx] == 0 && !cellInUnit(u, idx) && e.cand[idx]&bit != 0 {
					elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
				}
			}
			if len(elims) > 0 {
				return e.elimEvent("locked_candidates_claiming", cellsOf(pos), elims), true
			}
		}
	}
	return Event{}, false
}
