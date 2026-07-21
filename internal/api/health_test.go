package api_test

// Test for the /v1/health handler shape (P-0 AC-2). Per the brief, process-level port
// binding (AC-5) is validated by the coordinator's smoke run, not here; this test asserts
// only the handler's JSON contract per ADR-0010: {status:"ok", goVersion, apiVersion:"1"}.
//
// Test-defined source surface the builder implements to:
//   api.HealthHandler() http.Handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// AC-2: GET /v1/health returns HTTP 200 with a JSON body containing status:"ok",
// a Go version, and apiVersion:"1".
func TestHealthHandler_ShapeAndStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rr := httptest.NewRecorder()

	api.HealthHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/health status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Status     string `json:"status"`
		GoVersion  string `json:"goVersion"`
		APIVersion string `json:"apiVersion"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body is not decodable JSON: %v; raw=%q", err, rr.Body.String())
	}

	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.GoVersion == "" {
		t.Errorf("goVersion is empty, want a Go version label")
	}
	if body.APIVersion != "1" {
		t.Errorf("apiVersion = %q, want %q", body.APIVersion, "1")
	}
}
