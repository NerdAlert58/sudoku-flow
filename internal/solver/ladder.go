package solver

import "github.com/scottbushyhead/sudoku-flow/internal/sudoku"

// Technique is a canonical technique name; its string value IS the Event.Technique string.
type Technique string

// The ADR-0002 technique vocabulary. Singles PLACE; every entry at index >= 2 only ELIMINATES.
const (
	NakedSingle              Technique = "naked_single"
	HiddenSingle             Technique = "hidden_single"
	LockedCandidatesPointing Technique = "locked_candidates_pointing"
	LockedCandidatesClaiming Technique = "locked_candidates_claiming"
	NakedSubset              Technique = "naked_subset"
	HiddenSubset             Technique = "hidden_subset"
	XWing                    Technique = "x_wing"
	Swordfish                Technique = "swordfish"
	Jellyfish                Technique = "jellyfish"
	XYWing                   Technique = "xy_wing"
	XYZWing                  Technique = "xyz_wing"
	WWing                    Technique = "w_wing"
	SimpleColouring          Technique = "simple_colouring"
)

// technique binds a canonical name to its runner. A runner reads engine.board/engine.cand,
// performs its effect (a single placement, or one pattern's live eliminations), and returns
// the event plus whether it fired. Runners never guess or revert.
type technique struct {
	name Technique
	run  func(e *engine) (Event, bool)
}

// ladderTechniques is the enabled ladder in cheapest-first order (ADR-0002). runEngine walks
// it index 0..maxIdx and applies the first that makes a productive step, so a technique fires
// only when nothing cheaper can act — the discipline that makes HardestTechnique the genuinely
// required tier. Fish share one implementation parameterised by size (2/3/4).
var ladderTechniques = []technique{
	{NakedSingle, nakedSingle},
	{HiddenSingle, hiddenSingle},
	{LockedCandidatesPointing, lockedPointing},
	{LockedCandidatesClaiming, lockedClaiming},
	{NakedSubset, nakedSubset},
	{HiddenSubset, hiddenSubset},
	{XWing, func(e *engine) (Event, bool) { return fish(e, 2, "x_wing") }},
	{Swordfish, func(e *engine) (Event, bool) { return fish(e, 3, "swordfish") }},
	{Jellyfish, func(e *engine) (Event, bool) { return fish(e, 4, "jellyfish") }},
	{XYWing, xyWing},
	{XYZWing, xyzWing},
	{WWing, wWing},
	{SimpleColouring, simpleColouring},
}

// Ladder is the shipped technique ladder, cheapest-first (== ADR-0002). Read-only.
var Ladder = func() []Technique {
	l := make([]Technique, len(ladderTechniques))
	for i, t := range ladderTechniques {
		l[i] = t.name
	}
	return l
}()

// techBand maps each technique to its ADR-0013 difficulty band (Sudoku-Explainer buckets):
// Easy = singles; Medium = locked candidates + subsets; Hard = basic fish + xy-wing;
// Expert = xyz-wing / w-wing / simple colouring.
var techBand = map[Technique]string{
	NakedSingle:              "Easy",
	HiddenSingle:             "Easy",
	LockedCandidatesPointing: "Medium",
	LockedCandidatesClaiming: "Medium",
	NakedSubset:              "Medium",
	HiddenSubset:             "Medium",
	XWing:                    "Hard",
	Swordfish:                "Hard",
	Jellyfish:                "Hard",
	XYWing:                   "Hard",
	XYZWing:                  "Expert",
	WWing:                    "Expert",
	SimpleColouring:          "Expert",
}

// ladderIndexOf returns the ladder position of t, or the last index if t is unknown (so an
// unrecognised cap enables the full ladder rather than silently disabling everything).
func ladderIndexOf(t Technique) int {
	for i := range ladderTechniques {
		if ladderTechniques[i].name == t {
			return i
		}
	}
	return len(ladderTechniques) - 1
}

// SolveWithMaxTechnique runs the same deterministic solve loop as Solve but enables ONLY the
// techniques at ladder positions 0..indexOf(max) inclusive. It underpins the floor (necessity)
// and ceiling (grade) tests: floor(T) caps at T's predecessor and must NOT solve; ceiling(T)
// caps at T and must solve.
func SolveWithMaxTechnique(g sudoku.Grid, max Technique) SolveResult {
	return runEngine(g, computeCandidates, ladderIndexOf(max))
}

// Grade returns the ADR-0013 difficulty band of g's solve — the band of the hardest technique
// the solve was forced to use — or "" if g does not solve under the full ladder.
func Grade(g sudoku.Grid) string {
	res := Solve(g)
	if res.Status != StatusSolved {
		return ""
	}
	return techBand[res.HardestTechnique]
}
