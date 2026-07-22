package api

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"

	"github.com/scottbushyhead/sudoku-flow/internal/generator"
)

// GenerateHandler returns the POST /v1/generate handler. It mirrors SolveHandler's edge
// discipline: (1) the application/json content-type is validated BEFORE the body is read (F-12:
// 415 otherwise), (2) the body is decoded into GenerateRequest, (3) the difficulty is validated
// against the generator's {easy,medium,hard,expert} allowlist — an unknown value is the typed
// invalid_input case (F-14: HTTP 400 + ErrorResponse{Code:"invalid_input"}, never
// default-and-proceed), distinguished from an internal generation failure (500) by the
// ErrInvalidDifficulty sentinel — (4) the generator produces the puzzle, and (5) the finished
// GeneratedPuzzle is returned (HTTP 200). The internal backtracking uniqueness counter is never
// surfaced (ARCHITECTURE §Generate contract — blinded surface).
func GenerateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// F-12: content-type first, before the body is touched. A "; charset=..." suffix is fine.
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, ErrorResponse{
				Error: "Content-Type must be application/json",
				Code:  "unsupported_media_type",
			})
			return
		}

		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "request body is not valid JSON",
				Code:  "invalid_input",
			})
			return
		}

		puzzle, grade, err := generator.Generate(req.Difficulty)
		if err != nil {
			// F-14: an unknown difficulty is a client input error, not a server fault. Any other
			// error would be an internal generation failure (500) — kept distinct on purpose.
			if errors.Is(err, generator.ErrInvalidDifficulty) {
				writeJSON(w, http.StatusBadRequest, ErrorResponse{
					Error: "difficulty must be one of easy, medium, hard, expert",
					Code:  "invalid_input",
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error: "puzzle generation failed",
				Code:  "internal_error",
			})
			return
		}

		writeJSON(w, http.StatusOK, GeneratedPuzzle{
			Puzzle:     puzzle,
			Difficulty: req.Difficulty,
			Grade:      grade,
		})
	})
}
