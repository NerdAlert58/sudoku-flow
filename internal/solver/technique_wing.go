package solver

import "math/bits"

// Wings (ADR-0002 ladder index 9/10/11). All three are SOUND: a short chain of bi/tri-value
// cells forces a digit out of the cells that see the whole chain. None assumes uniqueness or
// reverts a guess.

// bivalueCells returns the empty cells holding exactly two candidates, ascending.
func bivalueCells(e *engine) []int {
	var out []int
	for i := 0; i < 81; i++ {
		if e.board[i] == 0 && bits.OnesCount16(e.cand[i]) == 2 {
			out = append(out, i)
		}
	}
	return out
}

// commonPeerElim removes digit d from every empty cell that sees BOTH a and b (excluding a, b
// and any extra cells), in ascending index order.
func commonPeerElim(e *engine, a, b, d int, extra ...int) []Elimination {
	bit := uint16(1) << d
	ex := [81]bool{}
	ex[a], ex[b] = true, true
	for _, x := range extra {
		ex[x] = true
	}
	var elims []Elimination
	for _, idx := range peersOf[a] {
		if ex[idx] || e.board[idx] != 0 {
			continue
		}
		if sees(idx, b) && e.cand[idx]&bit != 0 {
			elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
		}
	}
	return elims
}

// xyWing: pivot {X,Y} sees pincers {X,Z} and {Y,Z}; whichever pincer is not Z forces the other
// to Z, so Z is eliminated from any cell seeing both pincers. Witness = pivot + both pincers.
func xyWing(e *engine) (Event, bool) {
	for _, pivot := range bivalueCells(e) {
		pm := e.cand[pivot]
		peers := peersOf[pivot]
		for a := 0; a < len(peers); a++ {
			p1 := peers[a]
			if e.board[p1] != 0 || bits.OnesCount16(e.cand[p1]) != 2 {
				continue
			}
			for b := a + 1; b < len(peers); b++ {
				p2 := peers[b]
				if e.board[p2] != 0 || bits.OnesCount16(e.cand[p2]) != 2 {
					continue
				}
				s1, z1 := e.cand[p1]&pm, e.cand[p1]&^pm
				s2, z2 := e.cand[p2]&pm, e.cand[p2]&^pm
				// each pincer shares exactly one digit with the pivot and one outside digit;
				// the two shared digits differ (covering X and Y) and the outside digit matches.
				if bits.OnesCount16(s1) != 1 || bits.OnesCount16(z1) != 1 ||
					bits.OnesCount16(s2) != 1 || bits.OnesCount16(z2) != 1 {
					continue
				}
				if z1 != z2 || (s1|s2) != pm {
					continue
				}
				z := bits.TrailingZeros16(z1)
				if elims := commonPeerElim(e, p1, p2, z, pivot); len(elims) > 0 {
					return e.elimEvent("xy_wing", cellsOf([]int{pivot, p1, p2}), elims), true
				}
			}
		}
	}
	return Event{}, false
}

// xyzWing: pivot {X,Y,Z} sees pincers {X,Z} and {Y,Z}; Z is eliminated from any cell seeing all
// three (the pivot included, since the pivot may still be Z). Witness = pivot + both pincers.
func xyzWing(e *engine) (Event, bool) {
	for pivot := 0; pivot < 81; pivot++ {
		if e.board[pivot] != 0 || bits.OnesCount16(e.cand[pivot]) != 3 {
			continue
		}
		pm := e.cand[pivot]
		var pincers []int
		for _, p := range peersOf[pivot] {
			if e.board[p] == 0 && bits.OnesCount16(e.cand[p]) == 2 && e.cand[p]&^pm == 0 {
				pincers = append(pincers, p)
			}
		}
		for i := 0; i < len(pincers); i++ {
			for j := i + 1; j < len(pincers); j++ {
				p1, p2 := pincers[i], pincers[j]
				if (e.cand[p1] | e.cand[p2]) != pm {
					continue
				}
				common := e.cand[p1] & e.cand[p2]
				if bits.OnesCount16(common) != 1 {
					continue
				}
				z := bits.TrailingZeros16(common)
				elims := xyzElim(e, pivot, p1, p2, z)
				if len(elims) > 0 {
					return e.elimEvent("xyz_wing", cellsOf([]int{pivot, p1, p2}), elims), true
				}
			}
		}
	}
	return Event{}, false
}

// xyzElim removes d from every empty cell (other than the wing) that sees the pivot and both
// pincers.
func xyzElim(e *engine, pivot, p1, p2, d int) []Elimination {
	bit := uint16(1) << d
	var elims []Elimination
	for _, idx := range peersOf[pivot] {
		if idx == p1 || idx == p2 || e.board[idx] != 0 {
			continue
		}
		if sees(idx, p1) && sees(idx, p2) && e.cand[idx]&bit != 0 {
			elims = append(elims, Elimination{Cell: Cell{idx / 9, idx % 9}, Candidate: d})
		}
	}
	return elims
}

// wWing: two bi-value cells A,B with the same candidates {X,Y} that do not see each other,
// joined by a strong link on one digit (a unit where that digit sits in exactly two cells, one
// seeing A and one seeing B). The other digit is then eliminated from any cell seeing both A and
// B. Witness = A, B, and the two strong-link cells.
func wWing(e *engine) (Event, bool) {
	biv := bivalueCells(e)
	for i := 0; i < len(biv); i++ {
		for j := i + 1; j < len(biv); j++ {
			a, b := biv[i], biv[j]
			if e.cand[a] != e.cand[b] || sees(a, b) {
				continue
			}
			ds := digitsOf(e.cand[a]) // exactly two digits
			for k, z := range ds {
				if ev, ok := wWingLink(e, a, b, z, ds[1-k]); ok {
					return ev, true
				}
			}
		}
	}
	return Event{}, false
}

// wWingLink searches for a strong link on digit z bridging a and b; on success it eliminates the
// other digit from the cells seeing both a and b.
func wWingLink(e *engine, a, b, z, other int) (Event, bool) {
	zbit := uint16(1) << z
	for u := 0; u < 27; u++ {
		var pos []int
		for _, idx := range units[u] {
			if e.board[idx] == 0 && e.cand[idx]&zbit != 0 {
				pos = append(pos, idx)
			}
		}
		if len(pos) != 2 {
			continue
		}
		c1, c2 := pos[0], pos[1]
		if c1 == a || c1 == b || c2 == a || c2 == b {
			continue
		}
		linked := (sees(c1, a) && sees(c2, b)) || (sees(c1, b) && sees(c2, a))
		if !linked {
			continue
		}
		if elims := commonPeerElim(e, a, b, other); len(elims) > 0 {
			return e.elimEvent("w_wing", cellsOf([]int{a, b, c1, c2}), elims), true
		}
	}
	return Event{}, false
}
