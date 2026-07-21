# Phase P-2 — Advanced ladder & fixtures

**ID:** P-2 · **Status:** Not started · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

## Goal
Complete the constructive technique ladder (locked candidates through simple colouring), curate the
labeled per-technique fixture that proves each shipped technique, and add the difficulty grader —
closing the advanced-tier coverage gap the audit found.

## Entry gate
P-1 `Done` (solver loop, event log, metric quartet, status decision, singles tier).

## Dependencies
- P-1 — the solve loop, `Event`/`SolveResult` contracts, replay-test harness, and the metric quartet.

## Allow-list (source)
- `internal/solver/**` (non-test) — add technique implementations + the grader; do not change the contracts
- `internal/api/**` (non-test) — surface `hardestTechnique`/grade in the solve response if not already present

## Allow-list (tests)
- `internal/solver/*_test.go`
- `testdata/advanced/**` — the labeled per-technique fixture + oracle solutions

## Read-only context
- ARCHITECTURE.md §Components → `internal/solver`; §Contracts → Event, Solve contracts
- USERS.md §UC-1, §UC-2; §Demo-Data Reality Check
- EVAL.md §Eval matrix → UC-1, UC-2; §Datasets and fixtures → Labeled advanced fixture (per-technique ship gate); §Ground-truth process
- DESIGN_DECISIONS.md §ADR-0001 (constructive-only — no forcing chains), §ADR-0002 (the exact ladder), §ADR-0004 (no URs/BUG), §ADR-0013 (grading = hardest tier, Sudoku-Explainer ordering)
- AUDIT.md §D-Q3 (the coverage gap)

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
None — CI/CD wiring lands in P-6.

## Suggested steps
1. Implement, in ladder order: locked candidates (pointing/claiming), naked/hidden pairs-triples-quads, X-wing, swordfish, jellyfish, XY-wing, XYZ-wing, W-wing, simple colouring — each emitting a correctly-witnessed `Event`.
2. Curate `testdata/advanced/`: ≥3 puzzles that *require* each of the eleven non-trivial shipped techniques (unsolvable by any cheaper tier), plus the status-coverage grids (stalled / in-tier-refutable unsolvable / non-unique→stalled / invalid_input). Label each with its hardest-required technique and expected status; record oracle solutions.
3. Implement the difficulty grader: the highest tier forced during the solve sets the grade, bucketed Easy/Medium/Hard/Expert per ADR-0013.

## Acceptance criteria
- **AC-1:** For each of the eleven non-trivial shipped techniques there exist ≥3 fixture puzzles that require it, and the solver returns `status:"solved"` with a constraint-valid solution for every one; a technique with zero requiring puzzles fails this phase. Necessity is proven by the **floor test**: with technique T and all cheaper techniques disabled, each of T's fixture puzzles must `stall` (confirming no cheaper tier suffices). **Source:** EVAL.md §Datasets and fixtures (per-technique ship gate); §Ground-truth process.
- **AC-2:** The replay test (input → events → solution, every placement witnessed) passes for every advanced-fixture solve — the logic-only guarantee holds at the advanced tier. **Source:** EVAL.md §Eval matrix → UC-2.
- **AC-3:** No forcing-chain, Nishio, AIC, ALS, Unique-Rectangle, or BUG technique appears in `internal/solver`; the solver never assumes-and-reverts. **Source:** DESIGN_DECISIONS.md §ADR-0001, §ADR-0004.
- **AC-4:** The difficulty grader assigns the correct band to each labeled fixture (band = hardest technique the solve was forced to use), verifiable by disabling techniques above the claimed tier and confirming the solve still completes. **Source:** DESIGN_DECISIONS.md §ADR-0013; EVAL.md §Ground-truth process.
- **AC-5:** Non-unique fixtures return `status:"stalled"` (not `unsolvable`); in-tier-refutable unsolvable fixtures return `status:"unsolvable"`; invalid fixtures return `status:"invalid_input"`. **Source:** DESIGN_DECISIONS.md §ADR-0011; EVAL.md §Eval matrix → UC-1.

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
```
Expected: per-technique fixture tests, grader tests, replay tests, and status-coverage tests all pass.

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
curl -s -H 'Content-Type: application/json' -d '{"puzzle":"<an X-wing-requiring fixture>"}' localhost:8080/v1/solve | jq '.events[].technique'
```

## Human verification
1. Solve an X-wing/swordfish fixture and read the log — confirm the advanced technique actually fires and is witnessed. Why it matters: proves the upper tiers aren't dead code (the audit's central gap).
2. Check a graded fixture's returned grade matches its label. Why it matters: grading feeds P-3 generation.

## Regression check
Re-run P-0 + P-1 automated checks (seed 25/25 still solve; singles replay still passes).

## Exit gate
- ≥3 requiring puzzles per shipped technique, all solved with valid solutions and passing replay.
- Grader assigns the correct band to every labeled fixture.
- No banned technique present in the solver.
- Status-coverage fixtures (stalled / unsolvable / non-unique→stalled / invalid_input) all return their labeled status.
- Seed 25/25 still solve (regression).
- `go build`/`go vet`/`go test -race` all pass.

## Implementation notes (filled in by the builder)
> Record decisions and cross-cutting discoveries here.

## Deliverable line
`Phase 2 ready for review` OR `Phase 2 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
