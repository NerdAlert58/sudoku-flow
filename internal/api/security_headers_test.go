package api_test

// P-5 AC-3 — security response headers on / and /v1/*.
//
// Source: SECURITY.md F-10 (acceptance signal, verbatim): "the embedded-UI/edge emits a CSP
// whose `script-src` disallows `unsafe-inline`/`unsafe-eval`, HSTS with a non-trivial
// `max-age`, `X-Frame-Options: DENY` (or `frame-ancestors 'none'`), and
// `X-Content-Type-Options: nosniff`."
//
// Test-defined source surface the builder implements to:
//
//	api.SecurityHeaders(next http.Handler) http.Handler
//
// This is a global, path-agnostic edge middleware: in cmd/server it wraps the whole mux so
// every response — the SPA at "/" and every "/v1/*" endpoint — carries the headers. The
// middleware MUST set the headers before delegating to next (a handler that commits its
// status via WriteHeader freezes the header map), so the stub below writes a real 200 body
// to catch a "headers set too late" implementation. The test drives two request paths ("/"
// and "/v1/health") to assert the headers are present on both surfaces named by the AC.

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

// committedOKHandler stands in for any real downstream handler: it commits the response by
// setting a content type, calling WriteHeader(200), and writing a body. If SecurityHeaders
// set its headers AFTER calling next, they would be lost here — exactly the bug to catch.
func committedOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// parseCSPDirectives splits a Content-Security-Policy value into directive -> source tokens.
// e.g. "default-src 'self'; script-src 'self'" -> {"default-src":["'self'"], "script-src":["'self'"]}.
func parseCSPDirectives(csp string) map[string][]string {
	out := map[string][]string{}
	for _, raw := range strings.Split(csp, ";") {
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) == 0 {
			continue
		}
		out[strings.ToLower(fields[0])] = fields[1:]
	}
	return out
}

// containsToken reports whether tokens holds needle (case-insensitive, quotes kept intact).
func containsToken(tokens []string, needle string) bool {
	for _, tok := range tokens {
		if strings.EqualFold(tok, needle) {
			return true
		}
	}
	return false
}

// assertSecurityHeaders runs one request at reqPath through SecurityHeaders(committedOKHandler)
// and checks all four F-10 controls on the response.
func assertSecurityHeaders(t *testing.T, reqPath string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, reqPath, nil)
	rr := httptest.NewRecorder()

	api.SecurityHeaders(committedOKHandler()).ServeHTTP(rr, req)

	// Read the headers a real client receives (rr.Result().Header), NOT the live, mutable
	// recorder map (rr.Header()). The recorder map reflects any late writes, so it would still
	// pass if SecurityHeaders set its headers AFTER next.ServeHTTP committed the response;
	// rr.Result().Header is the snapshot flushed at WriteHeader time and catches that ordering
	// regression (the jasnah gap).
	h := rr.Result().Header

	// (1) CSP: an effective script-src source list that disallows unsafe-inline / unsafe-eval.
	// Per CSP semantics script-src falls back to default-src, so accept either directive as
	// the effective list; require one to be present and clean.
	csp := h.Get("Content-Security-Policy")
	if csp == "" {
		t.Errorf("[%s] missing Content-Security-Policy header", reqPath)
	} else {
		dirs := parseCSPDirectives(csp)
		effective, ok := dirs["script-src"]
		if !ok {
			effective, ok = dirs["default-src"]
		}
		if !ok {
			t.Errorf("[%s] CSP has neither script-src nor default-src to govern script execution: %q", reqPath, csp)
		} else {
			if containsToken(effective, "'unsafe-inline'") {
				t.Errorf("[%s] CSP effective script source list contains 'unsafe-inline': %q", reqPath, csp)
			}
			if containsToken(effective, "'unsafe-eval'") {
				t.Errorf("[%s] CSP effective script source list contains 'unsafe-eval': %q", reqPath, csp)
			}
		}
	}

	// (2) HSTS with a non-trivial max-age (> 0).
	sts := h.Get("Strict-Transport-Security")
	if sts == "" {
		t.Errorf("[%s] missing Strict-Transport-Security header", reqPath)
	} else if age, ok := hstsMaxAge(sts); !ok {
		t.Errorf("[%s] Strict-Transport-Security has no parseable max-age: %q", reqPath, sts)
	} else if age <= 0 {
		t.Errorf("[%s] Strict-Transport-Security max-age = %d, want > 0: %q", reqPath, age, sts)
	}

	// (3) X-Frame-Options: DENY OR a CSP frame-ancestors 'none'.
	xfo := h.Get("X-Frame-Options")
	frameDenied := strings.EqualFold(strings.TrimSpace(xfo), "DENY")
	if !frameDenied && csp != "" {
		if fa, ok := parseCSPDirectives(csp)["frame-ancestors"]; ok && containsToken(fa, "'none'") {
			frameDenied = true
		}
	}
	if !frameDenied {
		t.Errorf("[%s] framing not denied: want X-Frame-Options: DENY or CSP frame-ancestors 'none' (XFO=%q, CSP=%q)",
			reqPath, xfo, csp)
	}

	// (4) X-Content-Type-Options: nosniff.
	if xcto := h.Get("X-Content-Type-Options"); !strings.EqualFold(strings.TrimSpace(xcto), "nosniff") {
		t.Errorf("[%s] X-Content-Type-Options = %q, want %q", reqPath, xcto, "nosniff")
	}
}

// hstsMaxAge extracts the numeric max-age from an HSTS header value.
func hstsMaxAge(v string) (int, bool) {
	for _, part := range strings.Split(v, ";") {
		part = strings.TrimSpace(part)
		if lower := strings.ToLower(part); strings.HasPrefix(lower, "max-age=") {
			n, err := strconv.Atoi(strings.TrimSpace(part[len("max-age="):]))
			if err != nil {
				return 0, false
			}
			return n, true
		}
	}
	return 0, false
}

// AC-3: the "/" (embedded SPA) surface carries all four security headers.
func TestSecurityHeaders_OnRootPath(t *testing.T) {
	assertSecurityHeaders(t, "/")
}

// AC-3: a representative "/v1/*" surface carries all four security headers.
func TestSecurityHeaders_OnV1Path(t *testing.T) {
	assertSecurityHeaders(t, "/v1/health")
}
