package generator

import "math/rand"

// solvedGrid is a completed 9x9 Sudoku held row-major as 81 cells of 1..9. It is only ever
// produced by fullSolvedGrid, so its validity is an invariant rather than something callers
// re-check.
type solvedGrid [81]uint8

// baseSolved is the canonical valid solved grid produced by the classic
// value(r,c) = (3*(r%3) + r/3 + c) mod 9 + 1 pattern — a legal Sudoku by construction. Every
// generated grid is a symmetry image of this one, so no backtracking is needed to obtain a
// full solution (ADR-0003 step 1 / ARCHITECTURE §Components → internal/generator).
func baseSolved() solvedGrid {
	var g solvedGrid
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			g[r*9+c] = uint8((3*(r%3)+r/3+c)%9 + 1)
		}
	}
	return g
}

// fullSolvedGrid returns a random completed grid by applying validity-preserving symmetry
// transforms to baseSolved, driven entirely by rng so the same seed reproduces the same grid
// (EVAL UC-3 reproducibility). The transforms — digit relabel, row/column permutation within a
// band/stack, band/stack permutation, and an optional transpose — span the Sudoku symmetry
// group without ever violating the one-per-unit rule, so the result is a legal solution without
// any search.
func fullSolvedGrid(rng *rand.Rand) solvedGrid {
	base := baseSolved()

	// Digit relabel: a random bijection of 1..9. relabel[d] is the new value for old digit d.
	relabel := [10]uint8{}
	perm := rng.Perm(9)
	for i, p := range perm {
		relabel[i+1] = uint8(p + 1)
	}

	rowOrder := bandedOrder(rng) // permuted rows, respecting band structure
	colOrder := bandedOrder(rng) // permuted cols, respecting stack structure

	var g solvedGrid
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			g[r*9+c] = relabel[base[rowOrder[r]*9+colOrder[c]]]
		}
	}

	if rng.Intn(2) == 0 {
		g = transpose(g)
	}
	return g
}

// bandedOrder returns a permutation of 0..8 formed by shuffling the three bands (rows/cols
// 0-2, 3-5, 6-8) and independently shuffling the three lines within each band. This is exactly
// the subgroup of index permutations that preserves the box structure, so applying it to rows
// (or columns) keeps every box a legal set of 9 distinct digits.
func bandedOrder(rng *rand.Rand) [9]int {
	bands := []int{0, 1, 2}
	rng.Shuffle(len(bands), func(i, j int) { bands[i], bands[j] = bands[j], bands[i] })

	var order [9]int
	pos := 0
	for _, b := range bands {
		lines := []int{0, 1, 2}
		rng.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
		for _, l := range lines {
			order[pos] = b*3 + l
			pos++
		}
	}
	return order
}

// transpose reflects the grid across its main diagonal — another validity-preserving symmetry.
func transpose(g solvedGrid) solvedGrid {
	var out solvedGrid
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			out[c*9+r] = g[r*9+c]
		}
	}
	return out
}

// render produces the canonical 81-char string ('0' for a blank) from a working grid, matching
// sudoku.Grid.String() so the output round-trips through sudoku.Parse unchanged.
func render(cells *[81]uint8) string {
	var out [81]byte
	for i, v := range cells {
		out[i] = '0' + v
	}
	return string(out[:])
}
