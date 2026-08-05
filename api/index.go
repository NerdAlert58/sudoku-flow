// Package handler is the Vercel (@vercel/go) serverless entrypoint. @vercel/go does not run
// a package main / http.ListenAndServe server; it builds a single function that exports
// func Handler(w http.ResponseWriter, r *http.Request) and invokes it per request. This file
// is that function and nothing more — it delegates to the same app.NewHandler stack the local
// cmd/server binary serves, so behavior is byte-identical across both entrypoints.
//
// @vercel/go compiles this entrypoint as a module-less command-line-arguments build, which
// forbids importing internal/ packages. It therefore depends ONLY on the public .../app
// package; app is what reaches into internal/api. Do not add an internal/ import here.
package handler

import (
	"net/http"

	"github.com/scottbushyhead/sudoku-flow/app"
)

// h is built exactly once at package initialization and reused across every request, so the
// mux and middleware chain are not reconstructed per invocation.
var h = app.NewHandler()

// Handler is the exact signature @vercel/go requires. It forwards the incoming request —
// path unchanged — to the shared handler so the ServeMux still matches /v1/* and /.
func Handler(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}
