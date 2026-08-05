package app_test

// Tests for app.NewHandler — the single shared constructor both entrypoints (cmd/server and
// the Vercel serverless function) build from. These assert the assembled stack routes and
// serves end to end, not just the individual handlers the internal/api tests exercise directly.
//
// Test-defined source surface the builder implements to:
//   app.NewHandler() http.Handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/app"
)

// The full stack routes GET /v1/health to a 200 with the health JSON envelope, proving the
// mux, middleware chain, and health handler are all wired by NewHandler.
func TestNewHandler_HealthRoutes(t *testing.T) {
	srv := httptest.NewServer(app.NewHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json...", ct)
	}
}

// The catch-all GET / serves the embedded SPA (index.html) through the same stack, confirming
// the UI handler is reachable via NewHandler and not shadowed by the /v1 patterns.
func TestNewHandler_ServesSPA(t *testing.T) {
	srv := httptest.NewServer(app.NewHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
