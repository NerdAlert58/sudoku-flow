package api

import "net/http"

// cspPolicy is the Content-Security-Policy for every response (F-10). script-src/default-src
// carry NO 'unsafe-inline'/'unsafe-eval' — which is exactly why the SPA's JS and CSS are
// external files, not inline. frame-ancestors 'none' + X-Frame-Options: DENY deny framing two
// ways; base-uri 'self' pins the document base; img-src allows data: for any inline SVG/PNG the
// UI draws itself. No external origins appear, so the policy is 'self'-only end to end.
const cspPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; frame-ancestors 'none'; base-uri 'self'"

// SecurityHeaders is the global, path-agnostic edge middleware (F-10). It sets all four browser
// controls BEFORE delegating to next: once a downstream handler commits its status via
// WriteHeader the header map is frozen, so setting them after next.ServeHTTP would silently drop
// them. Wrapped around the whole mux in cmd/server, every response — the SPA at "/" and every
// "/v1/*" endpoint — carries the headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", cspPolicy)
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// corsAllowedOrigins is the CORS allowlist (F-9). It is intentionally empty: the v1 posture is
// same-origin-only — the SPA is served from the same origin as /v1, so no cross-origin grant is
// needed. A future dashboard on another origin gets added here as an explicit, enumerated entry.
// The map is never populated by reflecting the request's Origin.
var corsAllowedOrigins = map[string]bool{}

// CORS implements an explicit, non-reflecting cross-origin policy (F-9). An arbitrary Origin is
// NEVER echoed into Access-Control-Allow-Origin, and a wildcard is NEVER emitted (so the
// forbidden "*"-with-credentials pairing cannot occur). Only an Origin present in the enumerated
// allowlist receives an ACAO grant, and then only its exact value. Cross-origin preflights from
// disallowed origins get a 204 with no CORS grant, which the browser correctly treats as a deny.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && corsAllowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
