package generator

// AC-3 (DESIGN_DECISIONS §ADR-0003) — the CRITICAL import-boundary assertion.
//
// ADR-0003 rules that generation MAY backtrack (its uniqueness solution-counter) but the
// benchmarked solver MAY NOT, and — load-bearing — the backtracking counter must NEVER be
// importable from internal/solver. POST /v1/solve invokes only solver.Solve; therefore, if
// internal/generator is absent from the SOLVER's transitive dependency set, no
// backtracking/solution-counting code path is reachable from a solve request. That is exactly
// what this test proves, mechanically, via `go list -deps`.

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAC3_SolverDoesNotImportGenerator(t *testing.T) {
	const (
		solverPkg    = "github.com/scottbushyhead/sudoku-flow/internal/solver"
		generatorPkg = "github.com/scottbushyhead/sudoku-flow/internal/generator"
		sudokuPkg    = "github.com/scottbushyhead/sudoku-flow/internal/sudoku"
	)

	// `go list -deps X` prints the full transitive import graph of X (X itself included), one
	// canonical import path per line — module-style forward-slash paths, so this is
	// OS-independent. Run in the module (cwd is inside it under `go test`).
	out, err := exec.Command("go", "list", "-deps", solverPkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s failed: %v\n%s", solverPkg, err, out)
	}

	deps := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			deps[line] = true
		}
	}

	// Positive control: a real dependency graph MUST contain internal/sudoku. This guards
	// against a vacuous pass from empty/garbled `go list` output (which would otherwise make
	// the generator-absent assertion trivially true).
	if !deps[sudokuPkg] {
		t.Fatalf("positive control failed: %s not found in solver deps — `go list` output suspect:\n%s", sudokuPkg, out)
	}

	// The actual boundary assertion (ADR-0003): the generation-path backtracking counter is
	// unreachable from the benchmarked solve path.
	if deps[generatorPkg] {
		t.Fatalf("BOUNDARY VIOLATION (ADR-0003): %s is in the transitive deps of %s — the "+
			"generation-path backtracking/solution-counting code is reachable from the solver, "+
			"and therefore from POST /v1/solve", generatorPkg, solverPkg)
	}
}
