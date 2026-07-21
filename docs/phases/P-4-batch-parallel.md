# Phase P-4 — Batch & parallelism

**ID:** P-4 · **Status:** Not started · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

## Goal
A batch endpoint that solves a list of puzzles concurrently (one goroutine per puzzle, race-free) and
an honest intra-puzzle scan-parallel benchmark published as a measured negative result.

## Entry gate
P-1 `Done` (the solver + `/v1/solve`). P-2 recommended for advanced-fixture inputs but not required to start.

## Dependencies
- P-1 — the solver and `SolveResult` contract (batch solves each item through the same solver).

## Allow-list (source)
- `internal/api/**` (non-test) — add the batch handler
- `internal/solver/**` (non-test) — the flagged intra-puzzle scan-parallel variant only; do not change the sequential solver's contract
- `cmd/server/**` (non-test) — register the batch route

## Allow-list (tests)
- `internal/api/*_test.go`
- `internal/solver/*_test.go`
- `internal/solver/*_bench_test.go` — serial-vs-parallel benchmark

## Read-only context
- ARCHITECTURE.md §Contracts → Batch contract (bounded; per-goroutine Grid copy; CRLF-safe); §Parallelism posture; §Components → `internal/api`
- USERS.md §UC-4, §UC-5
- EVAL.md §Eval matrix → UC-4, UC-5
- DESIGN_DECISIONS.md §ADR-0006 (goroutine-per-puzzle; intra-puzzle = negative result)
- AUDIT.md §D-Q1 (CRLF/trailing-newline parsing), §P1 (batch size cap), §P2 (intra-puzzle doesn't pay)
- SECURITY.md §F-12 (content-type), and the batch size-cap control in ARCHITECTURE.md §Contracts (Batch)

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
None — CI/CD wiring lands in P-6.

## Suggested steps
1. Implement `POST /v1/validate-batch`: parse the list CRLF-safely, give each puzzle its own `Grid` copy in its own goroutine, collect results in input order, return per-item results + `solvedCount`/`total`.
2. Enforce the batch size cap (`http.MaxBytesReader` + max `len(puzzles)` → 413/`invalid_input`).
3. Add a flagged intra-puzzle scan-parallel solver variant and a `go test -bench` comparing it to the single-threaded solver; document the negative result.

## Acceptance criteria
- **AC-1:** `POST /v1/validate-batch` with the 25 `puzzles.txt` grids returns `solvedCount:25`, `total:25`, and each item's result equals that puzzle's single-`/v1/solve` result. **Source:** EVAL.md §Eval matrix → UC-4.
- **AC-2:** The batch path is race-free under `go test -race`, and batch results are byte-identical to solving the same list serially. **Source:** EVAL.md §Eval matrix → UC-5; DESIGN_DECISIONS.md §ADR-0006.
- **AC-3:** The batch loader tolerates CRLF line endings and a missing final newline without mis-parsing the last puzzle. **Source:** AUDIT.md §D-Q1.
- **AC-4:** A batch exceeding the size cap (body bytes or list length) is rejected with HTTP 413 / `invalid_input` before any solving begins. **Source:** ARCHITECTURE.md §Contracts (Batch) — input-size boundary control.
- **AC-5:** `POST /v1/validate-batch` with a non-`application/json` Content-Type is rejected with HTTP 415. **Source:** SECURITY.md §F-12.
- **AC-6:** A committed benchmark compares the intra-puzzle scan-parallel variant against the single-threaded solver on the same inputs, and the result is recorded as a negative (no material speedup) rather than presented as a feature. **Source:** DESIGN_DECISIONS.md §ADR-0006; USERS.md §UC-5.

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
go test -bench=. -benchmem ./internal/solver/
```
Expected: batch golden test, race-clean batch test, CRLF test, size-cap test pass; benchmark runs and reports serial vs parallel.

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
jq -Rn '{puzzles: [inputs]}' puzzles.txt | curl -s -H 'Content-Type: application/json' -d @- localhost:8080/v1/validate-batch | jq '{solvedCount,total}'
```

## Human verification
1. Batch-validate `puzzles.txt` — confirm `solvedCount:25`. Why it matters: the core success-criterion loop, automated.
2. Read the benchmark output — confirm the intra-puzzle variant is reported as no-speedup. Why it matters: honesty of the UC-5 story.

## Regression check
Re-run P-0 + P-1 (+ P-2 if landed) automated checks.

## Exit gate
- `solvedCount:25/25` on `puzzles.txt` batch, items matching single-solve.
- `go test -race` clean on the batch path; parallel results == serial.
- CRLF + missing-final-newline handled.
- Over-cap batch rejected with 413/`invalid_input`.
- Serial-vs-parallel benchmark committed and recorded as a negative result.
- `go build`/`go vet`/`go test -race` all pass.

## Implementation notes (filled in by the builder)
> Record decisions and cross-cutting discoveries here.

## Deliverable line
`Phase 4 ready for review` OR `Phase 4 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
