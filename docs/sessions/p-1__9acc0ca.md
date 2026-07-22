# Session: P-1
Date: 2026-07-20
Agent: subagent (golanger — test-author + builder + hidden-single backfill)
Piece / Brief: P-1 (docs/phases/P-1-solver-core.md)
Baseline SHA -> Head: 9acc0ca -> 4c3d6ba

## Accomplished
- `internal/solver/solver.go`: constructive singles-tier solver — `Solve(sudoku.Grid) SolveResult` applying naked single then hidden single, cheapest-first, row-major, one placement per main-loop pass, candidates recomputed each pass. No backtracking, no guessing. Emits a replayable technique-tagged `[]Event` (per-step `GridAfter`, non-empty `WitnessCells`) + the frozen metric quartet (`Iterations`, `CandidateChecks`, `EventCount`; `SolveTimeMs` left 0 for the handler). Three-status decision (solved / stalled / unsolvable) per ADR-0011.
- `internal/api/solve.go`: `SolveHandler` — F-12 content-type gate (415 via `mime.ParseMediaType` before body read), `sudoku.Parse` boundary rejection (400 + `{error,code}` `invalid_input` envelope, upstream of the solver), in-handler wall-clock `solveTimeMs` around `Solve` only, `SolveResult`→`SolveResponse` mapping.
- `cmd/server/main.go`: registered `POST /v1/solve`.
- Tests (test-author): 25-puzzle golden test vs an independent brute-force oracle; the AC-3 replay test (input→events→solution, every placement forced+witnessed); metric-quartet, determinism, three-status, 415; plus (round 2) a hidden-single fixture routed through the replay proof, a stalled-after-progress case, and a malformed-JSON-body 400 test.

## Decisions made (and why)
- One placement per pass + recompute candidates each pass → crisp `Iterations` (ADR-0007), structural determinism (ADR-0012), and mechanical forced-ness for the replay proof (ADR-0001). No maps/goroutines (AUDIT P2).
- `SolveTimeMs` measured in the handler, not `Solve` → wall-clock (ADR-0007) stays out of the determinism byte-identity, which covers event log + the three counters.
- `invalid_input` implemented as the handler's 400 + `{error,code}` envelope upstream of the solver (ARCHITECTURE §Summary: "malformed input is rejected with a typed envelope and never reaches the technique ladder"), NOT a 200 `status:"invalid_input"`. This resolves a genuine tension between the brief's AC-5 prose and ARCHITECTURE — adjudicated by the frozen `contract.go` comment ("statuses live on the success path; this envelope is the failure path") + ARCHITECTURE. Disclosed in the tests and solver comments; not averaged.
- No durable/architectural decision surfaced → no ADR promoted. (Carry-forward: confirm ARCHITECTURE §Summary is the authoritative adjudicator for the invalid_input reading at any future arch sign-off.)

## Deviations from the frozen plan
- `go test -race` could not run (Windows host, no C compiler). Plain `go test ./...` green; solve path has zero goroutines / zero shared mutable state; CI runs `-race` on Linux (P-6). Same environment gap flagged in P-0 — still a real prerequisite for P-4 (concurrency).

## Test evidence
- Red-state capture (Phase 5.0): `internal/solver` unimplemented (package not buildable) + `internal/api` `undefined: api.SolveHandler` — clean symbol-level red; P-0 `internal/sudoku` tests stayed green (api tests blocked-not-broken by same-package compile).
- Green-state capture (Phase 5.1): `go test ./...` green — all 15 P-1 tests + all P-0 tests pass. No tests deleted/skipped.
- Coverage report: coverage.lcov (lcov), 81.2% overall. Core fully covered: `Solve` 100%, `applyNakedSingle` 100%, `applyHiddenSingle` 100% (0→100% after the backfill), `computeCandidates`/`place`/`render` 100%, `SolveHandler` 100%. Uncovered: `cmd/server/main.go` wiring (smoke-owned, P-0 precedent) and `solve.go` eliminations-mapping (P-1 places only — exempt).
- Test-exempt lines applied: none.
- test-verifier verdict: FAIL (round 0) → PASS (round 1). Round-0 blocker: `hidden_single` (a core technique and an arm of the AC-3 no-backtracking proof) had 0 coverage — all 25 seeds solve by naked singles alone (AUDIT §D-Q3). Routing: re-dispatched test-author to add a hidden-single fixture through the replay proof; jasnah mutation-tested the fix (wrong-value AND place-nothing regressions both now fail) and re-verified PASS.
- Compliance evidence: none (COMPLIANCE.md `Applicable hats: N/A`).

## Deployment evidence
- **Target:** manual (CONTEXT.md `cicd_deploy_hook: manual`).
- **Deploy command run:** none — Phase 5c skipped.
- **Health check:** N/A — deploy skipped (manual/CI-managed at P-6).
- **Rollback command (unrun):** N/A — deploy skipped.
- **Env vars propagated:** none (only `$PORT`, platform-provided).
- **Deviations:** none. `deployment: SKIP (cicd_deploy_hook: manual)`.

## Leanness review
- **RIGOR:** basic
- **Findings:** 1 delete (`StatusInvalidInput` const), 1 yagni (`SolveResult.SolveTimeMs` field).
- **Net removable:** -4 lines
- **Disposition:** advisory-only. Both are frozen-contract elements — `SolveResult.SolveTimeMs` is a field of the ARCHITECTURE §Contracts Solve contract; `invalid_input` is one of ADR-0011's four defined statuses. Removing them would erode contract fidelity for -4 lines; the basic-rigor reviewer missed the contract linkage.
- **Raw report:**
    RIGOR: basic
    internal/solver/solver.go:L24: delete `StatusInvalidInput` const — Solve never returns it; the handler writes the "invalid_input" string literal directly. Remove the const.
    internal/solver/solver.go:L69: yagni `SolveResult.SolveTimeMs` field — never set by Solve, never read by the handler. Remove the field and its explanatory comment.
    net: -4 lines possible.

## Open / next session
- P-2 (advanced ladder & fixtures) builds on the solver: adds locked candidates → colouring and the labeled per-technique fixture (≥3 requiring each of 11 techniques). P-2's hidden-single-requiring puzzles that solve to completion will retire the last residual thinness jasnah noted (a hidden single on a fully-solved path).
- P-4 (batch/parallelism) also unblocked by P-1. Resolve local `-race` (C toolchain / WSL) before P-4.
- Carry-forward: confirm ARCHITECTURE §Summary as the authoritative adjudicator for the `invalid_input`-via-envelope reading at arch sign-off.
