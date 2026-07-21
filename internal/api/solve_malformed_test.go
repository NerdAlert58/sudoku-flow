package api_test

// P-1 HTTP-edge coverage backfill (test-only), jasnah non-blocking note: the body-decode
// error path in SolveHandler (solve.go: json.NewDecoder(...).Decode -> 400 invalid_input). It
// is DISTINCT from AC-5's bad-givens path: there the body is valid JSON and sudoku.Parse
// rejects the puzzle; here the body itself is not valid JSON, so the request fails before the
// puzzle is ever read. The existing postSolve helper always marshals a valid SolveRequest, so
// it cannot reach this branch — this test posts a raw malformed body.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// A POST /v1/solve with Content-Type application/json but a body that is not valid JSON is a
// 400 + ErrorResponse{Code:"invalid_input"} — the content-type gate (F-12) passes, then the
// JSON decode fails.
func TestP1_Handler_MalformedJSONBodyIsBadRequest(t *testing.T) {
	h := api.SolveHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/solve", strings.NewReader(`{"puzzle": `)) // truncated JSON
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON body: got HTTP %d, want 400 (body=%s)", rr.Code, rr.Body.String())
	}
	var env api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not an ErrorResponse: %v", err)
	}
	if env.Code != "invalid_input" {
		t.Fatalf("got code=%q, want invalid_input", env.Code)
	}
}
