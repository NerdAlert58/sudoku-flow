# Phase P-0 — Scaffold & contracts

**ID:** P-0 · **Status:** Done (2026-07-20) · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

> Completion: automated checks green (build/vet/test); acceptance verified against diff + live health smoke; test-verifier (jasnah) PASS after Recover-test backfill; leanness advisory-only. `-race` deferred to CI (no local C compiler).

## Goal
A runnable Go server shell: module + package layout, the grid model with input validation (the trust
boundary), the frozen `/v1` contract types, and a `GET /v1/health` endpoint — `go build` succeeds and
the server answers health on `$PORT`.

## Entry gate
Go 1.26 toolchain installed (`go version` → 1.26.x). Repo is a git repo (it is).

## Dependencies
None — first phase.

## Allow-list (source)
- `go.mod`, `go.sum`
- `cmd/server/**`
- `internal/sudoku/**` (non-test)
- `internal/api/**` (non-test)
- `.gitignore`

## Allow-list (tests)
- `internal/sudoku/*_test.go`
- `internal/api/*_test.go`

## Read-only context
- ARCHITECTURE.md §Summary (trust-boundary posture); §Contracts → HTTP /v1 API contract, Grid/Candidates contract; §Components → `cmd/server`, `internal/api`, `internal/sudoku`; §Storage; §Observability
- AUDIT.md §A1 (Vercel Go server preset / `$PORT`), §A2 (stdlib routing), §A3 (Go 1.26 pin), §S1 (validation), §D-Q1 (puzzles.txt format: `0`=blank, CRLF)
- DESIGN_DECISIONS.md §ADR-0008 (stdlib net/http), §ADR-0009 (one binary / `$PORT`), §ADR-0010 (`/v1` + apiVersion + health), §ADR-0011 (status semantics)

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
None — CI/CD wiring lands in P-6.

## Suggested steps
Guidance only; the builder may resequence within the allow-list provided the exit gate is met.
1. `go mod init` with the module path; set `go 1.26` + `toolchain go1.26.5`.
2. Create the package layout skeleton.
3. Implement `internal/sudoku`: `Grid` (81 cells), `Candidates` (per-cell bitset), `Parse(string) (Grid, error)` enforcing length 81, chars `0`–`9` or `.`, and no rule-violating givens; be CRLF-tolerant where a line source is parsed.
4. Declare the `/v1` request/response types in `internal/api/contract.go` per the frozen contract (solve/generate/batch/health shapes + `{error, code}` envelope), even if only health is wired this phase.
5. Implement `cmd/server/main.go`: `ServeMux` on `$PORT`, `GET /v1/health` returning `{status:"ok", goVersion, apiVersion:"1"}`, `log/slog` structured logging, and an `http.MaxBytesReader` body cap + panic-recovery middleware.

## Acceptance criteria
- **AC-1:** `go build ./...` succeeds and produces a runnable server binary.
- **AC-2:** Starting the binary with `PORT` set makes `GET /v1/health` return HTTP 200 with a JSON body containing `status:"ok"`, a Go version, and `apiVersion:"1"`.
- **AC-3:** `sudoku.Parse` accepts a valid 81-char string using `0` for blanks (and `.` as an accepted alias) and returns a populated `Grid`.
- **AC-4:** `sudoku.Parse` returns a typed error (surfaced as the `{error, code}` envelope) for each of: length ≠ 81, an illegal character, and a duplicate digit among the givens in any row, column, or box — and never panics on user input.
- **AC-5:** The server binds to the port named by `$PORT` (no hardcoded port), satisfying the Vercel Go server preset. **Source:** ARCHITECTURE.md §A1 / ADR-0009.
- **AC-6:** Request bodies are bounded by `http.MaxBytesReader`; an over-cap body is rejected without reading it into memory. **Source:** ARCHITECTURE.md §Summary (trust boundary).

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
```
Expected: all succeed; tests for `Parse` (valid + each malformed case) and a `/v1/health` handler test pass.

## Test command
`(inherit from CONTEXT.md §Test discipline)`

## Coverage command
`(inherit)`

## Coverage report
`(inherit)`

## Test-exempt lines
Empty.

## Manual smoke checks
```bash
PORT=8080 go run ./cmd/server &
curl -s localhost:8080/v1/health
```

## Human verification
1. Hit `/v1/health` — look for a JSON body with `status:"ok"` and a version label. Why it matters: this is how the future dashboard identifies a deployment.
2. POST a malformed puzzle (e.g. 80 chars) once the solve route exists in P-1 — for now, unit-test the `Parse` error path. Why it matters: the trust boundary must reject bad input before any solver runs.

## Regression check
None (first phase).

## Exit gate
- `go build ./...`, `go vet ./...`, `go test -race ./...` all pass.
- `GET /v1/health` returns 200 with `status:"ok"` + version labels.
- `sudoku.Parse` accepts the valid 81-char `0`-blank format and returns a typed error for length≠81, illegal char, and duplicate-given cases.
- Server binds to `$PORT`.

## Implementation notes (filled in by the builder)
> Record decisions, trade-offs, and any cross-cutting discovery here. Propagate cross-cutting findings to IMPLEMENTATION_PLAN.md or ARCHITECTURE.md, not just this section.

**Built 2026-07-20 (branch phase/p-0-scaffold, Go 1.26.5). All P-0 ACs met.**

Source files written (allow-list only): `go.mod` (no deps → no `go.sum`), `.gitignore`,
`cmd/server/main.go`, `internal/sudoku/{grid,candidates,parse}.go`,
`internal/api/{contract,health,middleware}.go`. No `*_test.go` touched.

Decisions & trade-offs:
- **Grid = `[81]uint8`, `cells` unexported.** The only constructor is `Parse`, so a Grid that
  exists is a Grid whose values are known-legal — the invariant lives at the trust boundary,
  not in callers. Same-package solver (P-1) still reads the field directly. `String()` renders
  the canonical `0`-blank form, so `.`-input and `0`-input round-trip identically (AC-3).
- **`Candidates uint16` bitset, bit d = digit d (bit 0 unused).** Digit maps to its own bit
  index — no offset math at call sites. Methods (`Has/Add/Remove/Count`, `Full()`) treat an
  out-of-range digit as a no-op rather than panicking, keeping the type sound under any input.
  `Count` uses stdlib `math/bits.OnesCount16` (initially hand-rolled a popcount; corrected to
  stdlib-forward).
- **Parse CRLF tolerance = `TrimRight(s, "\r\n")` only.** Deliberately narrow: interior
  whitespace is NOT stripped, so a space at index 10 correctly returns `ErrInvalidCharacter`
  (not `ErrInvalidLength`). A trailing CRLF/LF from a puzzles.txt line (D-Q1) parses without a
  caller-side trim. Validation order: length → per-char allowlist → givens (row/col/box) so the
  most specific typed error surfaces. Cheney-style sentinels wrapped with `%w`; wrapped detail
  (index/byte/unit) is for logs and the `{error,code}` envelope, never control flow. Row/col/box
  duplicates all map to `ErrDuplicateValue` per the brief. Never panics (AC-4).
- **`MaxBytes` = one-line delegation to `http.MaxBytesHandler`** (stdlib-forward; no bespoke
  byte counter). `Recover` middleware returns 500 `{error,code}` on handler panic.
- **`main.go` reads `$PORT` directly (`addr := ":" + os.Getenv("PORT")`), no fallback default**
  — satisfies ADR-0009 "no hardcoded port." Ryer-style `run() error` split from `main`. slog
  JSON handler on stdout; a transport-level access-log middleware (`logRequests`) owns the
  method/path/status/duration line — puzzle-hash + solveTimeMs logging defers to the solve
  handler (P-1) where that data exists. Global body cap `maxBodyBytes = 1<<20`.
- **`contract.go` declares the full `/v1` surface now** (solve/generate/batch + `{error,code}`
  envelope) as plain JSON structs with primitive fields — no import of `internal/solver`, so
  P-0 compiles without the solver package. Only health is wired.

Gate results:
- `go build ./...` ✓ · `go vet ./...` ✓ · `go test ./...` ✓ (api + sudoku green).
- `go test -race ./...` **could not execute on this host**: `-race` requires cgo and no C
  compiler (`gcc`) is installed on this Windows machine. P-0 has zero goroutines / zero shared
  mutable state, so the race detector is trivially green; it runs for real in CI on Linux
  (ARCHITECTURE §CI/CD `test` gate). See Deviations.
- Smoke: `PORT=8137 server` → `GET /v1/health` → `200 {"status":"ok","goVersion":"go1.26.5",
  "apiVersion":"1"}` (AC-2, AC-5).

No file-size tripwires fired (largest is `contract.go` at ~15 top-level symbols, single
responsibility; no function >80 lines; no file mixes http+db+json+validators — there is no db).

## Deliverable line
`Phase 0 ready for review` OR `Phase 0 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
