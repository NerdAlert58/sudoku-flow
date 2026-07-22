# Phase P-4 — Batch & parallelism

**ID:** P-4 · **Status:** Done (2026-07-21) · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

> Completion: batch `POST /v1/validate-batch` (goroutine-per-puzzle, order-preserved, size cap 413, 415) solvedCount 25/25; `SolveParallel` byte-identical to `Solve`; **`go test -race` clean (real gate — gcc installed)**; ADR-0006 negative result confirmed (intra-puzzle ~5.5x SLOWER). jasnah PASS (mutation-tested race-freedom); leanness dedup (-70 lines, runEngine parameterized) applied + re-gated PASS.

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

**Landed 2026-07-21 (branch phase/p-4-batch-parallel, baseline b10fa08).**

Source added (allow-list only):
- `internal/api/batch.go` — `BatchHandler()` + `const MaxBatchPuzzles = 256`.
- `internal/solver/solve_parallel.go` — `SolveParallel` + `runEngineParallel` + `computeCandidatesParallel`.
- `cmd/server/main.go` — registered `POST /v1/validate-batch` inside the existing `SecurityHeaders(CORS(Recover(MaxBytes(routes()))))` chain.

Decisions:
- **Goroutine-per-puzzle, disjoint-index writes (ADR-0006).** Each puzzle is parsed+solved in its own goroutine writing ONLY `items[i]`; a `sync.WaitGroup` joins. `sudoku.Grid` is a value type (`[81]uint8`, no pointers), so the per-goroutine grid lives on that goroutine's stack — the copy is intrinsic, not a defensive clone. No append-from-goroutine, no shared mutable state → `-race` clean (verified, incl. 16 concurrent batch requests in TestAC2_Batch_RaceFreeUnderConcurrentRequests). Cox-Buday: the WaitGroup + preallocated result slice is the "no shared writes" fan-out; ownership of each index is transferred to exactly one goroutine.
- **`MaxBatchPuzzles = 256`.** The list-length cap IS the goroutine-count bound — a request can never fan out unbounded goroutines. Over-cap by COUNT is the primary 413 path (checked before any solving, so AC-4's "no results leaked" holds). `http.MaxBytesReader` (1 MiB, own cap so the handler is bounded even driven directly in tests) is the byte-cap defense; a tripped reader surfaces `*http.MaxBytesError` via `errors.As` → the same 413/`invalid_input` envelope.
- **F-12 gate mirrors solve.go** — `mime.ParseMediaType` content-type check BEFORE the body is read; non-JSON → 415.
- **CRLF (D-Q1):** `strings.TrimSpace(line)` before `sudoku.Parse` handles both the trailing-CR CRLF-split artifact and the missing-final-newline last puzzle. `Puzzle` echoes the RAW input line (order/identity marker); parse failure is a per-item not-solved, never a whole-batch failure.
- **SolveParallel = byte-identical to Solve (ADR-0012).** `runEngineParallel` is `runEngine` verbatim with ONE substitution: `computeCandidates` → `computeCandidatesParallel`. The productive-step selection, elim-layering, event append, and terminal-status logic are the identical sequential code, so Events/counters/HardestTechnique are byte-identical. The parallel part is genuinely concurrent read-only work: peer masks (rows/cols/boxes) are built once sequentially, then the per-cell candidate derivation fans out across 3 bands of 27 cells, each goroutine writing ONLY its disjoint `cand[i]` indices while reading the masks read-only. `CandidateChecks` and the zero-candidate flag are folded AFTER the join in fixed row-major order, so they never depend on goroutine scheduling → deterministic across runs (TestAC6_..._DeterministicAcrossRuns passes).

**ADR-0006 negative-result benchmark (i7-14700K, windows/amd64, 25 seed grids/op):**

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkSolveSequential-28` | 621,559 | 592,610 | 4,350 |
| `BenchmarkSolveIntraParallel-28` | 3,453,303 | 1,318,684 | 19,475 |

Intra-puzzle scan-parallelism is **~5.6x SLOWER** and allocates ~4.5x more. A sub-millisecond 9x9 solve cannot amortise per-pass goroutine spawn/join overhead (AUDIT §P2 confirmed). This is the honest UC-5 story: it ships as a benchmarked negative result, never as a speed feature. The REAL parallelism win is inter-puzzle (goroutine-per-puzzle in the batch handler), not intra-puzzle.

Gates (all green): `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`, `go test -bench=. -benchmem ./internal/solver/`.

## Deliverable line
`Phase 4 ready for review` OR `Phase 4 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
