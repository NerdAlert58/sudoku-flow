package api

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// MaxBatchPuzzles is the maximum len(BatchRequest.Puzzles) the batch endpoint accepts. A longer
// list is rejected 413/invalid_input BEFORE any solving begins (ARCHITECTURE §Contracts → Batch;
// AUDIT §P1). The cap is a fixed-work bound so one request can never fan out an unbounded number
// of goroutines — the batch-size limit IS the goroutine-count limit (ADR-0006).
const MaxBatchPuzzles = 256

// maxBatchBodyBytes bounds the batch request body independently of the outer MaxBytes middleware,
// so BatchHandler enforces the byte cap even when driven directly (tests) rather than only behind
// the server chain. An over-cap body trips MaxBytesReader → 413/invalid_input, the same envelope
// as an over-cap list length.
const maxBatchBodyBytes int64 = 1 << 20

// BatchHandler returns the POST /v1/validate-batch handler. It enforces the application/json
// content-type BEFORE the body is read (F-12: 415 otherwise), wraps the body in
// http.MaxBytesReader and caps len(Puzzles) at MaxBatchPuzzles (over-cap by bytes OR by count →
// 413/invalid_input BEFORE any solving), CRLF-normalises each puzzle (AUDIT §D-Q1), and solves
// EACH puzzle on its OWN sudoku.Grid in its OWN goroutine (goroutine-per-puzzle, ADR-0006). Each
// goroutine writes only its own result-slice index, so there is zero shared mutable state and the
// path is race-free; results are collected in INPUT ORDER. A parse failure for one puzzle is a
// per-item not-solved, never a whole-batch failure. Success is HTTP 200 with BatchResult.
func BatchHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// F-12: content-type is validated first, before the body is touched (mirrors SolveHandler).
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, ErrorResponse{
				Error: "Content-Type must be application/json",
				Code:  "unsupported_media_type",
			})
			return
		}

		// Bound the body before decoding. An over-cap body makes Decode fail with a
		// *http.MaxBytesError, which maps to the same 413/invalid_input as an over-cap list.
		r.Body = http.MaxBytesReader(w, r.Body, maxBatchBodyBytes)
		var req BatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{
					Error: "request body too large",
					Code:  "invalid_input",
				})
				return
			}
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "request body is not valid JSON",
				Code:  "invalid_input",
			})
			return
		}

		// Size cap on the list length, checked BEFORE any solving so an over-cap batch never
		// starts work (AC-4: the observable proof is the error envelope, not a partial result).
		if len(req.Puzzles) > MaxBatchPuzzles {
			writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{
				Error: "too many puzzles in batch",
				Code:  "invalid_input",
			})
			return
		}

		// Goroutine-per-puzzle. Each goroutine solves an independent grid and writes ONLY its own
		// index in items — disjoint writes, no shared mutable state, no append from goroutines —
		// so a WaitGroup fan-out is race-free and preserves input order (ADR-0006).
		items := make([]BatchItem, len(req.Puzzles))
		var wg sync.WaitGroup
		wg.Add(len(req.Puzzles))
		for i, line := range req.Puzzles {
			go func(i int, line string) {
				defer wg.Done()
				items[i] = solveBatchItem(line)
			}(i, line)
		}
		wg.Wait()

		solved := 0
		for _, it := range items {
			if it.Solved {
				solved++
			}
		}

		writeJSON(w, http.StatusOK, BatchResult{
			APIVersion:  APIVersion,
			Results:     items,
			SolvedCount: solved,
			Total:       len(items),
		})
	})
}

// solveBatchItem CRLF-normalises one input line, parses and solves it on its own grid, and returns
// the per-item result. Puzzle echoes the RAW input line (order/identity marker). A parse failure
// is a per-item not-solved outcome — the batch as a whole still succeeds. sudoku.Grid is a value
// type (no pointers), so the grid parsed here lives entirely in this goroutine's stack: the
// per-goroutine copy is intrinsic, not a defensive clone.
func solveBatchItem(line string) BatchItem {
	item := BatchItem{Puzzle: line}
	// D-Q1: trim surrounding whitespace (a trailing CR from a CRLF split, or a missing final
	// newline) so the last puzzle of a CRLF-dirty source does not mis-parse.
	grid, err := sudoku.Parse(strings.TrimSpace(line))
	if err != nil {
		return item // per-item not-solved; do not fail the whole batch
	}
	start := hiNow()
	res := solver.Solve(grid)
	item.SolveTimeMs = hiElapsedMs(start)
	item.Solved = res.Solved
	item.Iterations = res.Iterations
	item.HardestTechnique = string(res.HardestTechnique)
	return item
}
