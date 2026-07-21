package solver_test

// This file is TEST CODE ONLY. It holds the brute-force backtracking ORACLE and the shared
// grid helpers used by the P-1 solver tests. Per EVAL.md §Ground-truth process, "a
// backtracking brute-force solver lives in test code only ... never in the shipped solve
// path." The oracle is ALLOWED to backtrack precisely because it is not the solver: the
// shipped internal/solver is forbidden to guess/backtrack (ADR-0001, ADR-0012). Nothing in
// this file is importable by non-test code.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadPuzzles reads the 25 seed grids from the repo-root puzzles.txt (D-Q2: all 25 unique;
// D-Q3: singles-only). CRLF-safe: each line is right-trimmed of \r and blank lines dropped,
// so a CRLF-terminated .txt (D-Q1) parses without a caller-side trim.
func loadPuzzles(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("..", "..", "puzzles.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading seed puzzles at %s: %v", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// parseBoard converts an 81-char grid string ('0' or '.' = blank) into a numeric board.
func parseBoard(s string) [81]int {
	var b [81]int
	for i := 0; i < 81 && i < len(s); i++ {
		if c := s[i]; c >= '1' && c <= '9' {
			b[i] = int(c - '0')
		}
	}
	return b
}

// boardString renders a numeric board as the canonical 81-char form ('0' = blank), matching
// sudoku.Grid.String() so replayed grids compare byte-for-byte against Event.GridAfter.
func boardString(b [81]int) string {
	var sb [81]byte
	for i, v := range b {
		sb[i] = byte('0' + v)
	}
	return string(sb[:])
}

// legal reports whether digit d (1..9) may be placed at idx without colliding in its row,
// column, or box.
func legal(b [81]int, idx, d int) bool {
	r, c := idx/9, idx%9
	for k := 0; k < 9; k++ {
		if b[r*9+k] == d { // row
			return false
		}
		if b[k*9+c] == d { // column
			return false
		}
	}
	br, bc := (r/3)*3, (c/3)*3
	for dr := 0; dr < 3; dr++ {
		for dc := 0; dc < 3; dc++ {
			if b[(br+dr)*9+(bc+dc)] == d {
				return false
			}
		}
	}
	return true
}

// candidateDigits returns the digits legal at an empty cell idx; nil for a filled cell.
func candidateDigits(b [81]int, idx int) []int {
	if b[idx] != 0 {
		return nil
	}
	var out []int
	for d := 1; d <= 9; d++ {
		if legal(b, idx, d) {
			out = append(out, d)
		}
	}
	return out
}

// bruteForce returns up to `limit` full solutions of b by DFS backtracking (row-major,
// first-empty). len == 1 proves the grid is uniquely solvable (confirms D-Q2). This is the
// ground-truth oracle; the shipped solver never runs this.
func bruteForce(b [81]int, limit int) []string {
	var out []string
	var rec func() bool
	rec = func() bool {
		idx := -1
		for i := 0; i < 81; i++ {
			if b[i] == 0 {
				idx = i
				break
			}
		}
		if idx == -1 {
			out = append(out, boardString(b))
			return len(out) >= limit
		}
		for d := 1; d <= 9; d++ {
			if legal(b, idx, d) {
				b[idx] = d
				if rec() {
					b[idx] = 0
					return true
				}
				b[idx] = 0
			}
		}
		return false
	}
	rec()
	return out
}

// constraints27Valid reports whether a COMPLETE 81-char grid satisfies all 9 row, 9 column,
// and 9 box constraints (each unit is a permutation of 1..9).
func constraints27Valid(s string) bool {
	if len(s) != 81 {
		return false
	}
	unit := func(get func(k int) byte) bool {
		var seen [10]bool
		for k := 0; k < 9; k++ {
			c := get(k)
			if c < '1' || c > '9' {
				return false
			}
			d := c - '0'
			if seen[d] {
				return false
			}
			seen[d] = true
		}
		return true
	}
	for i := 0; i < 9; i++ {
		row, col, box := i, i, i
		if !unit(func(k int) byte { return s[row*9+k] }) {
			return false
		}
		if !unit(func(k int) byte { return s[k*9+col] }) {
			return false
		}
		br, bc := (box/3)*3, (box%3)*3
		if !unit(func(k int) byte { return s[(br+k/3)*9+(bc+k%3)] }) {
			return false
		}
	}
	return true
}

// matchesGivens reports whether every given (non-blank) cell of input is preserved in
// solution — a completed grid must not overwrite a clue.
func matchesGivens(input, solution string) bool {
	for i := 0; i < 81 && i < len(input) && i < len(solution); i++ {
		if c := input[i]; c >= '1' && c <= '9' && solution[i] != c {
			return false
		}
	}
	return true
}

// nakedForced reports whether val is the ONLY candidate at cell given pre-state — the
// mechanical definition of a naked single (used to prove a placement was not a guess).
func nakedForced(pre [81]int, cell, val int) bool {
	cands := candidateDigits(pre, cell)
	return len(cands) == 1 && cands[0] == val
}

// hiddenForced reports whether val is legal at cell AND cell is the ONLY cell in at least one
// of its units (row, col, box) that can legally take val — the mechanical definition of a
// hidden single.
func hiddenForced(pre [81]int, cell, val int) bool {
	if pre[cell] != 0 || !legal(pre, cell, val) {
		return false
	}
	r, c := cell/9, cell%9
	countUnit := func(idxOf func(k int) int) int {
		n := 0
		for k := 0; k < 9; k++ {
			i := idxOf(k)
			if pre[i] == 0 && legal(pre, i, val) {
				n++
			}
		}
		return n
	}
	if countUnit(func(k int) int { return r*9 + k }) == 1 {
		return true
	}
	if countUnit(func(k int) int { return k*9 + c }) == 1 {
		return true
	}
	br, bc := (r/3)*3, (c/3)*3
	return countUnit(func(k int) int { return (br+k/3)*9 + (bc + k%3) }) == 1
}
