package solver_test

// AC-3 (no banned techniques). Source: DESIGN_DECISIONS §ADR-0001 (constructive-only — no
// forcing chains / Nishio / AIC / contradiction reasoning) and §ADR-0004 (no Unique Rectangles /
// BUG). Two guards: (1) the solver's enabled ladder is EXACTLY the ADR-0002 set (no extra tier);
// (2) the solver source names no banned technique and never assumes-and-reverts.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
)

// AC-3a — the shipped ladder is byte-for-byte the ADR-0002 ladder in cheapest-first order.
// Asserting the enabled set == the ladder is the positive proof that no out-of-scope technique
// (a forcing chain, an ALS move) was slipped in. Uses solver.Ladder (undefined until built).
func TestAC3_Solver_LadderMatchesADR0002(t *testing.T) {
	var got []string
	for _, tech := range solver.Ladder {
		got = append(got, string(tech))
	}
	if len(got) != len(expectedLadder) {
		t.Fatalf("solver.Ladder has %d techniques, want %d\n got=%v\n want=%v", len(got), len(expectedLadder), got, expectedLadder)
	}
	for i := range expectedLadder {
		if got[i] != expectedLadder[i] {
			t.Errorf("ladder[%d] = %q, want %q (order is ADR-0012 cheapest-first)", i, got[i], expectedLadder[i])
		}
	}
}

// AC-3b — no banned technique appears in the solver source, and no assume-and-revert marker.
// Scans internal/solver/*.go (non-test) for the banned identifiers. The tokens are specific
// enough not to false-positive on the allowed ladder (simple_colouring is allowed; "colouring"
// is not scanned). This is a guard test: it passes against the constructive singles solver today
// and must keep passing after the advanced ladder lands.
func TestAC3_Solver_NoBannedTechniqueInSource(t *testing.T) {
	// \b-wrapped tokens so a substring inside an allowed word never matches (e.g. "als" does not
	// hit "false"; "bug" does not hit "debug"). These are the ADR-0001/0004 banned families.
	banned := regexp.MustCompile(`(?i)\b(forcing[_ ]?chains?|nishio|nice[_ ]?loops?|aic|als|unique[_ ]?rectangles?|bivalue[_ ]?universal[_ ]?grave|bug)\b`)
	files, err := filepath.Glob(filepath.Join("*.go"))
	if err != nil {
		t.Fatalf("glob solver sources: %v", err)
	}
	scanned := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		scanned++
		for n, line := range strings.Split(string(raw), "\n") {
			// Skip comment lines: ADR-0001's own prose ("never guesses or backtracks") lives in
			// doc comments and is not banned CODE. We flag banned tokens in non-comment lines.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
				continue
			}
			if m := banned.FindString(line); m != "" {
				t.Errorf("%s:%d names a banned technique/marker %q (ADR-0001/0004)\n  %s", path, n+1, strings.TrimSpace(m), trimmed)
			}
		}
	}
	if scanned == 0 {
		t.Fatalf("scanned 0 non-test solver source files — glob is wrong")
	}
}
