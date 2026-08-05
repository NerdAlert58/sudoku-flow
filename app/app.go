// Package app builds the complete serving handler shared by every entrypoint: the local
// cmd/server binary and the Vercel (@vercel/go) serverless function. It is a public,
// in-tree package (import path .../app) precisely so the Vercel entrypoint can depend on
// it — @vercel/go compiles that entrypoint as a module-less command-line-arguments build,
// which forbids importing internal/ packages directly. app sits above internal/api and
// wires its leaf handlers and middleware into the full stack; internal/api stays internal.
package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// maxBodyBytes caps every request body at the transport edge. 1 MiB comfortably holds a
// batch of puzzle strings while rejecting anything pathological before it is read into
// memory; the batch handler adds a len(puzzles) cap on top of this (ARCHITECTURE.md §Batch).
const maxBodyBytes int64 = 1 << 20

// NewHandler builds the complete serving handler shared by every entrypoint (the local
// cmd/server binary and the Vercel serverless function). It returns the full chain, outer to
// inner: logRequests(SecurityHeaders(CORS(Recover(MaxBytes(routes()))))) — the F-10 security
// headers and F-9 CORS decision land at the edge so they cover the SPA at "/", every /v1/*
// endpoint, and a recovered 500; logRequests stays outermost for an accurate access log
// (Ryer's single NewServer constructor pattern — one place wires the whole stack).
func NewHandler() http.Handler {
	handler := api.SecurityHeaders(api.CORS(api.Recover(api.MaxBytes(routes(), maxBodyBytes))))
	return logRequests(handler)
}

// routes registers the /v1 endpoints. Health and solve are wired through P-1, generate through
// P-3; validate-batch (P-4) fans out one goroutine per puzzle against the frozen contract types.
func routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/health", api.HealthHandler())
	mux.Handle("POST /v1/solve", api.SolveHandler())
	mux.Handle("POST /v1/generate", api.GenerateHandler())
	mux.Handle("POST /v1/validate-batch", api.BatchHandler())
	mux.Handle("GET /v1/puzzles", api.PuzzlesHandler())
	// GET / serves the embedded SPA. The /v1 patterns above are more specific and win; this is
	// the catch-all for GET, so index.html and its assets (app.js, style.css) resolve here.
	mux.Handle("GET /", api.UIHandler())
	return mux
}

// logRequests emits one structured slog line per request (method, path, status, duration).
// Puzzle-hash and solveTimeMs logging attach in the solve handler (P-1) where that data
// exists; here the transport layer owns the access log.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"durationMs", float64(time.Since(start).Microseconds())/1000.0,
		)
	})
}

// statusRecorder captures the response status code for the access log. It records the
// first WriteHeader; an implicit 200 (a bare Write) is the zero-configuration default.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}
