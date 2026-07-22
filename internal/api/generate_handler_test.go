package api_test

// P-3 HTTP-edge tests for POST /v1/generate (RED until api.GenerateHandler is implemented).
//
// Test-defined source surface the builder implements:
//
//	// GenerateHandler returns the POST /v1/generate handler. It enforces application/json
//	// content-type BEFORE reading the body (F-12: 415 otherwise), decodes GenerateRequest,
//	// validates the difficulty enum against {easy,medium,hard,expert} (F-14: unknown value →
//	// HTTP 400 + ErrorResponse{Code:"invalid_input"}, never default-and-proceed), calls the
//	// generator, and returns GeneratedPuzzle{puzzle, difficulty, grade} (HTTP 200). The
//	// internal backtracking uniqueness counter is never surfaced (ARCHITECTURE §Generate).
//	func GenerateHandler() http.Handler
//
// These tests are black-box against the wire contract (contract.go GenerateRequest /
// GeneratedPuzzle / ErrorResponse) and do not import internal/generator.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

func postGenerate(t *testing.T, h http.Handler, difficulty, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(api.GenerateRequest{Difficulty: difficulty})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/generate", bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// --- independent brute-force solution counter (TEST CODE ONLY) ----------------------------
//
// Same role as the generator-package copy: an oracle for uniqueness that does NOT import the
// code under test and does NOT touch internal/solver. Counts up to cap solutions; -1 for a
// non-legal grid string.

func bruteForceSolutionCount(puzzle string, cap int) int {
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
		best := -1
		var bestCands []int
		for i := 0; i < 81; i++ {
			if cells[i] != 0 {
				continue
			}
			cands := legalDigits(&cells, i)
			if len(cands) == 0 {
				return
			}
			if best == -1 || len(cands) < len(bestCands) {
				best, bestCands = i, cands
			}
		}
		if best == -1 {
			count++
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

func legalDigits(cells *[81]int, idx int) []int {
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

// AC-1 (EVAL §Eval matrix → UC-3): POST /v1/generate returns HTTP 200 with a GeneratedPuzzle
// whose puzzle is valid (rule-valid givens) and has EXACTLY ONE solution, confirmed by the
// independent brute-force counter (cap 2, asserts == 1) — not the generator's own counter.
func TestAC1_Handler_GeneratedPuzzleIsUnique(t *testing.T) {
	h := api.GenerateHandler()
	for _, band := range []string{"easy", "medium", "hard", "expert"} {
		rr := postGenerate(t, h, band, "application/json")
		if rr.Code != http.StatusOK {
			t.Fatalf("band=%s: got HTTP %d, want 200 (body=%s)", band, rr.Code, rr.Body.String())
		}
		var gp api.GeneratedPuzzle
		if err := json.Unmarshal(rr.Body.Bytes(), &gp); err != nil {
			t.Fatalf("band=%s: body not a GeneratedPuzzle: %v (raw=%s)", band, err, rr.Body.String())
		}
		if len(gp.Puzzle) != 81 {
			t.Fatalf("band=%s: puzzle length = %d, want 81 (%q)", band, len(gp.Puzzle), gp.Puzzle)
		}
		if gp.Difficulty == "" || gp.Grade == "" {
			t.Fatalf("band=%s: difficulty and grade must be populated, got %+v", band, gp)
		}
		if _, err := sudoku.Parse(gp.Puzzle); err != nil {
			t.Fatalf("band=%s: generated puzzle is not rule-valid: %v (%q)", band, err, gp.Puzzle)
		}
		if n := bruteForceSolutionCount(gp.Puzzle, 2); n != 1 {
			t.Fatalf("band=%s: independent solution count = %d, want EXACTLY 1 (puzzle=%q)", band, n, gp.Puzzle)
		}
	}
}

// AC-4 (SECURITY §F-14): POST /v1/generate with an unknown difficulty is rejected with a typed
// invalid_input — HTTP 400 + ErrorResponse{Code:"invalid_input"} — NOT defaulted-and-proceeded.
// The body is valid JSON with a valid content-type, so the enum check is the sole reason for 400.
func TestAC4_Handler_UnknownDifficultyIsInvalidInput(t *testing.T) {
	h := api.GenerateHandler()
	for _, bad := range []string{"bogus", "extreme", "easyy", "0", "hardest", ""} {
		rr := postGenerate(t, h, bad, "application/json")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("difficulty %q: got HTTP %d, want 400 (F-14 typed invalid_input) body=%s", bad, rr.Code, rr.Body.String())
		}
		var env api.ErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
			t.Fatalf("difficulty %q: body not an ErrorResponse: %v", bad, err)
		}
		if env.Code != "invalid_input" {
			t.Fatalf("difficulty %q: code = %q, want invalid_input", bad, env.Code)
		}
	}
}

// AC-5 (SECURITY §F-12): POST /v1/generate with a non-application/json Content-Type is rejected
// with HTTP 415, before the body is interpreted. Difficulty "hard" is valid, so content-type is
// the only possible ground for rejection.
func TestAC5_Handler_RejectsNonJSONContentType(t *testing.T) {
	h := api.GenerateHandler()
	if rr := postGenerate(t, h, "hard", "text/plain"); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain content-type: got HTTP %d, want 415 (F-12)", rr.Code)
	}
	if rr := postGenerate(t, h, "hard", ""); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content-type: got HTTP %d, want 415 (F-12)", rr.Code)
	}
}
