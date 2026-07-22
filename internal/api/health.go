package api

import (
	"encoding/json"
	"net/http"
	"runtime"
)

// HealthHandler returns the GET /v1/health handler: HTTP 200 with a JSON body of
// {status:"ok", goVersion, apiVersion:"1"} (ADR-0010). It is a pure self-identification
// endpoint — no request body, no downstream calls — so it cannot fail on input.
func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, HealthResponse{
			Status:     "ok",
			GoVersion:  runtime.Version(),
			APIVersion: APIVersion,
		})
	})
}

// writeJSON serializes v as JSON with the given status code and the JSON content type. A
// late encode error (after the header is written) can only be logged by the caller's
// middleware, not corrected, so it is intentionally dropped here.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
