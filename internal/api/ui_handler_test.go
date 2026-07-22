package api_test

// P-5 AC-1 — embedded SPA serving (Go-testable slice).
//
// Source: USERS.md §UC-1 ("pastes the string into the embedded UI at /"); ARCHITECTURE.md
// §Frontend Design Language (embedded SPA at /, served via embed.FS, fully self-contained —
// "must not hit an external host"); DESIGN_DECISIONS.md ADR-0014 (embed.FS UI).
//
// Test-defined source surface the builder implements to:
//
//	api.UIHandler() http.Handler   // serves the embedded web/ SPA (embed.FS) at /
//
// The builder wires this at cmd/server as the "/" route and supplies the web/ assets
// (index.html + app.js + style.css). This test only exercises GET / against the returned
// handler: it does not — and cannot — assert the browser rendering behavior (AC-2 textContent
// encoding, AC-5 visual match), which is human-verified per the brief's Test-exempt lines.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// externalURL matches any absolute http(s) URL. Relative/same-origin references (e.g.
// "app.js", "/style.css") carry no scheme and are intentionally NOT matched — the assertion
// is only that the served SPA reaches out to no external origin (no CDN, no web-font fetch).
var externalURL = regexp.MustCompile(`(?i)https?://[^\s"'<>)]+`)

// AC-1: GET / serves the embedded SPA — HTTP 200, an HTML content type, a non-empty body.
func TestUIHandler_ServesEmbeddedSPA(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	api.UIHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rr.Code, http.StatusOK)
	}

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(strings.ToLower(ct), "text/html") {
		t.Errorf("GET / Content-Type = %q, want it to contain %q", ct, "text/html")
	}

	body, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("reading GET / body: %v", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		t.Fatalf("GET / body is empty, want a non-empty HTML document")
	}
}

// AC-1 (self-contained invariant): the served SPA references no external origin. A page that
// pulls a CDN script or a web font is a network + CSP violation of ARCHITECTURE §Frontend
// Design Language ("NO web-font fetch ... must not hit an external host").
func TestUIHandler_NoExternalOrigins(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	api.UIHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d (cannot scan body of a non-200)", rr.Code, http.StatusOK)
	}

	body, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("reading GET / body: %v", err)
	}

	if hits := externalURL.FindAllString(string(body), -1); len(hits) > 0 {
		t.Fatalf("served SPA references %d external origin(s), want 0 (self-contained, no CDN/font fetch): %v",
			len(hits), hits)
	}
}
