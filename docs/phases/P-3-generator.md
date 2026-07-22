# Phase P-3 — Generator

**ID:** P-3 · **Status:** Done (2026-07-21) · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

> Completion: generator (symmetry full-grid + backtracking uniqueness counter + difficulty targeting); band-hit 60/60 (100%); import boundary proven (solver ∌ generator via go list -deps); generation↔solve loop verified live; jasnah PASS (mutation-tested uniqueness/grade/boundary); leanness (popcount→math/bits) applied + re-gated. `-race` deferred to CI.

## Goal
A puzzle generator wired to `POST /v1/generate` that returns a valid, uniquely-solvable puzzle graded
at (or nearest to) a requested difficulty — using an internal backtracking uniqueness counter and the
solver as a difficulty oracle, with the counter never leaking into the solve path.

## Entry gate
P-2 `Done` (full technique ladder + grader — the generator's difficulty oracle).

## Dependencies
- P-2 — the full solver tier and the difficulty grader used as the uniqueness/difficulty oracle.

## Allow-list (source)
- `internal/generator/**` (non-test)
- `internal/api/**` (non-test) — add the generate handler
- `cmd/server/**` (non-test) — register the generate route

## Allow-list (tests)
- `internal/generator/*_test.go`
- `internal/api/*_test.go`

## Read-only context
- ARCHITECTURE.md §Contracts → Generate contract (blinded backtracking counter); §Components → `internal/generator`; §Cross-component flows (generation ↔ solve seam)
- USERS.md §UC-3
- EVAL.md §Eval matrix → UC-3
- DESIGN_DECISIONS.md §ADR-0003 (generation may backtrack; solver may not), §ADR-0013 (grading), §ADR-0004 (uniqueness via counter, not solve-path)
- SECURITY.md §F-14 (generate enum validation)

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
None — CI/CD wiring lands in P-6.

## Suggested steps
1. Build a full solved grid via symmetry transforms of a base grid (relabel, band/stack/row/col permutation, transpose) — no backtracking needed for a full grid.
2. Dig clues, using a backtracking solution-counter to keep uniqueness == 1; use the P-2 solver/grader as the difficulty oracle (hardest technique required → target band).
3. Target the requested band with bounded retry; return nearest-achievable if the exact band can't be hit.
4. Wire `POST /v1/generate` with strict `difficulty` enum validation.
5. Keep the backtracking counter inside `internal/generator`; assert `internal/solver` does not import it.

## Acceptance criteria
- **AC-1:** `POST /v1/generate` returns a puzzle that is valid and has exactly one solution (confirmed by the counter), for every request. **Source:** EVAL.md §Eval matrix → UC-3.
- **AC-2:** Across a sample of generated puzzles, ≥90% match the requested difficulty band (nearest-achievable with bounded retry otherwise). **Source:** EVAL.md §Eval matrix → UC-3.
- **AC-3:** `internal/solver` does not import `internal/generator`, and no backtracking/solution-counting code path is reachable from a `POST /v1/solve` request — verifiable by an import/dependency assertion. **Source:** DESIGN_DECISIONS.md §ADR-0003.
- **AC-4:** `POST /v1/generate` with an unknown `difficulty` value is rejected with a typed `invalid_input` (not a default-and-proceed). **Source:** SECURITY.md §F-14.
- **AC-5:** `POST /v1/generate` with a non-`application/json` Content-Type is rejected with HTTP 415. **Source:** SECURITY.md §F-12.

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
```
Expected: uniqueness property test, band-hit-rate test, import-boundary assertion, and enum-validation test all pass.

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
curl -s -H 'Content-Type: application/json' -d '{"difficulty":"hard"}' localhost:8080/v1/generate | jq .
curl -s -H 'Content-Type: application/json' -d '{"difficulty":"bogus"}' localhost:8080/v1/generate -w '%{http_code}\n'
```

## Human verification
1. Generate a puzzle, then solve it via `/v1/solve` — confirm it solves logic-only and its grade matches the requested difficulty. Why it matters: proves the generation↔solve loop.
2. Confirm an unknown difficulty is rejected, not silently defaulted. Why it matters: SECURITY F-14.

## Regression check
Re-run P-0 + P-1 + P-2 automated checks.

## Exit gate
- 100% of generated puzzles are uniquely solvable.
- ≥90% band-hit on the requested difficulty.
- No backtracking path reachable from `/v1/solve`; `internal/solver` does not import `internal/generator`.
- Unknown `difficulty` rejected with `invalid_input`.
- Prior phases' checks still pass; `go build`/`go vet`/`go test -race` all pass.

## Implementation notes (filled in by the builder)

**Files added:** `internal/generator/{grid,counter,generate}.go`, `internal/api/generate.go`;
`POST /v1/generate` registered in `cmd/server/main.go`.

**Full-grid construction (no search).** `fullSolvedGrid` starts from the canonical
`value(r,c) = (3*(r%3)+r/3+c) mod 9 + 1` base solution and applies only validity-preserving
symmetry transforms — digit relabel, band/stack permutation, row/col permutation within a
band/stack, optional transpose — all driven by one seeded `*rand.Rand`. A symmetry image of a
solution is a solution, so no backtracking is needed to obtain the full grid (ADR-0003 step 1).

**Two oracles, two roles (ADR-0003/0004).** Digging keeps a clue removal only if BOTH gates hold:
(1) the backtracking counter (`solutionCount`, MRV DFS, cap 2) still reports exactly one solution
— the uniqueness authority per ADR-0004 — and (2) the solver still solves the puzzle within the
target band's technique ceiling via `solver.SolveWithMaxTechnique` — the difficulty oracle. The
counter is checked first so it is genuinely load-bearing, not a rubber stamp on the solver.

**Band targeting = ceiling dig + exact confirm.** The ADR-0013 bands are contiguous ladder
ranges, so a puzzle solvable within band B's ceiling but not below it grades exactly at B. Digging
maximally under the ceiling (reshuffled passes until nothing more can be removed) drives a puzzle
to the edge of that ceiling's solvability, which strongly biases the hardest required technique
into band B. Each attempt is confirmed with the real `solver.Grade`; a miss retries with a fresh
solved grid, and the nearest-achievable band is returned if the per-band `attemptBudget` is
exhausted (EVAL UC-3). Observed band-hit: **60/60 (100%)** on the seeded AC-2 sample
(easy/medium/hard/expert each 15/15), in ~3.2s.

**Why logic-solvability is not the uniqueness proof.** A clue set the solver can solve logic-only
is necessarily unique, but ADR-0004 wants uniqueness certified by the counter, not inferred from
the solve path — so the counter remains the authoritative gate even though it correlates with the
ceiling check. This also keeps generated puzzles guaranteed logic-solvable (required for
`solver.Grade` to return a non-empty band and for the generation↔solve loop, UC-3).

**Import boundary (ADR-0003, AC-3).** `internal/generator` imports `internal/solver` (oracle only)
and `internal/sudoku`; nothing imports back. The backtracking counter lives entirely in
`internal/generator`, so `go list -deps internal/solver` cannot reach it — the solve path stays
provably guess-free. Confirmed by `TestAC3` (runs in the non-`-short` suite).

**Handler.** `GenerateHandler` mirrors `solve.go`: F-12 content-type check before body read (415),
JSON decode, then `generator.Generate`. `ErrInvalidDifficulty` is mapped to 400/`invalid_input`
(F-14, no default-and-proceed) and kept distinct from any other generation error (500). The
backtracking counter is never surfaced — the response is only `{puzzle, difficulty, grade}`.

**Verification.** `go build ./...`, `go vet ./...`, `go test ./...` (WITHOUT `-short`, so AC-2
executes) all green. `-race` deferred to CI (no C compiler locally, per task).

## Deliverable line
`Phase 3 ready for review` OR `Phase 3 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
