package generator

// P-3 generator-level tests (RED until internal/generator is implemented).
//
// Test-defined source surface the builder implements (documented here as the contract):
//
//	// Generate produces a valid, uniquely-solvable puzzle graded at (or nearest to) the
//	// requested difficulty band. difficulty is one of "easy"/"medium"/"hard"/"expert"
//	// (case-insensitive); any other value returns ErrInvalidDifficulty and an empty puzzle
//	// (SECURITY F-14 — reject, never default-and-proceed). The returned puzzle is the
//	// canonical 81-char string ('0' blanks); grade is the achieved band as reported by
//	// solver.Grade ("Easy"/"Medium"/"Hard"/"Expert"). Uniqueness is enforced internally by a
//	// backtracking solution-counter (ADR-0003) that never leaks into internal/solver.
//	func Generate(difficulty string) (puzzle string, grade string, err error)
//
//	// GenerateSeeded is Generate with a caller-provided seed so test runs are reproducible
//	// (EVAL UC-3: "generated on the fly (seeded for reproducibility in tests)"). Same
//	// (difficulty, seed) yields the same puzzle.
//	func GenerateSeeded(difficulty string, seed int64) (puzzle string, grade string, err error)
//
//	// ErrInvalidDifficulty is the sentinel returned for an unknown difficulty value. It is a
//	// classification boundary (Cheney): the handler maps it to 400/invalid_input, and must be
//	// able to distinguish it from an internal generation failure (which is a 500).
//	var ErrInvalidDifficulty error

import (
	"errors"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

var bands = []string{"easy", "medium", "hard", "expert"}

// --- independent brute-force solution counter (TEST CODE ONLY) ----------------------------
//
// This counter is deliberately NOT the generator's own uniqueness counter and does NOT import
// internal/solver — it is a plain backtracking DFS (MRV cell selection) written here so AC-1's
// uniqueness assertion is confirmed by an oracle independent of the code under test. It counts
// up to cap solutions (cap=2 is enough to tell "exactly one" from "more than one"). Returns -1
// if the string is not a legal 81-char 0..9/'.' grid.

func countSolutions(puzzle string, cap int) int {
	if len(puzzle) != 81 {
		return -1
	}
	var cells [81]int
	for i := 0; i < 81; i++ {
		switch c := puzzle[i]; {
		case c >= '1' && c <= '9':
			cells[i] = int(c - '0')
		case c == '0' || c == '.':
			cells[i] = 0
		default:
			return -1
		}
	}
	count := 0
	var rec func()
	rec = func() {
		if count >= cap {
			return
		}
		// MRV: choose the empty cell with the fewest legal candidates.
		best := -1
		var bestCands []int
		for i := 0; i < 81; i++ {
			if cells[i] != 0 {
				continue
			}
			cands := candidatesAt(&cells, i)
			if len(cands) == 0 {
				return // dead end — this branch has no solution
			}
			if best == -1 || len(cands) < len(bestCands) {
				best, bestCands = i, cands
			}
		}
		if best == -1 {
			count++ // grid full and legal ⇒ one solution
			return
		}
		for _, d := range bestCands {
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

func candidatesAt(cells *[81]int, idx int) []int {
	var used [10]bool
	r, c := idx/9, idx%9
	for k := 0; k < 9; k++ {
		used[cells[r*9+k]] = true
		used[cells[k*9+c]] = true
	}
	br, bc := (r/3)*3, (c/3)*3
	for dr := 0; dr < 3; dr++ {
		for dc := 0; dc < 3; dc++ {
			used[cells[(br+dr)*9+(bc+dc)]] = true
		}
	}
	var out []int
	for d := 1; d <= 9; d++ {
		if !used[d] {
			out = append(out, d)
		}
	}
	return out
}

// AC-1 (EVAL §Eval matrix → UC-3): every generated puzzle is valid (81 chars, rule-valid
// givens at the sudoku.Parse boundary) and has EXACTLY ONE solution, confirmed by the
// independent brute-force counter above (cap 2, asserts == 1). Also asserts the returned grade
// is self-consistent with the real solver.Grade of the puzzle (ADR-0013).
func TestAC1_Generator_UniqueAndValid(t *testing.T) {
	const perBand = 5
	for bi, band := range bands {
		for i := 0; i < perBand; i++ {
			seed := int64(bi*1000 + i + 1)
			puzzle, grade, err := GenerateSeeded(band, seed)
			if err != nil {
				t.Fatalf("band=%s seed=%d: unexpected error: %v", band, seed, err)
			}
			if len(puzzle) != 81 {
				t.Fatalf("band=%s seed=%d: puzzle length = %d, want 81 (%q)", band, seed, len(puzzle), puzzle)
			}
			g, perr := sudoku.Parse(puzzle)
			if perr != nil {
				t.Fatalf("band=%s seed=%d: generated puzzle is not rule-valid: %v (%q)", band, seed, perr, puzzle)
			}
			if n := countSolutions(puzzle, 2); n != 1 {
				t.Fatalf("band=%s seed=%d: independent solution count = %d, want EXACTLY 1 (puzzle=%q)", band, seed, n, puzzle)
			}
			if oracle := solver.Grade(g); !strings.EqualFold(grade, oracle) {
				t.Fatalf("band=%s seed=%d: returned grade %q != solver.Grade %q (contract inconsistency)", band, seed, grade, oracle)
			}
		}
	}
}

// AC-2 (EVAL §Eval matrix → UC-3): across a sample per band, >= 90% of generated puzzles are
// graded by the REAL solver.Grade (ADR-0013) at the requested band. Nearest-achievable with
// bounded retry accounts for the < 10% remainder. The oracle is solver.Grade, not the
// generator's self-reported grade.
func TestAC2_Generator_BandHitRateAtLeast90Pct(t *testing.T) {
	if testing.Short() {
		t.Skip("band-hit-rate sampling skipped in -short mode")
	}
	const perBand = 15
	totalHits, total := 0, 0
	for bi, band := range bands {
		bandHits := 0
		for i := 0; i < perBand; i++ {
			seed := int64(bi*10000 + i + 1)
			puzzle, _, err := GenerateSeeded(band, seed)
			if err != nil {
				t.Fatalf("band=%s seed=%d: unexpected error: %v", band, seed, err)
			}
			g, perr := sudoku.Parse(puzzle)
			if perr != nil {
				t.Fatalf("band=%s seed=%d: generated puzzle not rule-valid: %v", band, seed, perr)
			}
			if strings.EqualFold(solver.Grade(g), band) {
				bandHits++
			}
			total++
		}
		totalHits += bandHits
		t.Logf("band %-7s: %d/%d hit requested band", band, bandHits, perBand)
	}
	rate := float64(totalHits) / float64(total)
	if rate < 0.90 {
		t.Fatalf("band-hit rate = %.1f%% (%d/%d), want >= 90%% (EVAL UC-3)", rate*100, totalHits, total)
	}
}

// AC-4 (SECURITY §F-14) at the source boundary: an unknown difficulty returns the typed
// ErrInvalidDifficulty sentinel and an empty puzzle — never a defaulted-and-proceeded puzzle.
func TestAC4_Generator_UnknownDifficultyReturnsSentinel(t *testing.T) {
	for _, bad := range []string{"bogus", "", "easyy", "extreme", "0", "hardest"} {
		puzzle, _, err := GenerateSeeded(bad, 1)
		if err == nil {
			t.Fatalf("difficulty %q: got nil error, want ErrInvalidDifficulty (F-14: no default-and-proceed)", bad)
		}
		if !errors.Is(err, ErrInvalidDifficulty) {
			t.Fatalf("difficulty %q: error %v is not ErrInvalidDifficulty", bad, err)
		}
		if puzzle != "" {
			t.Fatalf("difficulty %q: puzzle must be empty on error, got %q", bad, puzzle)
		}
	}
}
