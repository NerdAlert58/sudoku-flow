package api_test

// Test for the request-body cap (P-0 AC-6; ARCHITECTURE §Summary trust boundary).
// Request bodies must be bounded so an over-cap body is rejected without reading it all
// into memory.
//
// Test-defined source surface the builder implements to:
//   api.MaxBytes(next http.Handler, n int64) http.Handler
//
// Intended to be a thin, stdlib-forward wrapper over http.MaxBytesHandler (Go stdlib) so
// no bespoke limiting logic is written; the seam exists only to let this test inject a
// small limit. The builder is free to implement it as `return http.MaxBytesHandler(next, n)`.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// readBodyHandler reads the entire request body; if the read fails (body over cap) it
// returns 413, otherwise 200. This surfaces the MaxBytesReader boundary as an HTTP status.
func readBodyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// AC-6: an over-cap body is rejected (not accepted as 200).
func TestMaxBytes_RejectsOverCapBody(t *testing.T) {
	const limit int64 = 16
	h := api.MaxBytes(readBodyHandler(), limit)

	body := strings.Repeat("x", 1024) // far over the 16-byte cap
	req := httptest.NewRequest(http.MethodPost, "/v1/solve", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("over-cap body (%d bytes, limit %d) was not rejected: got status 200",
			len(body), limit)
	}
}

// AC-6 (complement): an under-cap body passes through untouched.
func TestMaxBytes_AllowsUnderCapBody(t *testing.T) {
	const limit int64 = 1024
	h := api.MaxBytes(readBodyHandler(), limit)

	req := httptest.NewRequest(http.MethodPost, "/v1/solve", strings.NewReader("small body"))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("under-cap body rejected: got status %d, want 200", rr.Code)
	}
}

// panicHandler always panics, standing in for any downstream handler that faults at runtime.
func panicHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom: downstream handler faulted")
	})
}

// Recover: a panic in a wrapped handler is caught and converted to a 500 {error, code}
// envelope, and the panic does NOT propagate out of ServeHTTP (the process survives)
// (ARCHITECTURE.md §Components; contract.go ErrorResponse). This exercises the live path
// wired in cmd/server/main.go, which was previously uncovered.
//
// Mutation sensitivity: were Recover reverted to a pass-through (`return next`), the panic
// would escape ServeHTTP and this test would crash/fail before any assertion — and no 500
// envelope would ever be written, so the status and body assertions would fail too.
func TestRecover_PanicBecomes500Envelope(t *testing.T) {
	h := api.Recover(panicHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rr := httptest.NewRecorder()

	// (c) The panic must not propagate: if Recover is a pass-through, this call panics and
	// the test fails via the runtime panic rather than reaching the assertions below.
	h.ServeHTTP(rr, req)

	// (a) Status must be 500.
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("recovered panic: got status %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	// (b) Body must be the {error, code} envelope, well-formed per contract.go.
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("recovered panic: got Content-Type %q, want application/json; charset=utf-8", ct)
	}

	var env api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("recovered panic: body is not valid ErrorResponse JSON: %v (body=%q)", err, rr.Body.String())
	}
	if env.Error == "" {
		t.Fatalf("recovered panic: envelope %q field is empty, want a human-readable message", "error")
	}
	if env.Code == "" {
		t.Fatalf("recovered panic: envelope %q field is empty, want a stable machine code", "code")
	}
}
