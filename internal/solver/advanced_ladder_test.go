package solver_test

// P-2 shared test scaffolding: the authoritative advanced-tier API contract the builder must
// implement, the labeled-fixture loader, and the advanced-tier replay proof. AC tests live in
// advanced_fixtures_test.go, grader_test.go, banned_techniques_test.go, status_coverage_test.go.
//
// ============================================================================================
// SOLVER API THE P-2 BUILDER MUST IMPLEMENT (these tests reference it; until it exists the
// internal/solver test binary is COMPILE-RED — the sanctioned red for this phase. internal/sudoku
// and internal/api are unaffected and stay green. The P-1 solver tests are unmodified and go
// green again the moment these symbols exist.)
//
//	package solver
//
//	// Technique is a canonical technique name. Its string value IS the Event.Technique string.
//	type Technique string
//	const (
//	    NakedSingle              Technique = "naked_single"
//	    HiddenSingle             Technique = "hidden_single"
//	    LockedCandidatesPointing Technique = "locked_candidates_pointing"
//	    LockedCandidatesClaiming Technique = "locked_candidates_claiming"
//	    NakedSubset              Technique = "naked_subset"
//	    HiddenSubset             Technique = "hidden_subset"
//	    XWing                    Technique = "x_wing"
//	    Swordfish                Technique = "swordfish"
//	    Jellyfish                Technique = "jellyfish"
//	    XYWing                   Technique = "xy_wing"
//	    XYZWing                  Technique = "xyz_wing"
//	    WWing                    Technique = "w_wing"
//	    SimpleColouring          Technique = "simple_colouring"
//	)
//
//	// Ladder is the ADR-0002 technique ladder, cheapest-first (== expectedLadder below).
//	var Ladder []Technique
//
//	// SolveWithMaxTechnique runs the SAME deterministic solve loop as Solve, but enables ONLY the
//	// techniques at ladder positions 0..indexOf(max) inclusive; every technique above max is
//	// disabled. It underpins the floor (necessity) and ceiling (grade) tests:
//	//   floor(T):   SolveWithMaxTechnique(g, predecessor(T)) must NOT be solved  (T is required)
//	//   ceiling(T): SolveWithMaxTechnique(g, T)              must be solved       (T suffices)
//	func SolveWithMaxTechnique(g sudoku.Grid, max Technique) SolveResult
//
//	// Grade returns the difficulty band of g's solve ("Easy"|"Medium"|"Hard"|"Expert"), or ""
//	// if g does not solve. band = bandOf[hardest technique the solve was forced to use] (ADR-0013).
//	func Grade(g sudoku.Grid) string
//
//	// SolveResult gains one field: the hardest technique that fired during the solve (highest
//	// ladder index among Events), or "" if none fired. Feeds api BatchItem.hardestTechnique.
//	//    HardestTechnique Technique
//
// Every advanced technique (ladder index >= 2) emits its effect as ELIMINATIONS (Event.Eliminations);
// only the singles (index 0,1) ever PLACE. Placements always remain naked/hidden singles — advanced
// techniques progress the grid by removing candidates, after which a single fires (ADR-0001).
// ============================================================================================

import (
	"math/bits"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
)

// expectedLadder is the ADR-0002 ladder in cheapest-first order. It is the authoritative
// technique vocabulary these tests enforce; solver.Ladder must stringify to exactly this
// (asserted by TestAC3_Solver_LadderMatchesADR0002).
var expectedLadder = []string{
	"naked_single",
	"hidden_single",
	"locked_candidates_pointing",
	"locked_candidates_claiming",
	"naked_subset",
	"hidden_subset",
	"x_wing",
	"swordfish",
	"jellyfish",
	"xy_wing",
	"xyz_wing",
	"w_wing",
	"simple_colouring",
}

// bandOf maps each technique to its difficulty band (ADR-0013, Sudoku-Explainer-style buckets):
// Easy = singles; Medium = locked candidates + subsets; Hard = basic fish + xy-wing;
// Expert = xyz-wing / w-wing / simple colouring.
var bandOf = map[string]string{
	"naked_single":               "Easy",
	"hidden_single":              "Easy",
	"locked_candidates_pointing": "Medium",
	"locked_candidates_claiming": "Medium",
	"naked_subset":               "Medium",
	"hidden_subset":              "Medium",
	"x_wing":                     "Hard",
	"swordfish":                  "Hard",
	"jellyfish":                  "Hard",
	"xy_wing":                    "Hard",
	"xyz_wing":                   "Expert",
	"w_wing":                     "Expert",
	"simple_colouring":           "Expert",
}

// bandRank orders the four ADR-0013 bands cheapest-first, so an un-isolable fixture's overall
// grade can be asserted "no cheaper than the firing technique's band" (a higher ceiling is fine).
var bandRank = map[string]int{"Easy": 0, "Medium": 1, "Hard": 2, "Expert": 3}

// advTechniques is the ADR-0018 gate vocabulary: every shipped technique above naked_single (which
// the seed set covers). Each must be covered by EITHER the isolable OR the un-isolable tier.
var advTechniques = []string{
	"hidden_single", "locked_candidates_pointing", "locked_candidates_claiming",
	"naked_subset", "hidden_subset", "x_wing", "swordfish", "jellyfish",
	"xy_wing", "xyz_wing", "w_wing", "simple_colouring",
}

// tierIndex is a technique's position on the ladder (0-based); -1 if unknown.
func tierIndex(tech string) int {
	for i, t := range expectedLadder {
		if t == tech {
			return i
		}
	}
	return -1
}

// ADR-0018 two-tier ship gate: each fixture carries the tier its label is proven under.
const (
	tierIsolable   = "isolable"   // technique is the EXACT hardest step (floor + ceiling)
	tierUnisolable = "unisolable" // technique FIRES and every elimination is replay-sound
)

// fixture is one labeled advanced puzzle from testdata/advanced/fixtures.txt.
type fixture struct {
	technique string // canonical Event.Technique this fixture proves (the label)
	tier      string // ADR-0018 tier: tierIsolable | tierUnisolable
	band      string // ADR-0013 band of `technique` (bandOf[technique])
	puzzle    string // 81-char givens
	solution  string // 81-char oracle solution (unique)
	source    string // provenance
}

// loadAdvancedFixtures reads and parses testdata/advanced/fixtures.txt (pipe-delimited, '#'
// comments and blank lines skipped). ADR-0018 added the `tier` column (6 fields).
func loadAdvancedFixtures(t *testing.T) []fixture {
	t.Helper()
	return parsePipeFile(t, filepath.Join("..", "..", "testdata", "advanced", "fixtures.txt"),
		6, func(f []string) fixture {
			return fixture{technique: f[0], tier: f[1], band: f[2], puzzle: f[3], solution: f[4], source: f[5]}
		})
}

// statusFixture is one row of testdata/advanced/status.txt.
type statusFixture struct {
	category string
	status   string
	puzzle   string
	note     string
}

// loadStatusFixtures reads testdata/advanced/status.txt (AC-5 parseable categories).
func loadStatusFixtures(t *testing.T) []statusFixture {
	t.Helper()
	return parsePipeFile(t, filepath.Join("..", "..", "testdata", "advanced", "status.txt"),
		4, func(f []string) statusFixture {
			return statusFixture{category: f[0], status: f[1], puzzle: f[2], note: f[3]}
		})
}

// parsePipeFile reads a '#'-commented, pipe-delimited fixture file into typed rows.
func parsePipeFile[T any](t *testing.T, path string, cols int, mk func([]string) T) []T {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture file %s: %v", path, err)
	}
	var out []T
	for n, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) != cols {
			t.Fatalf("%s line %d: got %d fields, want %d: %q", path, n+1, len(f), cols, line)
		}
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		out = append(out, mk(f))
	}
	return out
}

// --- advanced-tier replay proof (AC-2) ------------------------------------------------------

// advUnits holds the 27 units (rows, cols, boxes) as cell-index lists, for the model-based
// hidden-single check in the replay proof. Built once.
var advUnits = func() [27][9]int {
	var u [27][9]int
	for i := 0; i < 9; i++ {
		for k := 0; k < 9; k++ {
			u[i][k] = i*9 + k
			u[9+i][k] = k*9 + i
			br, bc := (i/3)*3, (i%3)*3
			u[18+i][k] = (br+k/3)*9 + (bc + k%3)
		}
	}
	return u
}()

// basicCand returns the basic candidate bitset (bit d set iff digit d is legal) for every empty
// cell of board; a filled cell gets 0. Reuses legal() from oracle_test.go.
func basicCand(board [81]int) [81]uint16 {
	var cand [81]uint16
	for i := 0; i < 81; i++ {
		if board[i] != 0 {
			continue
		}
		for d := 1; d <= 9; d++ {
			if legal(board, i, d) {
				cand[i] |= uint16(1) << d
			}
		}
	}
	return cand
}

// replayAdvancedProvesForced is the AC-2 (UC-2) load-bearing proof at the advanced tier. It
// replays res.Events from the original input against a CANDIDATE MODEL (basic candidates minus
// every elimination the log has recorded so far), and asserts:
//
//   - every step is named + witnessed (else it is a hidden guess);
//   - every PLACEMENT is a naked/hidden single *under the current model* — advanced techniques
//     never place, so a placement tagged with anything but naked_single/hidden_single fails;
//   - every ELIMINATION is tagged with an advanced technique (ladder index >= 2), targets an
//     empty cell, and is SOUND: the eliminated digit is FALSE in the unique oracle solution
//     (removing a TRUE candidate would be a wrong deduction — a guess/bug — and fails here);
//   - each recorded GridAfter matches the replay, the final grid equals res.Solution AND the
//     oracle solution, and no filled cell is left unexplained.
//
// Design note (surfaced, not hidden): this proves placements are forced singles under the exact
// candidate state the recorded eliminations produce, and that no elimination is ever wrong. It
// does NOT re-derive each elimination's specific pattern (e.g. that an "x_wing"-tagged
// elimination really has an X-wing) — that per-technique witness is the builder's responsibility,
// triangulated by the floor (necessity) + ceiling tests, which prove the tagged technique is both
// required and sufficient at its tier. Extend the switch arms below with pattern verifiers if a
// stronger per-technique witness is later wanted.
func replayAdvancedProvesForced(t *testing.T, input, oracle string, res solver.SolveResult) (placements, eliminations int) {
	t.Helper()
	board := parseBoard(input)
	given := board
	placed := make(map[int]bool)
	var elim [81]uint16 // cumulative recorded eliminations (bit d)
	wantSeq := 1

	for _, ev := range res.Events {
		if ev.Seq != wantSeq {
			t.Fatalf("event seq gap: got %d want %d", ev.Seq, wantSeq)
		}
		wantSeq++
		if ev.Technique == "" || len(ev.WitnessCells) == 0 {
			t.Fatalf("seq %d: unnamed or unwitnessed step (a hidden guess)", ev.Seq)
		}
		if tierIndex(ev.Technique) < 0 {
			t.Fatalf("seq %d: technique %q is not on the ADR-0002 ladder", ev.Seq, ev.Technique)
		}

		// Candidate model = basic candidates (current board) minus everything eliminated so far.
		cand := basicCand(board)
		for i := 0; i < 81; i++ {
			cand[i] &^= elim[i]
		}

		if ev.Placement != nil {
			idx := ev.Placement.Cell.Row*9 + ev.Placement.Cell.Col
			val := ev.Placement.Value
			if idx < 0 || idx >= 81 {
				t.Fatalf("seq %d: placement cell out of range: %+v", ev.Seq, ev.Placement.Cell)
			}
			if board[idx] != 0 {
				t.Fatalf("seq %d: placement into non-empty cell %d", ev.Seq, idx)
			}
			if val < 1 || val > 9 || !legal(board, idx, val) {
				t.Fatalf("seq %d: illegal placement %d at cell %d", ev.Seq, val, idx)
			}
			switch ev.Technique {
			case "naked_single":
				if !modelNakedForced(cand, idx, val) {
					t.Fatalf("seq %d: labeled naked_single but %d is not the sole model-candidate at cell %d (hidden guess)", ev.Seq, val, idx)
				}
			case "hidden_single":
				if !modelHiddenForced(board, cand, idx, val) {
					t.Fatalf("seq %d: labeled hidden_single but %d is not a hidden single at cell %d under the model (hidden guess)", ev.Seq, val, idx)
				}
			default:
				t.Fatalf("seq %d: placement tagged %q, but only singles may place — advanced techniques eliminate (ADR-0001)", ev.Seq, ev.Technique)
			}
			board[idx] = val
			placed[idx] = true
			placements++
		}

		for _, el := range ev.Eliminations {
			ecell := el.Cell.Row*9 + el.Cell.Col
			if ecell < 0 || ecell >= 81 {
				t.Fatalf("seq %d: elimination cell out of range: %+v", ev.Seq, el.Cell)
			}
			if board[ecell] != 0 {
				t.Fatalf("seq %d: elimination from filled cell %d", ev.Seq, ecell)
			}
			if tierIndex(ev.Technique) < 2 {
				t.Fatalf("seq %d: %q is a singles technique but recorded an elimination (singles only place)", ev.Seq, ev.Technique)
			}
			if el.Candidate < 1 || el.Candidate > 9 {
				t.Fatalf("seq %d: elimination candidate %d out of range at cell %d", ev.Seq, el.Candidate, ecell)
			}
			// SOUNDNESS: the eliminated digit must be FALSE in the unique solution.
			if oracle[ecell]-'0' == byte(el.Candidate) {
				t.Fatalf("seq %d: %q eliminated candidate %d at cell %d, but that digit IS in the unique solution (a wrong deduction / guess)", ev.Seq, ev.Technique, el.Candidate, ecell)
			}
			elim[ecell] |= uint16(1) << el.Candidate
		}
		if len(ev.Eliminations) > 0 {
			eliminations += len(ev.Eliminations)
		}

		if got := boardString(board); got != ev.GridAfter {
			t.Fatalf("seq %d: gridAfter mismatch\n replay=%q\n event =%q", ev.Seq, got, ev.GridAfter)
		}
	}

	if res.Status == solver.StatusSolved {
		if got := boardString(board); got != res.Solution {
			t.Fatalf("replayed grid != returned solution\n replay=%q\n sol   =%q", got, res.Solution)
		}
		if res.Solution != oracle {
			t.Fatalf("returned solution != oracle\n sol   =%q\n oracle=%q", res.Solution, oracle)
		}
	}
	for i := 0; i < 81; i++ {
		blankAtStart, filledAtEnd := given[i] == 0, board[i] != 0
		if blankAtStart && filledAtEnd && !placed[i] {
			t.Fatalf("cell %d is filled but no placement event explains it (hidden guess)", i)
		}
		if !blankAtStart && placed[i] {
			t.Fatalf("given cell %d was re-placed by an event", i)
		}
	}
	return placements, eliminations
}

// modelNakedForced reports whether val is the ONLY candidate at cell under the model cand set.
func modelNakedForced(cand [81]uint16, cell, val int) bool {
	return bits.OnesCount16(cand[cell]) == 1 && cand[cell] == uint16(1)<<val
}

// modelHiddenForced reports whether val is a candidate at cell AND cell is the only cell in at
// least one of its three units that still holds val, under the model cand set.
func modelHiddenForced(board [81]int, cand [81]uint16, cell, val int) bool {
	bit := uint16(1) << val
	if cand[cell]&bit == 0 {
		return false
	}
	r, c := cell/9, cell%9
	unitsOfCell := [3]int{r, 9 + c, 18 + (r/3)*3 + c/3}
	for _, u := range unitsOfCell {
		cnt := 0
		for _, idx := range advUnits[u] {
			if board[idx] == 0 && cand[idx]&bit != 0 {
				cnt++
			}
		}
		if cnt == 1 {
			return true
		}
	}
	return false
}
