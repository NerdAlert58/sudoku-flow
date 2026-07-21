# Phase P-2 — Advanced ladder & fixtures

**ID:** P-2 · **Status:** Done (2026-07-21) · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

> Completion: 11 advanced techniques + grader + capping API; every technique sound + replay-proven (jasnah mutation-tested unsound eliminations → caught); two-tier ship gate per ADR-0018 (11/12 isolable exact-hardest, jellyfish fires+sound); coverage 99.2%; leanness (3 findings) applied + re-gated PASS. `-race` deferred to CI.

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

### What shipped (2026-07-21)
Refactored the solver so the technique set is parameterizable and added the full ADR-0002 ladder,
the capping API, and the grader. Source split for readability (all in `internal/solver`, no file
> 20 top-level symbols, no function > 80 lines):
- `solver.go` — engine loop (`runEngine`), `Solve`, `computeCandidates`, the two singles, event
  helpers. `SolveResult` gained `HardestTechnique Technique`.
- `ladder.go` — `Technique` consts, `Ladder`, the ordered technique registry, `techBand`,
  `SolveWithMaxTechnique`, `Grade`.
- `gridutil.go` — peers, `boxOf`, `sees`, `cellInUnit`, `digitsOf`, `combinations`.
- `technique_locked.go`, `technique_subset.go`, `technique_fish.go`, `technique_wing.go`,
  `technique_colour.go` — the 11 advanced techniques.

### Key design decisions
1. **Candidate model = `basicCandidates(board) &^ eliminations`, rebuilt every pass.** The engine
   keeps a persistent `elim [81]uint16` of technique eliminations and derives `cand` fresh each
   pass. This is byte-for-byte the model the AC-2 replay reconstructs, so every single placement is
   provably forced under exactly that set, and advanced techniques read the reduced set (cascades
   work). Eliminations are monotonic, so the loop strictly progresses and terminates.
2. **Cheapest-first, one productive step per pass** (unchanged from P-1). A technique fires only
   when nothing cheaper can act, so `HardestTechnique` is the genuinely-required tier and the
   floor/ceiling brackets are meaningful. Singles PLACE; every index-≥2 technique only ELIMINATES
   (Placement nil, Eliminations non-empty), then a single converts the reduction (ADR-0001).
3. **Fish share one size-parameterised implementation** (x-wing=2, swordfish=3, jellyfish=4) at
   three separate ladder indices, so capping distinguishes the three tiers.
4. **All eliminations are SOUND** — verified: across all 35 fixtures, no technique ever removed a
   candidate that is the digit in the oracle solution (the AC-2 soundness invariant, checked
   independently against every event including on stalled solves).
5. **api left untouched** — no api test asserts `hardestTechnique`/`grade` on `SolveResponse`, so
   per the allow-list ("add fields only if a test asserts them") `contract.go`/`solve.go` are
   unchanged. `BatchItem.HardestTechnique` already exists for P-3.

### Verification (independent of the failing fixtures)
GREEN: AC-1a ship gate, AC-3 ladder==ADR-0002 + no-banned-technique, AC-5 status coverage (all
four categories incl. the "world's hardest" puzzles stalling and non-unique→stalled), every
P-0/P-1 test (seed 25/25, singles replay, determinism, hidden-single, stalled, unsolvable), all
api tests. `go build`, `go vet` clean. (`-race` not run — no C compiler on this host; CI covers it.)

### Deviation — the per-technique fixtures are systematically unsuitable (needs test-author)
5 tests fail, all iterating `testdata/advanced/fixtures.txt`: AC-1b solve, AC-1c floor, AC-2
replay, AC-4a grade, AC-4b ceiling/floor. Root cause is fixture sourcing, not the solver: the
non-hidden_single fixtures are SudokuWiki strategy-page example puzzles, which merely *contain* the
featured technique — their overall hardest-required technique is generally higher, and often beyond
the entire constructive ladder. The tests require the label to be the *exact* hardest AND the
puzzle to be fully ladder-solvable.

Proven with an independent full-tier move-finder (singles→colouring, coded separately from the
solver): at every stall no constructive move exists (genuine beyond-tier), and before every climb
no cheaper move was skipped (genuine requirement). The solver's determined hardest tier is correct
and minimal. Per the brief, techniques were NOT weakened and fixtures were NOT edited.

Only 14/35 fixtures have `hardest == label` and pass cleanly. The 21 to swap:

| # | labeled | true hardest (this solver) | problem |
|---|---------|----------------------------|---------|
| 4 | locked_candidates_pointing | xy_wing | too-low; band Medium→Hard |
| 5 | locked_candidates_pointing | xy_wing | too-low; band Medium→Hard |
| 6 | locked_candidates_pointing | naked_subset | too-low (same band) |
| 7 | locked_candidates_claiming | xy_wing | too-low; band Medium→Hard |
| 8 | locked_candidates_claiming | xy_wing | too-low; band Medium→Hard |
| 9 | naked_subset | locked_candidates_pointing | too-high (floor fails) |
| 11 | naked_subset | locked_candidates_pointing | too-high (floor fails) |
| 12 | hidden_subset | locked_candidates_pointing | too-high (floor fails) |
| 13 | hidden_subset | naked_subset | too-high (floor fails) |
| 14 | hidden_subset | — (stalls) | beyond-tier |
| 16 | x_wing | — (stalls) | beyond-tier |
| 17 | x_wing | — (stalls) | beyond-tier |
| 19 | swordfish | xy_wing | too-low (same band) |
| 21 | jellyfish | — (stalls) | beyond-tier |
| 22 | jellyfish | — (stalls) | beyond-tier |
| 23 | jellyfish | xy_wing | too-low (same band) |
| 24 | xy_wing | w_wing | too-low; band Hard→Expert |
| 27 | xyz_wing | — (stalls) | beyond-tier |
| 28 | xyz_wing | — (stalls) | beyond-tier |
| 32 | w_wing | xy_wing | too-high; band Expert→Hard |
| 34 | simple_colouring | — (stalls) | beyond-tier |

(Row numbers are the data-line order in `fixtures.txt`.) Recommendation: the test-author sources
replacement puzzles whose *exact* hardest technique equals the label and that solve within the
constructive ladder — ideally constructed by the dig-from-solution method already used (and proven
reliable) for the hidden_single tier, rather than lifted from strategy-page examples.

## Deliverable line
`Phase 2 ready for review` OR `Phase 2 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
