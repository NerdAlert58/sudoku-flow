package solver_test

// AC-4 (difficulty grader) under the ADR-0018 two-tier gate. Source: DESIGN_DECISIONS §ADR-0013
// (grade = hardest technique the solve was FORCED to use, Sudoku-Explainer ordering, bucketed
// Easy/Medium/Hard/Expert), §ADR-0018 (two-tier gate); EVAL.md §Ground-truth process (ceiling +
// floor method).
//
// Uses solver.Grade, solver.SolveResult.HardestTechnique, solver.SolveWithMaxTechnique.

import (
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// AC-4a — every fixture's declared band equals its technique's ADR-0013 band (label
// self-consistency). For an ISOLABLE fixture the whole-puzzle Grade equals that band and
// HardestTechnique agrees (the technique IS the bottleneck). For an UN-ISOLABLE fixture the puzzle
// may legitimately grade HIGHER (a costlier technique is the true bottleneck), so Grade is only
// required to be no CHEAPER than the firing technique's band.
func TestAC4_Grader_AssignsLabeledBand(t *testing.T) {
	for _, f := range loadAdvancedFixtures(t) {
		if bandOf[f.technique] != f.band {
			t.Errorf("[%s] declared band %q != bandOf[%s]=%q (label inconsistency)",
				f.technique, f.band, f.technique, bandOf[f.technique])
		}
		grid, err := sudoku.Parse(f.puzzle)
		if err != nil {
			t.Fatalf("[%s] parse: %v", f.technique, err)
		}
		if f.tier == tierIsolable {
			if got := solver.Grade(grid); got != f.band {
				t.Errorf("[%s isolable] Grade = %q, want %q\n  %s", f.technique, got, f.band, f.puzzle)
			}
			res := solver.Solve(grid)
			if res.Status != solver.StatusSolved {
				t.Errorf("[%s isolable] not solved (%q); cannot confirm HardestTechnique", f.technique, res.Status)
				continue
			}
			if hb := bandOf[string(res.HardestTechnique)]; hb != f.band {
				t.Errorf("[%s isolable] HardestTechnique=%q (band %q), want band %q", f.technique, res.HardestTechnique, hb, f.band)
			}
		} else {
			if got := solver.Grade(grid); got != "" && bandRank[got] < bandRank[f.band] {
				t.Errorf("[%s unisolable] Grade=%q is CHEAPER than the firing technique's band %q — label too high",
					f.technique, got, f.band)
			}
		}
	}
}

// AC-4b — CEILING + FLOOR (isolable tier only; the ADR-0013 / EVAL ground-truth method). For each
// isolable fixture with hardest technique T:
//
//	ceiling: capping the ladder at T must still SOLVE (nothing above T is needed) — and equal oracle;
//	floor:   capping at T's predecessor must NOT solve (something at T's tier is needed).
//
// Together these bracket the grade at exactly T's band. Un-isolable fixtures are excluded by
// construction: they are the ones for which no exact-ceiling puzzle exists (ADR-0018).
func TestAC4_Grader_CeilingAndFloorBracketTheTier(t *testing.T) {
	for _, f := range loadAdvancedFixtures(t) {
		if f.tier != tierIsolable {
			continue
		}
		grid, err := sudoku.Parse(f.puzzle)
		if err != nil {
			t.Fatalf("[%s] parse: %v", f.technique, err)
		}
		k := tierIndex(f.technique)

		ceil := solver.SolveWithMaxTechnique(grid, solver.Technique(f.technique))
		if ceil.Status != solver.StatusSolved {
			t.Errorf("[%s] CEILING: capped at %q got %q, want solved (fixture needs a technique ABOVE its label)\n  %s",
				f.technique, f.technique, ceil.Status, f.puzzle)
		} else if ceil.Solution != f.solution {
			t.Errorf("[%s] CEILING: capped-solve != oracle solution", f.technique)
		}

		if k >= 1 {
			floor := solver.SolveWithMaxTechnique(grid, solver.Technique(expectedLadder[k-1]))
			if floor.Status == solver.StatusSolved {
				t.Errorf("[%s] FLOOR: capped at %q still solved — fixture does not require %q",
					f.technique, expectedLadder[k-1], f.technique)
			}
		}
	}
}

// AC-4c — the band buckets are exactly the ADR-0013 four, and Grade returns "" for a non-solved
// grid (the empty grid stalls). Guards the grader's output vocabulary.
func TestAC4_Grader_BandVocabularyAndUnsolved(t *testing.T) {
	valid := map[string]bool{"Easy": true, "Medium": true, "Hard": true, "Expert": true}
	for _, f := range loadAdvancedFixtures(t) {
		if !valid[f.band] {
			t.Fatalf("fixture band %q is not one of Easy/Medium/Hard/Expert", f.band)
		}
	}
	blank, err := sudoku.Parse(strings.Repeat("0", 81))
	if err != nil {
		t.Fatalf("empty grid must parse: %v", err)
	}
	if got := solver.Grade(blank); got != "" {
		t.Errorf("Grade(empty grid) = %q, want \"\" (a stalled grid has no grade)", got)
	}
}
