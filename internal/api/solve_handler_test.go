package api_test

// P-1 HTTP-edge tests for POST /v1/solve. These drive the test-facing handler surface the
// builder implements:
//
//	// SolveHandler returns the POST /v1/solve handler. It parses SolveRequest.Puzzle via
//	// sudoku.Parse, calls solver.Solve, measures solveTimeMs around the Solve call (P3/ADR-0007),
//	// and serializes a SolveResponse. Content-Type must be application/json (F-12) else 415.
//	// A sudoku.Parse failure (bad length/char/duplicate given) is rejected BEFORE the solver
//	// with the {error, code} envelope: HTTP 400 + ErrorResponse{Code:"invalid_input"} (ADR-0011,
//	// ARCHITECTURE §Summary: "Malformed input is rejected with a typed {error, code} envelope").
//	func SolveHandler() http.Handler
//
// CONTRACT NOTE (surfaced, not averaged): ADR-0011 lists invalid_input among the four success
// statuses, but ARCHITECTURE §Summary + §Components say malformed input is rejected via the
// {error, code} FAILURE envelope (and contract.go's ErrorResponse comment: "statuses live on
// the success path; this envelope is the failure path"). These tests therefore assert
// invalid_input on the ErrorResponse.Code with HTTP 400, not on a 200 SolveResponse.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

func loadSeed(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("..", "..", "puzzles.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading seed puzzles at %s: %v", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// constraints27 reports whether a COMPLETE 81-char grid satisfies all 27 unit constraints.
// A completed, constraint-valid grid that preserves a uniquely-solvable puzzle's givens is
// necessarily that puzzle's unique solution — so this is oracle-equivalent at the wire without
// importing the oracle (the explicit oracle equality lives in internal/solver).
func constraints27(s string) bool {
	if len(s) != 81 {
		return false
	}
	unit := func(get func(k int) byte) bool {
		var seen [10]bool
		for k := 0; k < 9; k++ {
			c := get(k)
			if c < '1' || c > '9' || seen[c-'0'] {
				return false
			}
			seen[c-'0'] = true
		}
		return true
	}
	for i := 0; i < 9; i++ {
		row, col, box := i, i, i
		br, bc := (box/3)*3, (box%3)*3
		if !unit(func(k int) byte { return s[row*9+k] }) ||
			!unit(func(k int) byte { return s[k*9+col] }) ||
			!unit(func(k int) byte { return s[(br+k/3)*9+(bc+k%3)] }) {
			return false
		}
	}
	return true
}

func givensPreserved(input, sol string) bool {
	for i := 0; i < 81 && i < len(input) && i < len(sol); i++ {
		if c := input[i]; c >= '1' && c <= '9' && sol[i] != c {
			return false
		}
	}
	return true
}

func postSolve(t *testing.T, h http.Handler, puzzle, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(api.SolveRequest{Puzzle: puzzle})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/solve", bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// AC-1 (EVAL §Eval matrix → UC-1): POST /v1/solve with each of the 25 seed grids returns
// status "solved", solved=true, and a complete solution satisfying all 27 constraints and
// preserving the givens (⇒ the unique oracle solution, since D-Q2 puzzles are unique).
func TestAC1_Handler_SolvesAll25ViaPostSolve(t *testing.T) {
	h := api.SolveHandler()
	seed := loadSeed(t)
	if len(seed) != 25 {
		t.Fatalf("expected 25 seed puzzles, got %d", len(seed))
	}
	for n, line := range seed {
		rr := postSolve(t, h, line, "application/json")
		if rr.Code != http.StatusOK {
			t.Fatalf("puzzle %d: got HTTP %d, want 200 (body=%s)", n+1, rr.Code, rr.Body.String())
		}
		var resp api.SolveResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("puzzle %d: body not a SolveResponse: %v", n+1, err)
		}
		if resp.Status != "solved" || !resp.Solved {
			t.Fatalf("puzzle %d: got status=%q solved=%v, want solved/true", n+1, resp.Status, resp.Solved)
		}
		if len(resp.Solution) != 81 || strings.ContainsAny(resp.Solution, "0.") {
			t.Fatalf("puzzle %d: solution not complete: %q", n+1, resp.Solution)
		}
		if !constraints27(resp.Solution) {
			t.Fatalf("puzzle %d: solution violates the 27 constraints: %q", n+1, resp.Solution)
		}
		if !givensPreserved(line, resp.Solution) {
			t.Fatalf("puzzle %d: solution overwrites a given clue", n+1)
		}
		if resp.APIVersion != api.APIVersion {
			t.Fatalf("puzzle %d: apiVersion=%q, want %q", n+1, resp.APIVersion, api.APIVersion)
		}
	}
}

// AC-2 (ADR-0007): the solve response includes the full metric quartet, measured in-handler.
func TestAC2_Handler_ResponseIncludesMetricQuartet(t *testing.T) {
	h := api.SolveHandler()
	rr := postSolve(t, h, loadSeed(t)[0], "application/json")
	if rr.Code != http.StatusOK {
		t.Fatalf("got HTTP %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}

	// All four quartet keys must be PRESENT in the emitted JSON (frozen /v1 shape, ADR-0007).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body not JSON object: %v", err)
	}
	for _, k := range []string{"solveTimeMs", "eventCount", "iterations", "candidateChecks"} {
		if _, ok := raw[k]; !ok {
			t.Fatalf("solve response missing metric quartet key %q", k)
		}
	}

	var resp api.SolveResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not a SolveResponse: %v", err)
	}
	if resp.Iterations <= 0 {
		t.Fatalf("iterations must be > 0 (main-loop scan passes)")
	}
	if resp.CandidateChecks <= 0 {
		t.Fatalf("candidateChecks must be > 0 (candidate-cell inspections)")
	}
	if resp.EventCount != len(resp.Events) {
		t.Fatalf("eventCount %d != len(events) %d", resp.EventCount, len(resp.Events))
	}
	if resp.EventCount <= 0 {
		t.Fatalf("eventCount must be > 0 for a solved puzzle")
	}
	// solveTimeMs is measured in-handler around solver.Solve (P3): present, non-negative, and
	// not implausibly large for a sub-millisecond 9x9 solve.
	if resp.SolveTimeMs < 0 {
		t.Fatalf("solveTimeMs must be >= 0, got %v", resp.SolveTimeMs)
	}
	if resp.SolveTimeMs > 5000 {
		t.Fatalf("solveTimeMs implausibly large (%v ms) — transport likely included, must be solve-only", resp.SolveTimeMs)
	}
}

// AC-4 (ADR-0012): the same puzzle POSTed twice yields a byte-identical solution, event log,
// and counter metrics. solveTimeMs is excluded from byte-identity (wall-clock per ADR-0007;
// see the reconciliation in internal/solver's determinism test).
func TestAC4_Handler_DeterministicAcrossRepeatedPosts(t *testing.T) {
	h := api.SolveHandler()
	line := loadSeed(t)[0]

	var a, b api.SolveResponse
	if err := json.Unmarshal(postSolve(t, h, line, "application/json").Body.Bytes(), &a); err != nil {
		t.Fatalf("decode first: %v", err)
	}
	if err := json.Unmarshal(postSolve(t, h, line, "application/json").Body.Bytes(), &b); err != nil {
		t.Fatalf("decode second: %v", err)
	}

	if a.Solution != b.Solution {
		t.Fatalf("solution not deterministic:\n a=%q\n b=%q", a.Solution, b.Solution)
	}
	if a.Status != b.Status || a.EventCount != b.EventCount || a.Iterations != b.Iterations || a.CandidateChecks != b.CandidateChecks {
		t.Fatalf("status/counter metrics not deterministic")
	}
	ja, _ := json.Marshal(a.Events)
	jb, _ := json.Marshal(b.Events)
	if !bytes.Equal(ja, jb) {
		t.Fatalf("event log not byte-identical across identical POSTs")
	}
}

// AC-5 invalid_input (ADR-0011, ARCHITECTURE §Summary): malformed or rule-violating givens are
// rejected upstream of the solver with HTTP 400 + ErrorResponse{Code:"invalid_input"}.
func TestAC5_Handler_InvalidInputForMalformedGivens(t *testing.T) {
	h := api.SolveHandler()
	cases := map[string]string{
		"too short":       "123",
		"bad character":   strings.Repeat("0", 80) + "x",
		"duplicate given": "11" + strings.Repeat("0", 79), // two 1s in row 0
	}
	for name, puzzle := range cases {
		rr := postSolve(t, h, puzzle, "application/json")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: got HTTP %d, want 400 (body=%s)", name, rr.Code, rr.Body.String())
		}
		var env api.ErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s: body not an ErrorResponse: %v", name, err)
		}
		if env.Code != "invalid_input" {
			t.Fatalf("%s: got code=%q, want invalid_input", name, env.Code)
		}
	}
}

// AC-6 content-type (SECURITY §F-12): POST /v1/solve with a non-application/json Content-Type
// is rejected with HTTP 415.
func TestAC6_Handler_RejectsNonJSONContentType(t *testing.T) {
	h := api.SolveHandler()
	line := loadSeed(t)[0]

	if rr := postSolve(t, h, line, "text/plain"); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain content-type: got HTTP %d, want 415", rr.Code)
	}
	// A missing Content-Type is likewise not application/json → 415 (strict allowlist, F-12).
	if rr := postSolve(t, h, line, ""); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content-type: got HTTP %d, want 415", rr.Code)
	}
}
