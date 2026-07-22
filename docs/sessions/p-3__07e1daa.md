# Session: P-3
Date: 2026-07-21
Agent: subagent (golanger — test-author + builder + leanness-apply)
Piece / Brief: P-3 (docs/phases/P-3-generator.md)
Baseline SHA -> Head: 07e1daa -> 4bf893e

## Accomplished
- `internal/generator`: `Generate` / `GenerateSeeded` / `ErrInvalidDifficulty`. Full solved grid via symmetry transforms of a base solution (no backtracking for the full grid, ADR-0003 step 1); clue-digging gated by a backtracking uniqueness counter (`solutionCount`, MRV DFS, cap 2 — the ADR-0004 uniqueness authority) AND the solver's `SolveWithMaxTechnique` as the difficulty oracle; difficulty targeting to the requested band with bounded per-band retry, confirmed by the real `solver.Grade`.
- `internal/api/generate.go`: `GenerateHandler` wired at `POST /v1/generate` — F-12 content-type gate (415), enum validation (`ErrInvalidDifficulty` → 400 `invalid_input`, distinct from 500), blinded response `{puzzle, difficulty, grade}` (counter never surfaced).

## Decisions made (and why)
- Two oracles, distinct roles: the backtracking counter is the uniqueness authority (checked FIRST on every dig, so it's load-bearing not a rubber stamp); the solver is the difficulty oracle. Keeps ADR-0004's "uniqueness via counter, not solve-path" honest.
- Band targeting = maximal ceiling-dig + exact `Grade` confirm (contiguous ladder bands make this reliable); nearest-achievable returned if the per-band budget is exhausted.
- Deterministic `GenerateSeeded` (single seeded `*rand.Rand` threaded through) for reproducible tests; unseeded `Generate` (time-seeded) is the production entry.
- **Import boundary (ADR-0003):** `internal/generator` imports `internal/solver` (oracle) + `internal/sudoku`; nothing imports back. The counter lives entirely in the generator, so it is provably unreachable from a `/v1/solve` request.
- No durable/architectural decision surfaced → no ADR promoted.

## Deviations from the frozen plan
- None. No test/contract.go/go.mod modified; import boundary honored.
- `go test -race` not run (Windows host, no C compiler). Generator is single-threaded, seeded path has no shared state. CI runs `-race` on Linux (P-6). **Still a hard prerequisite for P-4.**

## Test evidence
- Red-state capture (Phase 5.0): generator + api compile-red on undefined symbols (GenerateSeeded, ErrInvalidDifficulty, api.GenerateHandler); solver/sudoku green.
- Green-state capture (Phase 5.1): `go test -count=1 ./...` (no `-short`, so AC-2's 60-sample band-hit ran) — all packages green; live smoke: generate Hard (26 clues, grade "Hard") → solve it back → solved:true.
- Coverage report: coverage.lcov (lcov), 93.2% total. Counter/dig/grid 97–100%. Exemptions ruled reasonable by test-verifier: `Generate` 0% (1-line time-seeded wrapper, exercised cross-package by the handler test — per-package attribution artifact); GenerateHandler 76.5% / solvableWithin 75% / bandDist 71.4% (defensive-unreachable branches).
- Test-exempt lines applied: none.
- test-verifier verdict: **PASS** (mutation-tested: non-unique emission → AC-1 independent counter fails; mis-grade → AC-1/AC-2 fail; leaked counter import → AC-3 `go list -deps` fails). Re-gated **PASS** after the leanness edit.
- Compliance evidence: none (COMPLIANCE.md `Applicable hats: N/A`).

## Deployment evidence
- **Target:** manual (CONTEXT.md `cicd_deploy_hook: manual`).
- **Deploy command run:** none — Phase 5c skipped.
- **Health check:** N/A — deploy skipped (manual/CI at P-6).
- **Rollback command (unrun):** N/A — deploy skipped.
- **Env vars propagated:** none (only `$PORT`).
- **Deviations:** none. `deployment: SKIP (cicd_deploy_hook: manual)`.

## Leanness review
- **RIGOR:** basic
- **Findings:** 1 stdlib (`counter.go` hand-rolled popcount → `math/bits.OnesCount16`).
- **Net removable:** -7 lines
- **Disposition:** **applied** (user-chosen), then re-gated: coverage + test-verifier both re-passed (jasnah PASS; import boundary re-confirmed; coverage-dip by construction). Aligns popcount with the stdlib usage already in `internal/sudoku` and `internal/solver`.
- **Raw report:**
    RIGOR: basic
    counter.go:L79-86: stdlib hand-rolled popcount (Kernighan) reinvents math/bits.OnesCount16. Delete popcount, import "math/bits", call bits.OnesCount16(cands).
    net: -7 lines possible.

## Open / next session
- P-4 (batch/parallelism) and P-5 (embedded UI) now the ready set (both dep P-1, already Done). **Resolve local `-race` (C toolchain / WSL) before P-4** — it's the phase where the race gate genuinely matters (goroutine-per-puzzle).
- P-6 (CI/CD & ship) still blocked on P-4/P-5.
- Non-blocking followups (test-verifier RUBRIC_GAP, for a future /nerdflow:cleanup): AC-2 asserts aggregate ≥90% not per-band (add a per-band floor if a weak single band is a concern); no malformed-JSON-body test on `/v1/generate` (the GenerateHandler 76.5% decode branch) — mirrors the solve handler.
