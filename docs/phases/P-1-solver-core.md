# Phase P-1 — Solver core & singles

**ID:** P-1 · **Status:** Not started · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

## Goal
A working constructive solver for the singles tier wired to `POST /v1/solve`: it solves all 25 seed
puzzles logic-only, returns the frozen metric quartet + three-status decision + a replayable
technique-tagged event log, and the replay proves the solve used no backtracking.

## Entry gate
P-0 `Done` (module, `sudoku.Grid`/`Candidates`/`Parse`, `/v1` contract types, health server).

## Dependencies
- P-0 — the grid model + parse/validate, the `/v1` contract types, and the server shell.

## Allow-list (source)
- `internal/solver/**` (non-test)
- `internal/api/**` (non-test) — add the solve handler; do not rewrite the contract types
- `cmd/server/**` (non-test) — register the solve route

## Allow-list (tests)
- `internal/solver/*_test.go`
- `internal/api/*_test.go`
- `testdata/**` — golden solutions for the seed set

## Read-only context
- ARCHITECTURE.md §Contracts → Solve contract, Event contract, Grid/Candidates contract; §Components → `internal/solver`; §Observability
- USERS.md §UC-1, §UC-2
- EVAL.md §Eval matrix → UC-1, UC-2; §Ground-truth process (brute-force oracle in test code)
- DESIGN_DECISIONS.md §ADR-0001 (constructive-only), §ADR-0007 (metric quartet), §ADR-0011 (status), §ADR-0012 (determinism)
- AUDIT.md §D-Q2 (all 25 unique), §D-Q3 (seed is singles-only), §P2 (single-threaded, no intra-puzzle parallelism), §P3 (time measured in-handler)
- SECURITY.md §F-12 (content-type enforcement)

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
None — CI/CD wiring lands in P-6.

## Suggested steps
1. Define `SolveResult`, `Event`, `SolveStatus` per the contracts.
2. Implement the solve loop: cheapest-first, row-major, applying naked single + hidden single until fixpoint; count the metric quartet; decide `solved` / `stalled` / `unsolvable` per ADR-0011.
3. Emit an `Event` for every productive deduction (technique, witness cells, effect, gridAfter).
4. Add a brute-force backtracking oracle in **test code only** for golden solutions.
5. Wire `POST /v1/solve`; measure `solveTimeMs` inside the handler; enforce `application/json` content-type (415 otherwise).
6. Write the golden test (25 seed puzzles) and the replay test (input → events → solution).

## Acceptance criteria
- **AC-1:** `POST /v1/solve` with each of the 25 `puzzles.txt` grids returns `status:"solved"`, `solved:true`, and a `solution` that satisfies all 27 row/column/box constraints and equals the brute-force oracle's unique solution. **Source:** EVAL.md §Eval matrix → UC-1.
- **AC-2:** Each solve response includes the frozen metric quartet — `solveTimeMs`, `eventCount`, `iterations`, `candidateChecks` — with the definitions of ADR-0007; `solveTimeMs` is measured inside the handler (excludes transport). **Source:** DESIGN_DECISIONS.md §ADR-0007.
- **AC-3:** For every `solved` response, replaying the event log from the original input — applying each event to its pre-state — reproduces each recorded post-state and yields a final grid byte-identical to `solution`, with zero cells placed by anything other than a named, witnessed technique. **Source:** EVAL.md §Eval matrix → UC-2.
- **AC-4:** The same puzzle POSTed twice yields byte-identical `solution`, event log, and quartet (determinism). **Source:** DESIGN_DECISIONS.md §ADR-0012.
- **AC-5:** A valid grid the singles tier cannot finish returns `status:"stalled"` (not a guess, not `unsolvable`); a grid whose givens are malformed/rule-violating returns `status:"invalid_input"`; a grid where the tier constructively drives a cell to zero candidates returns `status:"unsolvable"`. **Source:** DESIGN_DECISIONS.md §ADR-0011.
- **AC-6:** `POST /v1/solve` with a non-`application/json` Content-Type is rejected with HTTP 415. **Source:** SECURITY.md §F-12.

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
```
Expected: golden test (25/25) and replay test pass; content-type + status-decision tests pass.

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
curl -s -H 'Content-Type: application/json' -d '{"puzzle":"<one line from puzzles.txt>"}' localhost:8080/v1/solve | jq .
```

## Human verification
1. Read a returned event log end-to-end — every step should name a technique (naked/hidden single) and a cell. Why it matters: this is the human-facing proof of logic-only solving.
2. Confirm `iterations`/`candidateChecks` are populated and non-zero. Why it matters: these are the benchmark axis.

## Regression check
Re-run P-0 automated checks (`go build`, `go vet`, health-handler + parse tests).

## Exit gate
- All 25 seed puzzles return `status:"solved"` with a constraint-valid solution equal to the oracle's.
- The replay test passes for every seed solve (input → solution, no unexplained placement).
- The metric quartet is present and deterministic across repeated identical requests.
- The three non-`solved` statuses each return correctly on a representative input.
- `POST /v1/solve` rejects non-JSON content types with 415.
- `go build`/`go vet`/`go test -race` all pass.

## Implementation notes (filled in by the builder)
> Record decisions and cross-cutting discoveries here; propagate upward as needed.

## Deliverable line
`Phase 1 ready for review` OR `Phase 1 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
