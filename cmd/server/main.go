// Command server is the single binary that serves sudoku-flow on both localhost and the
// Vercel Go server preset. It reads $PORT (no hardcoded port, ADR-0009), builds an
// http.ServeMux with the /v1 routes (ADR-0008 stdlib routing), and owns observability
// (log/slog) and the outer middleware chain: body cap (MaxBytes) then panic recovery.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// maxBodyBytes caps every request body at the transport edge. 1 MiB comfortably holds a
// batch of puzzle strings while rejecting anything pathological before it is read into
// memory; the batch handler adds a len(puzzles) cap on top of this (ARCHITECTURE.md §Batch).
const maxBodyBytes int64 = 1 << 20

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// run wires the handler and blocks in ListenAndServe. Splitting it out of main keeps the
// wiring testable and gives a single error return for the process to act on (Ryer's
// "return an error from run" pattern).
func run() error {
	// Outer chain: SecurityHeaders(CORS(...)) at the edge so the F-10 headers and the F-9 CORS
	// decision land on every response — the SPA at "/" and every /v1/* endpoint, including a
	// recovered 500 (the headers are set before Recover ever writes). logRequests stays
	// outermost for an accurate access log.
	handler := api.SecurityHeaders(api.CORS(api.Recover(api.MaxBytes(routes(), maxBodyBytes))))

	addr := ":" + os.Getenv("PORT")
	slog.Info("starting server", "addr", addr, "apiVersion", api.APIVersion)
	return http.ListenAndServe(addr, logRequests(handler))
}

// routes registers the /v1 endpoints. Health and solve are wired through P-1, generate through
// P-3; validate-batch (P-4) fans out one goroutine per puzzle against the frozen contract types.
func routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/health", api.HealthHandler())
	mux.Handle("POST /v1/solve", api.SolveHandler())
	mux.Handle("POST /v1/generate", api.GenerateHandler())
	mux.Handle("POST /v1/validate-batch", api.BatchHandler())
	// GET / serves the embedded SPA. The /v1 patterns above are more specific and win; this is
	// the catch-all for GET, so index.html and its assets (app.js, style.css) resolve here.
	mux.Handle("GET /", api.UIHandler())
	return mux
}

// logRequests emits one structured slog line per request (method, path, status, duration).
// Puzzle-hash and solveTimeMs logging attach in the solve handler (P-1) where that data
// exists; here the cmd layer owns the transport-level access log.
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
