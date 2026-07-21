package api

import (
	"encoding/json"
	"mime"
	"net/http"
	"time"

	"github.com/scottbushyhead/sudoku-flow/internal/solver"
	"github.com/scottbushyhead/sudoku-flow/internal/sudoku"
)

// SolveHandler returns the POST /v1/solve handler. It enforces the application/json
// content-type BEFORE reading the body (F-12: a strict allowlist, 415 otherwise), parses the
// puzzle at the sudoku.Parse trust boundary (malformed/rule-violating givens are rejected
// upstream of the solver with HTTP 400 + {error, code} envelope, Code "invalid_input" —
// ARCHITECTURE §Summary, ADR-0011), runs the constructive solver while measuring solveTimeMs
// as wall-clock around the Solve call only (P3/ADR-0007, excludes transport), and returns the
// SolveResponse (HTTP 200) with the frozen metric quartet.
func SolveHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// F-12: content-type is validated first, before the body is touched. A missing or
		// non-application/json type (a "; charset=..." suffix is accepted) is a 415.
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, ErrorResponse{
				Error: "Content-Type must be application/json",
				Code:  "unsupported_media_type",
			})
			return
		}

		var req SolveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "request body is not valid JSON",
				Code:  "invalid_input",
			})
			return
		}

		// Trust boundary: the only untrusted input crosses here. A parse failure is the
		// invalid_input case and is rejected before the solver ever sees the grid.
		grid, err := sudoku.Parse(req.Puzzle)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: err.Error(),
				Code:  "invalid_input",
			})
			return
		}

		start := time.Now()
		res := solver.Solve(grid)
		solveTimeMs := float64(time.Since(start).Microseconds()) / 1000.0

		writeJSON(w, http.StatusOK, SolveResponse{
			APIVersion:      APIVersion,
			Input:           req.Puzzle,
			Status:          string(res.Status),
			Solved:          res.Solved,
			Solution:        res.Solution,
			Iterations:      res.Iterations,
			EventCount:      res.EventCount,
			CandidateChecks: res.CandidateChecks,
			SolveTimeMs:     solveTimeMs,
			Events:          toContractEvents(res.Events),
		})
	})
}

// toContractEvents maps the solver's internal event log onto the frozen /v1 contract types.
// The api layer stays blinded from solver internals — only the contract shape crosses the
// boundary — so the two type sets are converted explicitly rather than shared.
func toContractEvents(evs []solver.Event) []Event {
	out := make([]Event, len(evs))
	for i, e := range evs {
		ce := Event{
			Seq:          e.Seq,
			Technique:    e.Technique,
			WitnessCells: toContractCells(e.WitnessCells),
			GridAfter:    e.GridAfter,
		}
		if e.Placement != nil {
			ce.Placement = &Placement{
				Cell:  Cell{Row: e.Placement.Cell.Row, Col: e.Placement.Cell.Col},
				Value: e.Placement.Value,
			}
		}
		for _, el := range e.Eliminations {
			ce.Eliminations = append(ce.Eliminations, Elimination{
				Cell:      Cell{Row: el.Cell.Row, Col: el.Cell.Col},
				Candidate: el.Candidate,
			})
		}
		out[i] = ce
	}
	return out
}

func toContractCells(cs []solver.Cell) []Cell {
	out := make([]Cell, len(cs))
	for i, c := range cs {
		out[i] = Cell{Row: c.Row, Col: c.Col}
	}
	return out
}
