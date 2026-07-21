package api

import (
	"log/slog"
	"net/http"
)

// MaxBytes bounds a request body to n bytes. It is a thin, stdlib-forward delegation to
// http.MaxBytesHandler: an over-cap body makes the next handler's Body.Read fail (and the
// server close the connection) rather than being buffered into memory (ARCHITECTURE.md
// §Summary trust boundary; P-0 AC-6). No bespoke byte counting is written here.
func MaxBytes(next http.Handler, n int64) http.Handler {
	return http.MaxBytesHandler(next, n)
}

// Recover wraps next with panic recovery. A panic in any handler is caught, logged with
// slog, and returned to the caller as a 500 {error, code} envelope so the process survives
// (ARCHITECTURE.md §Components: "a panic in a handler is recovered ... returned as a 500").
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("handler panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
				)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{
					Error: "internal server error",
					Code:  "internal_error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
