package api_test

// P-4 HTTP-edge tests for POST /v1/validate-batch (RED until the builder implements
// api.BatchHandler and api.MaxBatchPuzzles).
//
// Test-defined SOURCE SURFACE the builder implements (adjust nothing in contract.go —
// BatchRequest / BatchItem / BatchResult already exist there):
//
//	// BatchHandler returns the POST /v1/validate-batch handler. It:
//	//   * enforces application/json content-type BEFORE reading the body (F-12: 415 otherwise);
//	//   * wraps the body in http.MaxBytesReader and caps len(Puzzles) at MaxBatchPuzzles — an
//	//     over-cap request (body bytes OR list length) is rejected with HTTP 413 +
//	//     ErrorResponse{Code:"invalid_input"} BEFORE any solving begins (ARCHITECTURE §Batch);
//	//   * CRLF-normalises each puzzle string (trims a trailing CR / surrounding whitespace) so a
//	//     CRLF-dirty or missing-final-newline source does not mis-parse the last puzzle (AUDIT D-Q1);
//	//   * gives EACH puzzle its OWN sudoku.Grid copy solved in its OWN goroutine (goroutine-per-
//	//     puzzle, zero shared mutable state → race-free, ADR-0006), collecting results in INPUT
//	//     ORDER, and returns BatchResult{results, solvedCount, total} (HTTP 200).
//	func BatchHandler() http.Handler
//
//	// MaxBatchPuzzles is the maximum len(BatchRequest.Puzzles) the batch endpoint accepts; a
//	// longer list is rejected 413/invalid_input before any solving (ARCHITECTURE §Batch).
//	const MaxBatchPuzzles = <builder-chosen positive int>
//
// These tests reuse the package-level helpers already defined in solve_handler_test.go
// (loadSeed, postSolve) — they are NOT redefined here.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// postBatch marshals a BatchRequest and drives BatchHandler through httptest.
func postBatch(t *testing.T, h http.Handler, puzzles []string, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(api.BatchRequest{Puzzles: puzzles})
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/validate-batch", bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// solveResponseFor drives the single POST /v1/solve handler for one puzzle so the batch item
// can be compared against that puzzle's genuine single-solve wire result.
func solveResponseFor(t *testing.T, puzzle string) api.SolveResponse {
	t.Helper()
	rr := postSolve(t, api.SolveHandler(), puzzle, "application/json")
	if rr.Code != http.StatusOK {
		t.Fatalf("single /v1/solve for %q: HTTP %d (body=%s)", puzzle, rr.Code, rr.Body.String())
	}
	var resp api.SolveResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("single /v1/solve body not a SolveResponse: %v", err)
	}
	return resp
}

// AC-1 (EVAL §Eval matrix → UC-4): POST /v1/validate-batch with the 25 puzzles.txt grids
// (CRLF-safe read via loadSeed) returns solvedCount:25, total:25, and each item's result equals
// that puzzle's single-/v1/solve result — Solved and Iterations byte-equal to the single-solve
// wire response, HardestTechnique equal to solver.Solve (the field the /v1/solve wire omits),
// Puzzle echoing the input line.
func TestAC1_Batch_SolvesAll25MatchingSingleSolve(t *testing.T) {
	h := api.BatchHandler()
	seed := loadSeed(t)
	if len(seed) != 25 {
		t.Fatalf("expected 25 seed puzzles, got %d", len(seed))
	}

	rr := postBatch(t, h, seed, "application/json")
	if rr.Code != http.StatusOK {
		t.Fatalf("got HTTP %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	var res api.BatchResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("body not a BatchResult: %v (raw=%s)", err, rr.Body.String())
	}

	if res.APIVersion != api.APIVersion {
		t.Fatalf("apiVersion=%q, want %q", res.APIVersion, api.APIVersion)
	}
	if res.Total != 25 {
		t.Fatalf("total=%d, want 25", res.Total)
	}
	if res.SolvedCount != 25 {
		t.Fatalf("solvedCount=%d, want 25", res.SolvedCount)
	}
	if len(res.Results) != 25 {
		t.Fatalf("len(results)=%d, want 25", len(res.Results))
	}

	for i, line := range seed {
		item := res.Results[i]
		if item.Puzzle != line {
			t.Fatalf("item %d: puzzle=%q, want input line %q (order not preserved?)", i, item.Puzzle, line)
		}

		single := solveResponseFor(t, line)
		if item.Solved != single.Solved {
			t.Fatalf("item %d: batch solved=%v != single /v1/solve solved=%v", i, item.Solved, single.Solved)
		}
		if !item.Solved {
			t.Fatalf("item %d: puzzle %q not solved by batch", i, line)
		}
		if item.Iterations != single.Iterations {
			t.Fatalf("item %d: batch iterations=%d != single /v1/solve iterations=%d", i, item.Iterations, single.Iterations)
		}

		// HardestTechnique is not on the /v1/solve wire; the batch item must carry the solver's
		// hardest technique for this puzzle.
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("item %d: seed failed to parse: %v", i, err)
		}
		wantHardest := string(solver.Solve(grid).HardestTechnique)
		if item.HardestTechnique != wantHardest {
			t.Fatalf("item %d: hardestTechnique=%q, want %q", i, item.HardestTechnique, wantHardest)
		}
	}
}

// AC-2 (EVAL §Eval matrix → UC-5; ADR-0006): the goroutine-per-puzzle batch results are
// byte-identical to solving the same list SERIALLY, in input order. SolveTimeMs is excluded from
// byte-identity (wall-clock per ADR-0007). A single batch request already fans out one goroutine
// per puzzle, so this test also exercises the internal fan-out under `go test -race`.
func TestAC2_Batch_ParallelResultsEqualSerial(t *testing.T) {
	h := api.BatchHandler()
	seed := loadSeed(t)

	// Serial expectation: solve each puzzle in order on this goroutine.
	expected := make([]api.BatchItem, len(seed))
	solvedWant := 0
	for i, line := range seed {
		grid, err := sudoku.Parse(line)
		if err != nil {
			t.Fatalf("puzzle %d: parse: %v", i, err)
		}
		r := solver.Solve(grid)
		expected[i] = api.BatchItem{
			Puzzle:           line,
			Solved:           r.Solved,
			Iterations:       r.Iterations,
			HardestTechnique: string(r.HardestTechnique),
		}
		if r.Solved {
			solvedWant++
		}
	}

	rr := postBatch(t, h, seed, "application/json")
	if rr.Code != http.StatusOK {
		t.Fatalf("got HTTP %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	var res api.BatchResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("body not a BatchResult: %v", err)
	}
	if len(res.Results) != len(expected) {
		t.Fatalf("len(results)=%d, want %d", len(res.Results), len(expected))
	}
	if res.SolvedCount != solvedWant || res.Total != len(seed) {
		t.Fatalf("solvedCount/total = %d/%d, want %d/%d", res.SolvedCount, res.Total, solvedWant, len(seed))
	}

	for i := range expected {
		got := res.Results[i]
		got.SolveTimeMs = 0 // wall-clock, excluded from byte-identity (ADR-0007)
		wantJSON, _ := json.Marshal(expected[i])
		gotJSON, _ := json.Marshal(got)
		if !bytes.Equal(wantJSON, gotJSON) {
			t.Fatalf("item %d: parallel batch != serial\n serial  =%s\n parallel=%s", i, wantJSON, gotJSON)
		}
	}
}

// AC-2 (race, UC-5 / ADR-0006): fire many concurrent batch requests, each fanning out one
// goroutine per puzzle, to give `go test -race` maximum surface over any shared/global mutable
// state. Every response must be a consistent solvedCount:25/total:25 BatchResult. The assertion
// value is the -race gate the coordinator runs; the equality here is a liveness cross-check.
func TestAC2_Batch_RaceFreeUnderConcurrentRequests(t *testing.T) {
	h := api.BatchHandler()
	seed := loadSeed(t)
	body, err := json.Marshal(api.BatchRequest{Puzzles: seed})
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}

	const workers = 16
	type outcome struct {
		code        int
		solvedCount int
		total       int
	}
	outcomes := make([]outcome, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/validate-batch", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			o := outcome{code: rr.Code}
			var res api.BatchResult
			if json.Unmarshal(rr.Body.Bytes(), &res) == nil {
				o.solvedCount, o.total = res.SolvedCount, res.Total
			}
			outcomes[w] = o // each worker writes its own index — no shared write
		}(w)
	}
	wg.Wait()

	for w, o := range outcomes {
		if o.code != http.StatusOK {
			t.Fatalf("worker %d: HTTP %d, want 200", w, o.code)
		}
		if o.solvedCount != 25 || o.total != 25 {
			t.Fatalf("worker %d: solvedCount/total = %d/%d, want 25/25", w, o.solvedCount, o.total)
		}
	}
}

// AC-3 (AUDIT §D-Q1): the batch loader tolerates CRLF line endings and a missing final newline
// without mis-parsing the LAST puzzle. Every element but the last carries a trailing CR (the
// artifact of splitting a CRLF file on "\n"); the last element is clean (missing final newline).
// All 25 must still solve — the handler must CRLF-normalise each element before sudoku.Parse.
func TestAC3_Batch_CRLFSafeAndMissingFinalNewline(t *testing.T) {
	h := api.BatchHandler()
	seed := loadSeed(t)

	dirty := make([]string, len(seed))
	for i, line := range seed {
		if i == len(seed)-1 {
			dirty[i] = line // last puzzle: no trailing CR (missing final newline)
		} else {
			dirty[i] = line + "\r" // CRLF artifact
		}
	}

	rr := postBatch(t, h, dirty, "application/json")
	if rr.Code != http.StatusOK {
		t.Fatalf("got HTTP %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	var res api.BatchResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("body not a BatchResult: %v", err)
	}
	if res.Total != len(seed) || res.SolvedCount != len(seed) {
		t.Fatalf("CRLF batch: solvedCount/total = %d/%d, want %d/%d", res.SolvedCount, res.Total, len(seed), len(seed))
	}
	if len(res.Results) != len(seed) {
		t.Fatalf("len(results)=%d, want %d", len(res.Results), len(seed))
	}
	// The LAST puzzle specifically must not be dropped or mis-parsed.
	if last := res.Results[len(res.Results)-1]; !last.Solved {
		t.Fatalf("last puzzle mis-parsed under CRLF/missing-final-newline: %+v", last)
	}
	for i, item := range res.Results {
		if !item.Solved {
			t.Fatalf("item %d not solved under CRLF input: %+v", i, item)
		}
	}
}

// AC-4 (ARCHITECTURE §Contracts → Batch): a batch whose list length exceeds MaxBatchPuzzles is
// rejected with HTTP 413 + ErrorResponse{Code:"invalid_input"} BEFORE any solving begins. The
// over-cap list is asserted to produce the error envelope (not a partial BatchResult), which is
// the observable proof that no per-puzzle solving ran.
func TestAC4_Batch_OverCapRejected413BeforeSolving(t *testing.T) {
	h := api.BatchHandler()

	// One past the cap. Content is irrelevant: rejection is on COUNT before parse/solve.
	over := make([]string, api.MaxBatchPuzzles+1)
	blank := strings.Repeat("0", 81)
	for i := range over {
		over[i] = blank
	}

	rr := postBatch(t, h, over, "application/json")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-cap batch: got HTTP %d, want 413 (body=%s)", rr.Code, rr.Body.String())
	}

	var env api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("over-cap body not an ErrorResponse: %v (raw=%s)", err, rr.Body.String())
	}
	if env.Code != "invalid_input" {
		t.Fatalf("over-cap code=%q, want invalid_input", env.Code)
	}

	// Proof that solving did not begin: the response is the error envelope, not a (partial)
	// BatchResult — no results/solvedCount keys are emitted.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("over-cap body not a JSON object: %v", err)
	}
	if _, ok := raw["results"]; ok {
		t.Fatalf("over-cap response leaked a results array — solving ran before the cap check")
	}
	if _, ok := raw["solvedCount"]; ok {
		t.Fatalf("over-cap response leaked solvedCount — solving ran before the cap check")
	}
}

// AC-5 (SECURITY §F-12): POST /v1/validate-batch with a non-application/json Content-Type is
// rejected with HTTP 415, before the body is interpreted.
func TestAC5_Batch_RejectsNonJSONContentType(t *testing.T) {
	h := api.BatchHandler()
	seed := loadSeed(t)

	if rr := postBatch(t, h, seed, "text/plain"); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain content-type: got HTTP %d, want 415 (F-12)", rr.Code)
	}
	if rr := postBatch(t, h, seed, ""); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content-type: got HTTP %d, want 415 (F-12)", rr.Code)
	}
}
