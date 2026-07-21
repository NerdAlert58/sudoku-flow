package solver_test

// AC-5 (status coverage). Source: DESIGN_DECISIONS §ADR-0011 (the four statuses; non-unique →
// stalled, NOT unsolvable); EVAL.md §Datasets and fixtures (status coverage) and §Eval matrix →
// UC-1. Four categories, >= 3 grids each:
//   above_tier_stalled -> stalled   (valid+unique but above the ADR-0002 tier)
//   non_unique         -> stalled   (>=2 solutions; the solver must NOT claim unsolvable)
//   in_tier_unsolvable -> unsolvable (a cell reaches zero candidates in-tier)
//   invalid_input      -> invalid_input (decided at sudoku.Parse, before the solver)

import (
	"errors"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// AC-5a — the parseable status fixtures each return their labeled status, and each category has
// >= 3 grids. Non-unique fixtures are additionally proven non-unique by the oracle (>=2 solutions),
// and in-tier-unsolvable fixtures are proven to have an immediate zero-candidate cell.
func TestAC5_StatusCoverage_ParseableFixtures(t *testing.T) {
	byCat := map[string]int{}
	for _, s := range loadStatusFixtures(t) {
		byCat[s.category]++
		grid, err := sudoku.Parse(s.puzzle)
		if err != nil {
			t.Fatalf("[%s] fixture must parse (status is decided by the solver, not Parse): %v\n  %s", s.category, err, s.puzzle)
		}

		switch s.category {
		case "non_unique":
			if sols := bruteForce(parseBoard(s.puzzle), 2); len(sols) < 2 {
				t.Errorf("[non_unique] oracle found %d solutions, want >= 2 (precondition)\n  %s", len(sols), s.puzzle)
			}
		case "in_tier_unsolvable":
			if !hasZeroCandidateCell(parseBoard(s.puzzle)) {
				t.Errorf("[in_tier_unsolvable] no cell has zero candidates — the contradiction is not in-tier reachable\n  %s", s.puzzle)
			}
		}

		res := solver.Solve(grid)
		wantStatus := solver.Status(s.status)
		if res.Status != wantStatus {
			t.Errorf("[%s] got status=%q, want %q\n  %s\n  note: %s", s.category, res.Status, wantStatus, s.puzzle, s.note)
		}
		if wantStatus != solver.StatusSolved && res.Solved {
			t.Errorf("[%s] solved=true on a %q grid", s.category, wantStatus)
		}
	}
	for _, cat := range []string{"above_tier_stalled", "non_unique", "in_tier_unsolvable"} {
		if byCat[cat] < 3 {
			t.Errorf("category %q has %d grids, want >= 3 (AC-5)", cat, byCat[cat])
		}
	}
}

// AC-5b — invalid_input: malformed / rule-violating givens are rejected at the sudoku.Parse trust
// boundary with the matching sentinel (ADR-0011), so they never reach the solver. >= 3 grids, one
// per error class plus an extra.
func TestAC5_StatusCoverage_InvalidInputRejectedAtParse(t *testing.T) {
	cases := []struct {
		name string
		grid string
		want error
	}{
		{"duplicate_in_row", "110000000" + strings.Repeat("0", 72), sudoku.ErrDuplicateValue},
		{"duplicate_in_box", "100000000" + "010000000" + strings.Repeat("0", 63), sudoku.ErrDuplicateValue},
		{"invalid_character", "X" + strings.Repeat("0", 80), sudoku.ErrInvalidCharacter},
		{"wrong_length", strings.Repeat("0", 80), sudoku.ErrInvalidLength},
	}
	if len(cases) < 3 {
		t.Fatalf("need >= 3 invalid_input grids, have %d", len(cases))
	}
	for _, c := range cases {
		_, err := sudoku.Parse(c.grid)
		if err == nil {
			t.Errorf("[%s] Parse accepted a malformed grid; want error", c.name)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("[%s] Parse error = %v, want errors.Is(_, %v)", c.name, err, c.want)
		}
	}
}

// hasZeroCandidateCell reports whether some empty cell of board has no legal digit — an in-tier
// constructive contradiction (reuses legal() from oracle_test.go).
func hasZeroCandidateCell(board [81]int) bool {
	for i := 0; i < 81; i++ {
		if board[i] != 0 {
			continue
		}
		any := false
		for d := 1; d <= 9; d++ {
			if legal(board, i, d) {
				any = true
				break
			}
		}
		if !any {
			return true
		}
	}
	return false
}
