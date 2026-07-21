// Package generator builds valid, uniquely-solvable Sudoku puzzles graded at (or nearest to) a
// requested difficulty band. It is the one place the two epistemic rules of the system meet
// (ADR-0003): it MAY backtrack — its internal uniqueness counter (counter.go) is a standard
// depth-first solution-counter — while the benchmarked solver never does.
//
// Import boundary (ADR-0003, asserted by TestAC3): generator depends on internal/solver (used
// only as a difficulty ORACLE via Grade / SolveWithMaxTechnique) and internal/sudoku, and is a
// leaf consumer of both. internal/solver must never import internal/generator, so the
// backtracking counter is unreachable from POST /v1/solve.
//
// Method (ARCHITECTURE §Components → internal/generator): build a full solution by symmetry
// transforms of a base solved grid (no search — grid.go), then dig clues in random order,
// keeping a removal only while (1) the backtracking counter still reports exactly one solution
// and (2) the solver can still solve it within the target band's technique ceiling. Retry with
// fresh grids until the real solver.Grade lands on the requested band, or return the
// nearest-achievable puzzle within a bounded attempt budget.
package generator

import (
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// ErrInvalidDifficulty is the sentinel returned for an unknown difficulty value. It is a
// classification boundary (SECURITY F-14): the handler maps it to 400/invalid_input and must
// distinguish it from an internal generation failure (a 500). Callers test with errors.Is.
var ErrInvalidDifficulty = errors.New("generator: unknown difficulty")

// band binds a requested difficulty to its solver grade string and the ladder technique that is
// the ceiling of its band (the hardest technique allowed while digging so the puzzle stays
// inside the band). The bands are contiguous ranges of the ADR-0013 ladder, so a puzzle that is
// solvable within ceilingTech but not below it grades at exactly this band.
type band struct {
	grade   string           // solver.Grade output for this band
	ceiling solver.Technique // hardest technique permitted while digging
}

// bandSpecs is the case-insensitive difficulty allowlist (SECURITY F-14 — reject, never default).
var bandSpecs = map[string]band{
	"easy":   {grade: "Easy", ceiling: solver.HiddenSingle},
	"medium": {grade: "Medium", ceiling: solver.HiddenSubset},
	"hard":   {grade: "Hard", ceiling: solver.XYWing},
	"expert": {grade: "Expert", ceiling: solver.SimpleColouring},
}

// bandIndex orders the grades so nearest-achievable can pick the closest band when the exact
// band is not hit within budget.
var bandIndex = map[string]int{"Easy": 0, "Medium": 1, "Hard": 2, "Expert": 3}

// attemptBudget caps the retry loop per band. Easy/Medium land almost immediately; Hard and
// especially Expert need a fresh solved grid more often because a random maximal dig only
// sometimes requires the band's signature techniques, so they get a larger budget. Exceeding
// the budget returns the nearest-achievable puzzle rather than failing (EVAL UC-3).
var attemptBudget = map[string]int{"Easy": 8, "Medium": 30, "Hard": 120, "Expert": 240}

// maxDigPasses bounds the dig to a handful of reshuffled passes; digging converges to a minimal
// puzzle in two or three passes, and a minimal puzzle is what pushes difficulty up toward the
// harder bands.
const maxDigPasses = 8

// GenerateSeeded is Generate with a caller-provided seed for reproducibility (EVAL UC-3). The
// same (difficulty, seed) yields the same puzzle: all randomness is threaded through a single
// seeded *rand.Rand — no global rand, no time-based seed on this path. An unknown difficulty
// returns ErrInvalidDifficulty and an empty puzzle (F-14: no default-and-proceed).
func GenerateSeeded(difficulty string, seed int64) (puzzle string, grade string, err error) {
	b, ok := bandSpecs[strings.ToLower(strings.TrimSpace(difficulty))]
	if !ok {
		return "", "", ErrInvalidDifficulty
	}

	rng := rand.New(rand.NewSource(seed))
	budget := attemptBudget[b.grade]

	var bestPuzzle, bestGrade string
	bestDist := 1 << 30

	for attempt := 0; attempt < budget; attempt++ {
		cells := digForBand(fullSolvedGrid(rng), b.ceiling, rng)
		p := render(&cells)

		g, perr := sudoku.Parse(p)
		if perr != nil {
			continue // unreachable: a dug solved grid is always rule-valid — defensive only
		}
		got := solver.Grade(g)
		if got == b.grade {
			return p, got, nil
		}

		// Track the nearest-achievable puzzle in case the exact band is never hit.
		if d := bandDist(got, b.grade); bestPuzzle == "" || d < bestDist {
			bestPuzzle, bestGrade, bestDist = p, got, d
		}
	}
	return bestPuzzle, bestGrade, nil
}

// Generate is the production entry: an unseeded GenerateSeeded. The seed is non-deterministic so
// each call yields a fresh puzzle; the difficulty allowlist is enforced identically.
func Generate(difficulty string) (puzzle string, grade string, err error) {
	return GenerateSeeded(difficulty, time.Now().UnixNano())
}

// digForBand removes clues from a full solution in random order, keeping a removal only while
// the puzzle stays uniquely solvable (the backtracking counter — ADR-0004 uniqueness authority)
// AND still solvable by the solver within ceil (the difficulty oracle, which bounds the band and
// guarantees the puzzle remains logic-solvable). Reshuffled passes run until a full pass removes
// nothing, yielding a minimal puzzle. The result is always unique and solvable within ceil, so
// its grade is at most the ceiling's band; the caller confirms the exact band with solver.Grade.
func digForBand(full solvedGrid, ceil solver.Technique, rng *rand.Rand) [81]uint8 {
	cells := [81]uint8(full)
	order := rng.Perm(81)

	for pass := 0; pass < maxDigPasses; pass++ {
		removed := 0
		for _, c := range order {
			if cells[c] == 0 {
				continue
			}
			saved := cells[c]
			cells[c] = 0

			// Uniqueness gate (ADR-0004): the backtracking counter is the authority. This is
			// checked first so it is genuinely load-bearing, not a rubber stamp on the solver.
			if solutionCount(&cells, 2) != 1 {
				cells[c] = saved
				continue
			}
			// Difficulty gate: keep the puzzle inside the requested band's technique ceiling.
			if !solvableWithin(&cells, ceil) {
				cells[c] = saved
				continue
			}
			removed++
		}
		if removed == 0 {
			break
		}
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	return cells
}

// solvableWithin reports whether the constructive solver can fully solve cells using only ladder
// techniques up to and including max. A logic solve is by definition non-guessing, so a puzzle
// that solves within any ceiling also has a unique solution — but uniqueness is nonetheless
// certified independently by solutionCount (ADR-0004), keeping the two oracles' roles distinct.
func solvableWithin(cells *[81]uint8, max solver.Technique) bool {
	g, err := sudoku.Parse(render(cells))
	if err != nil {
		return false
	}
	return solver.SolveWithMaxTechnique(g, max).Status == solver.StatusSolved
}

// bandDist is the ladder-distance between two grades, used to choose the nearest-achievable band
// when the exact one is not hit. An unsolvable/empty grade is treated as maximally distant.
func bandDist(got, want string) int {
	gi, ok := bandIndex[got]
	if !ok {
		return 1 << 20
	}
	wi := bandIndex[want]
	if gi > wi {
		return gi - wi
	}
	return wi - gi
}
