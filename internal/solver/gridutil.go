package solver

// Geometry helpers shared by the advanced techniques: precomputed peer lists, unit membership,
// and small bit / combination utilities. All are pure and read-only after init.

// peersOf[i] lists the 20 cells sharing a row, column, or box with cell i (i itself excluded),
// in ascending index order — a deterministic scan order for the wing and colouring eliminations.
var peersOf = buildPeers()

func buildPeers() [81][]int {
	var p [81][]int
	for i := 0; i < 81; i++ {
		var seen [81]bool
		r, c := i/9, i%9
		for k := 0; k < 9; k++ {
			seen[r*9+k] = true
			seen[k*9+c] = true
		}
		br, bc := (r/3)*3, (c/3)*3
		for dr := 0; dr < 3; dr++ {
			for dc := 0; dc < 3; dc++ {
				seen[(br+dr)*9+(bc+dc)] = true
			}
		}
		seen[i] = false
		lst := make([]int, 0, 20)
		for j := 0; j < 81; j++ {
			if seen[j] {
				lst = append(lst, j)
			}
		}
		p[i] = lst
	}
	return p
}

// boxOf returns the 0..8 box index of cell idx.
func boxOf(idx int) int {
	return (idx/9/3)*3 + (idx%9)/3
}

// sees reports whether cells a and b are peers (share a row, column, or box). A cell does not
// see itself.
func sees(a, b int) bool {
	if a == b {
		return false
	}
	ra, ca := a/9, a%9
	rb, cb := b/9, b%9
	if ra == rb || ca == cb {
		return true
	}
	return ra/3 == rb/3 && ca/3 == cb/3
}

// cellInUnit reports whether cell idx belongs to unit u (rows 0..8, cols 9..17, boxes 18..26).
func cellInUnit(u, idx int) bool {
	switch {
	case u < 9:
		return idx/9 == u
	default:
		return idx%9 == u-9
	}
}

// cellsOf converts cell indices to Cell coordinates, preserving order (for witness lists).
func cellsOf(idxs []int) []Cell {
	out := make([]Cell, len(idxs))
	for i, idx := range idxs {
		out[i] = Cell{Row: idx / 9, Col: idx % 9}
	}
	return out
}

// digitsOf returns the ascending digits (1..9) present in a candidate mask.
func digitsOf(mask uint16) []int {
	var out []int
	for d := 1; d <= 9; d++ {
		if mask&(uint16(1)<<d) != 0 {
			out = append(out, d)
		}
	}
	return out
}

// combinations invokes fn with each ascending k-combination of the indices [0, n). It stops
// early and returns true as soon as fn returns true; otherwise it returns false when exhausted.
// The idx slice is reused between calls, so fn must copy anything it retains.
func combinations(n, k int, fn func(idx []int) bool) bool {
	if k <= 0 || k > n {
		return false
	}
	idx := make([]int, k)
	var rec func(start, depth int) bool
	rec = func(start, depth int) bool {
		if depth == k {
			return fn(idx)
		}
		for i := start; i <= n-(k-depth); i++ {
			idx[depth] = i
			if rec(i+1, depth+1) {
				return true
			}
		}
		return false
	}
	return rec(0, 0)
}
