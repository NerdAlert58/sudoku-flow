# IMPLEMENTATION_PLAN — sudoku-flow

**Status:** Active. Updated phase-by-phase as work lands.
**Source inputs (frozen):** PRD.md, AUDIT.md, USERS.md, ARCHITECTURE.md, DESIGN_DECISIONS.md, EVAL.md, SECURITY.md, COMPLIANCE.md.
**Delivery shape:** Linear phase-gate — 7 phases, P-0 through P-6. Per-phase briefs live in `docs/phases/`.
**Created:** 2026-07-20.
**Deadline:** None (PRD — AI-driven continuous build until done).

## Phase index

| ID | Title | Goal | Depends on | Status | Brief |
|---|---|---|---|---|---|
| P-0 | Scaffold & contracts | Runnable server shell: module, layout, grid model + validation (trust boundary), `/v1` contract types, `/v1/health` | — | Done (2026-07-20) | [P-0](docs/phases/P-0-scaffold.md) |
| P-1 | Solver core & singles | Solve loop + event log + metric quartet + three-status decision + naked/hidden singles; `POST /v1/solve` solves all 25 seed puzzles, replay-proven | P-0 | Done (2026-07-20) | [P-1](docs/phases/P-1-solver-core.md) |
| P-2 | Advanced ladder & fixtures | Locked candidates → colouring; labeled per-technique fixture (≥3 each); difficulty grader | P-1 | Not started | [P-2](docs/phases/P-2-advanced-ladder.md) |
| P-3 | Generator | Symmetry + backtracking uniqueness counter + digger + difficulty targeting; `POST /v1/generate` | P-2 | Not started | [P-3](docs/phases/P-3-generator.md) |
| P-4 | Batch & parallelism | `POST /v1/validate-batch` goroutine-per-puzzle + intra-puzzle negative-result benchmark; `-race` clean | P-1 | Not started | [P-4](docs/phases/P-4-batch-parallel.md) |
| P-5 | Embedded UI | `embed.FS` McKinsey-clean SPA + security headers/CORS/output-encoding | P-1 | Not started | [P-5](docs/phases/P-5-ui.md) |
| P-6 | CI/CD & Ship | GitHub Actions gates + manual Vercel deploy + govulncheck + SHA-pinning + README | P-0..P-5 | Not started | [P-6](docs/phases/P-6-cicd-ship.md) |

## Critical path

`P-0 → P-1 → P-2 → P-3 → P-6` — the solver must reach full technique tier before the generator's
difficulty oracle is meaningful and before CI can gate the assembled system.

Convergence points: P-4 (batch) and P-5 (UI) fan off P-1's `/v1/solve` and re-synchronize at P-6,
which cannot exit until every prior phase's artifacts exist to be gated. The riskiest convergence is
P-2 → P-3: the generator's difficulty targeting depends on the full solver tier and its grader being
correct, so a grading defect in P-2 silently corrupts P-3's band-hit rate.

## Day-Zero prerequisites (verify before P-0 begins)

Verify these before P-0 starts; skipping them means the first `go build` or the P-6 deploy fails with
an environment error rather than a code error, wasting a build session.

- [ ] **Go 1.26 toolchain installed** (`go version` reports 1.26.x). AUDIT notes it is absent on the
      build machine. (Being handled during this planning session.)
- [ ] **Git configured** for branch + PR flow (already a git repo).
- [ ] **Vercel account + project linked + `VERCEL_TOKEN` generated** — required only before P-6's
      deploy workflow; not needed for P-0..P-5.
- [ ] **GitHub repo secrets set** (`VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID`) + branch
      protection requiring the `test` check + `production` Environment required-reviewer rule —
      required only before P-6.

## Phase shape

> Every phase brief contains, in order:
>
> 1. **Goal** — one sentence, the observable outcome this phase ships.
> 2. **Entry gate** — what must be true before starting.
> 3. **Dependencies** — phase IDs whose exit gates must be `Done` (citing what specifically is consumed).
> 4. **Allow-list (source)** — files the builder may create or modify. Excludes test files.
> 5. **Allow-list (tests)** — files the test-author may create or modify. Excludes source files.
> 6. **Read-only context** — pointers to frozen-input sections (cite by §). UI phases MUST cite `ARCHITECTURE.md §Frontend Design Language`.
> 7. **Compliance requirements** — from `COMPLIANCE.md` (N/A for this project — Applicable hats: N/A).
> 8. **CI/CD requirements** — from `ARCHITECTURE.md §CI/CD topology` (P-6 only).
> 9. **Suggested steps** — ordered guidance, not contract; builder may resequence within the allow-list.
> 10. **Acceptance criteria** — observable outcomes with stable IDs (`AC-N`).
> 11. **Automated checks** — exact commands with expected results.
> 12. **Test command** — `(inherit from CONTEXT.md §Test discipline)` or override or `N/A`.
> 13. **Coverage command** — `(inherit)` or override or `N/A`.
> 14. **Coverage report** — `(inherit)`.
> 15. **Test-exempt lines** — empty by default; `<file>:L<a>-<b> — <rationale>`.
> 16. **Manual smoke checks** — quick by-hand commands.
> 17. **Human verification** — numbered checks with what to look for and why it matters.
> 18. **Regression check** — re-run all prior phases' automated checks.
> 19. **Exit gate** — boolean criteria; all must be true; no adjectives.
> 20. **Implementation notes (filled in by the builder)** — empty at write time.
> 21. **Deliverable line** — `Phase N ready for review` OR `Phase N blocked because: <sentence>`.
> 22. **Health check** — `(inherit from CONTEXT.md §Deployment discipline)` or override or `N/A`.
> 23. **Rollback command** — `(inherit)` or override.
> 24. **Env vars required** — bulleted names; empty by default.
>
> **Fresh session per phase.** Each phase brief stands alone — a builder loads only `docs/phases/P-N-*.md` plus the frozen-input sections it cites.

## Universal forbidden actions (every phase)

- Never bypass hooks (`--no-verify`, `--skip-checks`).
- Never push directly to `main`/`master`. Branch + PR for every change.
- Never edit the frozen inputs to match the code. They are upstream.
- Never expand a phase's allow-list silently. Stop and surface.
- Never weaken or skip a gate. Fix the code, not the test.
- Never start a phase before its entry gate is true.
- Never duplicate a cross-cutting contract into a phase — reference by section.
- Never restate phase content from this index into a phase brief, or vice versa.
- **Never introduce backtracking, trial-and-error, guess-then-revert, or contradiction/forcing-chain
  reasoning into the SOLVE path** (DESIGN_DECISIONS ADR-0001). The generator's uniqueness counter
  (P-3) is the *only* place backtracking is permitted, and it must never be importable from
  `internal/solver`.
- **Never add uniqueness-assuming techniques** (Unique Rectangles, BUG) to the solver (ADR-0004).
- **Never add persistence, auth, accounts, or session state** (USERS refusals).
- **Never let `unsolvable` claim more than an in-tier constructive contradiction** (ADR-0011).

## Universal automated checks (run before every phase exit)

```bash
go build ./...      # must succeed — a build-breaking regression is a phase failure regardless of other progress
go vet ./...        # must be clean
go test -race ./... # all tests pass with the race detector enabled
```

## Cross-cutting contracts

Each is named once here (or in the frozen inputs). Phase briefs reference by section — never restate.

- **`/v1` HTTP contract** — owner: `ARCHITECTURE.md §Contracts → HTTP /v1 API contract`. All handlers
  conform; additive→optional field, breaking→`/v2`. The stable cross-iteration comparison surface.
- **Solve / Event / Grid-Candidates / Generate / Batch contracts** — owner: `ARCHITECTURE.md §Contracts`.
  Internal Go structs; grid-ownership rule (single goroutine per solve; per-puzzle copy in batch) is
  part of the Grid contract.
- **Error envelope `{error, code}`** — owner: `ARCHITECTURE.md §Contracts` (shared-infra owners) + `internal/api`. Every handler returns this shape on error.
- **Metric quartet definitions** — owner: `DESIGN_DECISIONS.md ADR-0007`. `solveTimeMs`, `eventCount`,
  `iterations`, `candidateChecks`, each defined precisely and frozen.
- **Determinism rule** — owner: `DESIGN_DECISIONS.md ADR-0012`. Cheapest-tier-first, row-major; identical
  input → byte-identical trace.
- **Frontend design language** — owner: `ARCHITECTURE.md §Frontend Design Language` + `DESIGN_DECISIONS.md ADR-0015`. The inline McKinsey-clean recipe; P-5 copies it verbatim.

## Integration acceptance

| UC | Eval-matrix row | Integration test | Owning phase | Runs at |
|---|---|---|---|---|
| UC-1 Solve | EVAL §Eval matrix → UC-1 | `POST /v1/solve` golden test over all 25 `puzzles.txt` (easy tier) + advanced fixtures at-or-below tier; assert solution == oracle, correct `status` | P-1 (easy), P-2 (advanced) | P-1 exit (25/25), P-2 exit (all fixtures) |
| UC-2 Explain | EVAL §Eval matrix → UC-2 | Replay property test: input → each event → final grid byte-identical to `solution`, no unexplained placement | P-1, P-2 | P-1 exit, P-2 exit |
| UC-3 Generate | EVAL §Eval matrix → UC-3 | `POST /v1/generate` property test: 100% unique (oracle counter), ≥90% band-hit | P-3 | P-3 exit |
| UC-4 Batch | EVAL §Eval matrix → UC-4 | `POST /v1/validate-batch` golden test over `puzzles.txt`: `solvedCount == 25`, items match single-solve | P-4 | P-4 exit |
| UC-5 Parallel | EVAL §Eval matrix → UC-5 | `go test -race` on batch + serial-vs-parallel result-equality benchmark | P-4 | P-4 exit |

## Cross-phase concerns

- **Timekeeping** — no calendar deadline (PRD). Pacing is operator-directed; each phase runs to its
  exit gate before the next begins. No hard-latest dates.

  | Phase | Soft start | Soft finish |
  |---|---|---|
  | P-0 | after Day-Zero go install | build+health green |
  | P-1 | P-0 done | 25/25 + replay green |
  | P-2 | P-1 done | per-technique fixtures green |
  | P-3 | P-2 done | unique+band green |
  | P-4 | P-1 done | race+batch green |
  | P-5 | P-1 done | UI solves in browser |
  | P-6 | P-0..P-5 done | CI gates green on PR |

- **Scope creep stop-list** — These are the only deliverables. Adding anything is scope creep.
  - No forcing chains / AIC / ALS / Nishio (ADR-0001).
  - No Unique Rectangles / BUG (ADR-0004).
  - No persistence, auth, accounts, or history (USERS refusals).
  - No React comparison dashboard (separate future repo).
  - No intra-puzzle parallelism as a claimed speedup — negative result only (ADR-0006).
  - No dark mode / design-system kit for the UI (ADR-0015).
  - No Cloud Run / second deploy target (ADR-0005).

- **Risk monitoring**

  | Gate | Highest risk during | In-plan mitigation |
  |---|---|---|
  | Logic-only guarantee | P-1, P-2 | UC-2 replay test (input→solution, every placement witnessed) is a hard exit gate |
  | Advanced-tier unverified (AUDIT D-Q3) | P-2 | ≥3 required-puzzles-per-technique fixture is a P-2 exit gate |
  | Grading defect corrupts generation | P-2→P-3 | grader property test in P-2; band-hit test in P-3 |
  | Data race in batch (ADR-0006) | P-4 | `go test -race` in every phase's universal checks + P-4 exit gate |
  | CI/CD supply-chain (SECURITY F-15) | P-6 | Actions pinned to commit SHAs; no `pull_request_target` — P-6 AC |

- **Followups** — deferred post-MVP. These belong in a future `FOLLOWUPS.md` or `STATE.md`.
  - Cloud Run deploy target for uncapped deployed benchmarking (ADR-0005 alternative).
  - Dark mode for the UI (ADR-0015).
  - Extending the technique tier to ALS/AIC if the operator later wants harder-puzzle coverage
    (requires re-running `/nerdflow:arch` — reverses ADR-0001).

## How to dispatch

> Linear plan: phases run one at a time. To start phase P-N:
>
> 1. Confirm P-(N-1) shows `Status: Done` in the Phase index above. If not, do not start.
> 2. If you want extra isolation, create a worktree: `git worktree add ../sudoku-flow-pN -b phase/P-N`. Otherwise the current branch is fine for a linear single-builder flow.
> 3. Open a fresh Claude Code session. Paste:
>
>    > Read `docs/phases/P-N-<name>.md` and the sections of ARCHITECTURE.md / AUDIT.md / USERS.md / DESIGN_DECISIONS.md / EVAL.md / SECURITY.md it cites under Read-only context. Build the phase. Follow the acceptance criteria. Stay within the allow-list. Verify every exit-gate item is observably true. Record decisions in the Implementation notes section. Cross-cutting discoveries propagate to IMPLEMENTATION_PLAN.md or ARCHITECTURE.md, not just here. Open a PR when the exit gate passes.
>
> 4. When the phase lands, update the Phase index: change `Status: Not started` to `Status: Done` and append a one-line completion note.
> 5. For programmatic dispatch — picking the next ready phase, fanning out a subagent, or creating a worktree — run `/nerdflow:build`.

## Frozen-input contract for this plan

This plan is the input to per-phase work. If a phase's allow-list is wrong or an exit gate is missing
a requirement, stop and amend the relevant brief explicitly (dated entry in the Amendment record
below). Do not silently expand allow-lists. Do not silently soften gates. The plan can change; silent
drift cannot. The frozen inputs (PRD/AUDIT/USERS/ARCHITECTURE/DESIGN_DECISIONS/EVAL/SECURITY/COMPLIANCE)
are upstream — if one is wrong, stop and re-run `/nerdflow:arch` on that slice.

## Amendment record

### Amendment <YYYY-MM-DD> — <reason>
(template hint — real amendments added during execution)
