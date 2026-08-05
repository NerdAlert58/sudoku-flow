// Command server is the standalone binary that serves sudoku-flow on localhost (and any
// host-a-process target). It reads $PORT (no hardcoded port, ADR-0009), owns observability
// (log/slog) and process lifecycle, and delegates the entire handler stack — routes plus the
// middleware chain — to app.NewHandler so the Vercel serverless entrypoint (api/index.go)
// serves a byte-identical handler.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/scottbushyhead/sudoku-flow/app"
	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// run wires the handler and blocks in ListenAndServe. Splitting it out of main keeps the
// wiring testable and gives a single error return for the process to act on (Ryer's
// "return an error from run" pattern). The handler itself is built by app.NewHandler so this
// binary and the serverless function share one construction path.
func run() error {
	handler := app.NewHandler()

	addr := ":" + os.Getenv("PORT")
	slog.Info("starting server", "addr", addr, "apiVersion", api.APIVersion)
	return http.ListenAndServe(addr, handler)
}
