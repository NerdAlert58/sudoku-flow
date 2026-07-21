package api_test

// P-5 AC-4 — explicit, non-reflecting CORS policy.
//
// Source: SECURITY.md F-9 (acceptance signal, verbatim): "an explicit CORS decision is
// implemented — same-origin-only, or an enumerated Origin allowlist for the future dashboard;
// the server never reflects arbitrary `Origin` and never sends `*` with credentials."
//
// Test-defined source surface the builder implements to:
//
//	api.CORS(next http.Handler) http.Handler
//
// The test is deliberately agnostic to WHICH acceptable posture the builder picks. Same-origin
// only (emit no Access-Control-Allow-Origin at all) and an enumerated allowlist (emit ACAO only
// for a known-good Origin) both PASS — the test pins only the two invariants F-9 forbids:
//   (a) an arbitrary Origin is never reflected back, and
//   (b) Access-Control-Allow-Origin: * is never paired with Allow-Credentials: true.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

const evilOrigin = "https://evil.example"

// corsOKHandler is the wrapped downstream: a committed 200 so the middleware runs its full path.
func corsOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// AC-4 (a): an arbitrary Origin is not reflected into Access-Control-Allow-Origin.
func TestCORS_DoesNotReflectArbitraryOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/solve", nil)
	req.Header.Set("Origin", evilOrigin)
	rr := httptest.NewRecorder()

	api.CORS(corsOKHandler()).ServeHTTP(rr, req)

	if acao := rr.Header().Get("Access-Control-Allow-Origin"); acao == evilOrigin {
		t.Fatalf("CORS reflected an arbitrary Origin: Access-Control-Allow-Origin = %q (must not echo %q)",
			acao, evilOrigin)
	}
}

// AC-4 (b): the response never pairs a wildcard origin with credentials. This combination is
// rejected by browsers AND, if a server emitted it, would defeat the SOP — F-9 forbids it
// outright regardless of the requesting Origin.
func TestCORS_NoWildcardOriginWithCredentials(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/solve", nil)
	req.Header.Set("Origin", evilOrigin)
	rr := httptest.NewRecorder()

	api.CORS(corsOKHandler()).ServeHTTP(rr, req)

	acao := rr.Header().Get("Access-Control-Allow-Origin")
	acac := rr.Header().Get("Access-Control-Allow-Credentials")

	if acao == "*" && strings.EqualFold(strings.TrimSpace(acac), "true") {
		t.Fatalf("CORS sent Access-Control-Allow-Origin: * together with Allow-Credentials: true "+
			"(ACAO=%q, ACAC=%q) — forbidden combination", acao, acac)
	}
}
