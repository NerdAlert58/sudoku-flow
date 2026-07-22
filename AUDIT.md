# AUDIT — sudoku-flow

The captured state of the world this project must live with. ARCHITECTURE.md treats every
finding here as a hard input: a decision that ignores an audit finding is invalid. Gathered
at `/nerdflow:arch` on 2026-07-20 from three parallel research passes (Sudoku solving
domain, Go/Vercel/CI-CD platform, `puzzles.txt` fixture) plus environment inspection.

## Summary

**The central risk is retired: all 25 seed puzzles are solvable by pure logic — in fact by naked singles alone.** A brute-force check confirmed every line in `puzzles.txt` has exactly one solution and is well-formed, and a singles-only solver clears all 25 to the byte-identical unique solution. The PRD success criterion "solve all 25 logic-only, zero backtracking" is fully satisfiable and low-risk. The *consequence* is a coverage gap, not a solving risk (see D-Q3).

**The seed fixture proves plumbing, not the technique suite.** Because all 25 are easy-tier, passing them exercises none of the advanced techniques the project scopes (hidden singles, locked candidates, subsets, X-wing, swordfish, wings, colouring). A labeled per-tier fixture must be curated during build or the advanced solver ships unverified. This is the single most important eval consequence of the audit.

**Vercel changed: the serverless-vs-server tension the PRD implies is largely gone.** Vercel now offers a Go *server preset* — the same `main.go` (a stdlib `net/http` handler listening on `$PORT`) serves both localhost and Vercel with no adapter. The residual platform constraint is the **10-second Hobby per-request cap** plus throttled/variable free-tier CPU, which collides with large-batch (UC-4) and pollutes cross-deployment speed comparison. Resolution ratified: Vercel is the zero-cost demo; serious benchmarking runs on localhost.

**"No backtracking" is a solver-path property that CI must mechanically prove, not assume.** The only thing that enforces the project's hardest constraint is the event-log replay test: every placement must be forced by its named technique given prior grid state. If that test is not built, the logic-only guarantee is unverified marketing. It is a required, non-optional test.

**"Implements all known logic algorithms" is an open-ended scope that conflicts with the leanness comparison axis.** The technique family has dozens of chain/ALS variants that never fire on realistic puzzles. Reframed (DESIGN_DECISIONS ADR-0002) to a defined, ordered tier: Singles → Wings + basic fish + simple colouring.

**The Go toolchain is not installed on this machine.** The golanger builder writes Go, but `go build`/`go test` require the toolchain. Installing `go 1.26` is a Day-Zero prerequisite for `/nerdflow:impl` and build — not an architecture blocker.

## Architecture

### A1. Vercel Go server preset — one binary, local and deployed
- **Where:** `vercel.json` (`"framework": "go"`), `cmd/server/main.go`, Vercel Go runtime docs.
- **What:** Vercel detects a root `go.mod` + a `main.go`, builds a binary, and runs it listening on `$PORT`. A standard `http.ListenAndServe(":"+os.Getenv("PORT"), mux)` works verbatim — the same binary that serves localhost serves Vercel. No `/api/*.go` serverless-function adapter is needed.
- **So what:** the architecture is a single conventional Go HTTP service, not a function-per-endpoint layout. Localhost and the deployment are byte-identical, which is exactly what a cross-deployment comparison needs.
- **Open questions for ARCHITECTURE.md:** none — resolved as the one-binary topology (ADR-0009).

### A2. stdlib net/http is sufficient
- **Where:** `internal/api/`, Go 1.22+ `net/http.ServeMux`.
- **What:** Go 1.22+ `ServeMux` supports method+path patterns (`mux.HandleFunc("POST /v1/solve", …)`) and wildcards. For ~4 endpoints this fully replaces chi/gin/echo. `log/slog` (structured logging) is stdlib since 1.21.
- **So what:** no third-party HTTP dependency; aligns with the golanger stdlib-forward default and keeps the dependency surface (and the leanness axis) minimal.
- **Open questions:** none — stdlib ratified (ADR-0008).

### A3. Go version and pinning
- **Where:** `go.mod`.
- **What:** latest stable Go is 1.26.5 (2026-07-07). Pin `go 1.26` plus `toolchain go1.26.5`; Vercel reads the `go` directive and honors `toolchain`.
- **So what:** reproducible builds across localhost, CI, and Vercel — a prerequisite for comparable benchmarks.
- **Open questions:** none.

### A4. Embedded UI from the same binary
- **Where:** `web/`, served via `embed.FS` + `http.FileServerFS` at `/`.
- **What:** static assets embed into the binary; one artifact serves API + UI on localhost and any deployment with no separate frontend build, repo, or CDN.
- **So what:** the "optional simple UI" ships for free with the deploy and needs no additional infrastructure.
- **Open questions:** none — embedded UI ratified (ADR-0014).

### A5. Versioned, deployment-agnostic contract
- **Where:** `internal/api/contract.go`, `/v1/*` routes, `/v1/health`.
- **What:** a future React dashboard calls *multiple* deployments and compares them, so the JSON shape must be identical across NerdFlow iterations. A `/v1` prefix + an `apiVersion` field + a self-labeling `/v1/health` (goVersion, apiVersion) freeze the comparison surface.
- **So what:** the contract is the one thing that must NOT drift between builds; it is declared in a single module and versioned (additive→field, breaking→`/v2`).
- **Open questions:** none.

## Security

### S1. Minimal attack surface; single secret
- **Where:** `internal/api/` (input boundary), GitHub Actions Secrets, `internal/sudoku` (validation).
- **What:** the service is stateless — no DB, no auth, no PII, no user accounts, no persistence. The only untrusted input is the 81-char puzzle string; the only secret is the Vercel deploy token (plus two non-secret Vercel IDs).
- **So what:** least-privilege is trivially satisfiable (scope the token to the single project). The one real control that must exist is strict input validation: exactly 81 characters, `0`–`9` only, reject anything else with a typed error before the solver runs. Malformed input must never reach the solver.
- **Open questions for ARCHITECTURE.md:** where the validation boundary sits (answer: `internal/sudoku` parse/validate, called by every handler before solving) and how errors surface (answer: the `{error, code}` envelope owned by `internal/api`).

## Data Quality

### D-Q1. puzzles.txt format specifics
- **Where:** `C:\Users\Scott\Git\sudoku-flow\puzzles.txt` (2073 bytes).
- **What:** 25 lines, each exactly 81 chars. Empty cell = `0` (digit zero) — **not** `.`; the only characters present are `0`–`9`. Line endings are **CRLF**, with **no trailing newline** after line 25. Clue counts range 24–29.
- **So what:** the parser MUST accept `0` as the blank marker (mandatory to consume the seed set; accepting `.` is optional generosity). The batch loader MUST trim `\r` and tolerate a missing final newline, or the last line mis-parses.
- **Open questions:** none — codified as a CONSTRAINT on the Grid/Batch contracts.

### D-Q2. All 25 well-formed and unique
- **Where:** `puzzles.txt`, verified by a brute-force solution counter.
- **What:** every puzzle has valid givens (no duplicate in any row/column/box) and exactly one solution.
- **So what:** correctness for the seed set can be checked by comparing the solver's output to the brute-force oracle's unique solution; no committed answer key is needed (none exists).
- **Open questions:** none.

### D-Q3. Seed set is entirely easy tier — the coverage gap
- **Where:** `puzzles.txt`; EVAL.md `## Datasets and fixtures`.
- **What:** all 25 solve by **naked singles alone**. None require hidden singles, locked candidates, subsets, or any advanced technique. Passing 25/25 gives *false confidence* that the advanced tiers work — they would be entirely unexercised.
- **So what:** a **labeled advanced fixture set** (per tier: hidden singles, locked candidates, naked/hidden subsets, X-wing, swordfish, XY-wing) must be curated during build and is a Day-Zero prerequisite for the eval strategy. The success criterion "solve all 25" is necessary but far from sufficient to prove the in-scope technique suite.
- **Open questions for ARCHITECTURE.md / EVAL.md:** how many cases per tier and who labels them (answer in EVAL: brute-force oracle labels solutions; each fixture line annotated with its hardest-required technique).

### D-Q4. No answer key committed
- **Where:** repo root.
- **What:** the only tracked data file is `puzzles.txt`; no solutions file exists (as the PRD expects).
- **So what:** ground truth for tests is generated by the backtracking brute-force oracle in test code, never shipped in the solve path.
- **Open questions:** none.

## Performance

### P1. Vercel Hobby 10s cap and throttled CPU
- **Where:** Vercel Functions limits (Hobby tier).
- **What:** Hobby functions have a hard 10-second per-request duration cap and throttled, variable vCPU. Single-puzzle solves are sub-millisecond and safe; a full `puzzles.txt` batch with complete event logs, or any parallelism benchmark, can approach or breach 10s and is measured on noisy CPU.
- **So what:** UC-4 batch on Vercel must be size-bounded; serious timing and the UC-5 parallelism benchmark run on localhost (real cores, no cap). Vercel is a correctness/single-solve demo. Cross-deployment speed is only comparable within one host class.
- **Open questions:** none — resolved as Vercel-demo + localhost-benchmark (ADR-0005); batch size cap is a build-time acceptance detail.

### P2. Intra-puzzle parallelism does not pay
- **Where:** `internal/solver`, UC-5.
- **What:** a single 9×9 logic solve is sub-millisecond–few-ms single-threaded; productive deductions form a sequential dependency cascade (each mutates candidates the next reads). Only the read-only scan phase is embarrassingly parallel, and goroutine/channel overhead dwarfs the µs-scale scan work. The genuine parallelism is one goroutine per puzzle at batch/request level.
- **So what:** UC-5 ships as batch goroutine-per-puzzle (real, linear scaling) plus an explicitly-labeled intra-puzzle experiment published as a measured negative result. The PRD's "solve multiple cells simultaneously" is reframed accordingly.
- **Open questions:** none — resolved (ADR-0006).

### P3. Timing must be measured in-handler
- **Where:** `internal/api` handlers, `SolveResult.solveTimeMs`.
- **What:** cold starts and HTTP round-trip inflate naive latency; `solveTimeMs` must wrap the solve call inside the handler, excluding cold start and transport.
- **So what:** documented that `solveTimeMs` is solve-only; the metric is host-class-comparable but not cross-platform-absolute.
- **Open questions:** none.
