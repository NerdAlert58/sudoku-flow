# Architecture — sudoku-flow

## Summary

`sudoku-flow` is a single, stateless Go HTTP service that solves Sudoku puzzles using only
constructive, logic-based deduction — never guessing, never backtracking on the solve path —
and returns a solved grid, an honest status, a frozen set of performance counters, and a
replayable, technique-tagged event log. It is **not** a puzzle app: no accounts, no persistence,
no human-facing hints. It exists to be a measurable, reproducible artifact that the NerdFlow
operator benchmarks across successive NerdFlow builds (see USERS.md).

**Topology: a linear in-process pipeline behind a thin HTTP layer.** A request enters the
`api` layer, is parsed and validated by `sudoku`, handed to the `solver`, and the result is
serialized back. There is no distributed component graph, no message bus, no database —
everything is function calls inside one binary. This topology is chosen because the domain work
(a 9×9 logic solve) is sub-millisecond and inherently sequential; distributing it would add
latency and synchronization cost for no benefit (AUDIT P2). The only concurrency is
**goroutine-per-puzzle** in the batch path, where each puzzle is fully independent.

**Inter-component contract surface.** The one contract that crosses the *external* boundary is
the versioned HTTP `/v1` JSON API declared in `internal/api/contract.go`; it is the stable
comparison surface a future React dashboard depends on, so its change rule is strict: additive
changes add optional fields, breaking changes mint `/v2`. Internally, three typed contracts
cross package boundaries — `SolveResult` (`internal/solver`), `Event` (`internal/solver`), and
`Grid`/`Candidates` (`internal/sudoku`). All are Go structs; the rule for changing an internal
contract is a compile-checked refactor plus updated tests, since there is a single binary and no
independent deployment of consumers.

**Trust boundary posture.** Exactly one untrusted input exists: the 81-character puzzle string.
It is allowlist-validated at the `sudoku` parse boundary (length exactly 81; characters `0`–`9`
or `.`; no rule-violating givens) before any solver code runs. Malformed input is rejected with
a typed `{error, code}` envelope and never reaches the technique ladder. Request bodies are
bounded by an `http.MaxBytesReader` in `internal/api`, and the batch endpoint enforces a
**maximum puzzle-list length** (reject over the cap with `413` / `invalid_input`) — a
validation-boundary control, not merely a reliance on the platform's duration/body caps. There is no other
untrusted surface — no auth tokens to forge, no database to inject, no file uploads, no PII.

**Human-in-the-loop gates.** None in the request path (the solver is fully deterministic). The
only human gate is operational: the CI/CD production deploy requires a manual `workflow_dispatch`
and a required reviewer on the `production` GitHub Environment (see `## CI/CD topology`).

**Deterministic vs. model-driven.** Everything is deterministic. There is no LLM, no judge, no
randomness in the solve path. The generator uses seeded pseudo-randomness for grid symmetry
transforms and clue-digging order, but its output is validated to be uniquely solvable, so the
contract it produces is deterministic in correctness if not in exact grid. Determinism is the
whole point: the same puzzle must yield a byte-identical trace run-to-run, or cross-iteration
benchmarking is meaningless.

**Observability posture.** Per-request performance is returned *in the response body* (the
metric quartet), which is the primary signal. `cmd/server` additionally emits `log/slog`
structured logs (request id, endpoint, puzzle hash, status, solveTimeMs). No external APM; Vercel
and localhost both capture stdout. `solveTimeMs` is measured inside the handler, excluding cold
start and transport (AUDIT P3).

**Parallelism and scale — what is NOT built.** Intra-puzzle cell/scan parallelism is not a
product feature; it ships only as a flagged, benchmarked *negative result* (AUDIT P2). No
horizontal scaling, no worker pool beyond goroutine-per-puzzle, no queue. The service scales by
process replication (stateless), which the deploy targets handle for free.

## Diagram

```
                        untrusted 81-char puzzle string
                                     │
                                     ▼
   ┌───────────────────────────────────────────────────────────────┐
   │  clients:  curl  ·  embedded UI (web/, embed.FS)  ·  future     │
   │            React dashboard  ── all speak the /v1 JSON contract  │
   └───────────────────────────────┬───────────────────────────────┘
                                    │  HTTP /v1  (contract.go)
                                    ▼
   ┌───────────────────────────────────────────────────────────────┐
   │  cmd/server/main.go  — mux on $PORT, slog, serves web/ at /     │
   └───────┬───────────────┬───────────────┬───────────────┬────────┘
           │ POST /v1/solve │ /v1/generate  │ /v1/validate- │ GET
           │                │               │      batch    │ /v1/health
           ▼                ▼               ▼               ▼
   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────┐
   │ internal/api │  │ internal/api │  │ internal/api │  │  health │
   │  solve hdlr  │  │  gen hdlr    │  │  batch hdlr  │  │  hdlr   │
   └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └─────────┘
          │                 │                 │ fan-out: 1 goroutine
          │ SolveRequest    │ GenerateRequest │ per puzzle (own Grid copy)
          ▼                 ▼                 ▼
   ┌──────────────┐  ┌────────────────────┐  │
   │internal/sudoku│ │ internal/generator │  │
   │ parse+validate│ │  symmetry xform +  │  │
   │ Grid/Candidate│ │  digger + BACKTRACK │  │
   └──────┬───────┘  │  uniqueness counter │  │
          │          └─────────┬──────────┘  │
          │  Grid              │ uses solver  │
          ▼                    │ as oracle    │
   ┌───────────────────────────▼──────────────▼──────────────────┐
   │  internal/solver  — cheapest-first ladder over Grid/Candidate │
   │  techniques/: singles → locked candidates → subsets →         │
   │  X-wing → swordfish → jellyfish → XY/XYZ/W-wing → colouring   │
   │  emits []Event (replayable)  ·  counts metric quartet         │
   └───────────────────────────┬──────────────────────────────────┘
                               │ SolveResult{status, solution,
                               │   events[], solveTimeMs,
                               │   eventCount, iterations,
                               │   candidateChecks}
                               ▼
                    JSON response  (/v1 contract, apiVersion)
```
Terminal artifact: a `SolveResult` (or `BatchResult`) JSON document — the solved grid, honest
status, replayable event log, and frozen metric quartet.

## Diagram (rendered)

Source: [`docs/diagrams/architecture.d2`](docs/diagrams/architecture.d2). Rendered SVG:
`docs/diagrams/architecture.svg`.

> The SVG is **unrendered** — `d2` is not installed on this machine. To render:
> `brew install d2` (or see https://d2lang.com), then
> `d2 --layout=elk -t 1 docs/diagrams/architecture.d2 docs/diagrams/architecture.svg`.
> Re-render whenever the `.d2` changes and commit both so GitHub shows it inline.
> Once rendered: `![Architecture](docs/diagrams/architecture.svg)`

## Contracts

### HTTP `/v1` API contract
- **What flows:** JSON request/response for `POST /v1/solve`, `POST /v1/generate`,
  `POST /v1/validate-batch`, `GET /v1/health`. Solve response:
  `{apiVersion, input, status, solved, solution, iterations, eventCount, candidateChecks,
  solveTimeMs, events[]}`. Errors: `{error, code}` with an appropriate HTTP status.
- **Where declared:** `internal/api/contract.go` (Go structs with `json` tags).
- **Producers / consumers:** produced by `internal/api` handlers; consumed by curl, the
  embedded UI, and the future React dashboard.
- **Versioning rule:** additive change → new **optional** field, same `/v1`; breaking change →
  new `/v2` prefix and a bumped `apiVersion`. The `/v1` shape is frozen after Phase 6 — this is
  the cross-iteration comparison surface and must not drift silently.
- **Blinded surfaces:** internal solver data structures (raw `Candidates` bitsets, generator
  internals) are never exposed — only the event log and the metric quartet. No PII exists.
- **Deferred?** No — declared.

### Solve contract
- **What flows:** `SolveRequest{grid Grid}` → `SolveResult{status SolveStatus, solved bool,
  solution string, events []Event, eventCount int, iterations int, candidateChecks int,
  solveTimeMs float64}`. `SolveStatus ∈ {solved, invalid_input, unsolvable, stalled}`.
- **Where declared:** `internal/solver` (e.g. `internal/solver/result.go`).
- **Producers / consumers:** produced by `internal/solver`; consumed by `internal/api`
  (all three POST handlers) and `internal/generator` (uniqueness oracle).
- **Versioning rule:** compile-checked struct change + updated tests; single binary, no
  independent consumer deployment.
- **Blinded surfaces:** none internally — full fidelity within the process.
- **Deferred?** No — declared.

### Event contract
- **What flows:** `Event{seq int, technique string, witnessCells []Cell, effect Effect,
  gridAfter string}` where `Effect` is either a `Placement{cell, value}` or
  `Eliminations[]{cell, candidate}`.
- **Where declared:** `internal/solver/event.go`.
- **Producers / consumers:** produced by each technique in `internal/solver/techniques`;
  consumed by API serialization and by the **replay test** that proves no-backtracking.
- **Versioning rule:** additive fields are safe (they surface in the `/v1` events array as new
  optional keys → additive at the HTTP layer too); removing/renaming a field is a breaking
  `/v2` change.
- **Blinded surfaces:** none.
- **Deferred?** No — declared. This contract is load-bearing: it is the mechanical proof of the
  logic-only guarantee (EVAL UC-2).

### Grid / Candidates contract
- **What flows:** `Grid` (81 cells, values 0–9, 0=blank) and `Candidates` (per-cell 9-bit
  candidate set). Parsing: `Parse(string) (Grid, error)` enforcing length 81, chars `0`–`9`/`.`,
  and no rule-violating givens.
- **Where declared:** `internal/sudoku` (`grid.go`, `candidates.go`, `parse.go`).
- **Producers / consumers:** produced by `internal/sudoku` parse; read/mutated by every
  technique and by the solver loop.
- **Ownership rule (shared mutable state):** within a single solve, **one goroutine** owns and
  mutates the `Grid`/`Candidates` (the sequential deduction cascade). In batch, **each puzzle
  gets its own independent `Grid` copy in its own goroutine** — there is **zero shared mutable
  state across goroutines**, which is what makes goroutine-per-puzzle trivially race-free.
- **Versioning rule:** compile-checked; internal.
- **Blinded surfaces:** none.
- **Deferred?** No — declared.

### Generate contract
- **What flows:** `GenerateRequest{difficulty string}` →
  `GeneratedPuzzle{puzzle string, difficulty string, grade string}`.
- **Where declared:** `internal/generator`.
- **Producers / consumers:** produced by `internal/generator`; consumed by the generate handler.
- **Versioning rule:** as Solve contract.
- **Blinded surfaces:** the internal backtracking uniqueness counter is never exposed — the
  contract emits only the finished puzzle and its grade. This is the deliberate seam that keeps
  "generation may backtrack" invisible to the "solver may not" guarantee.
- **Deferred?** No — declared.

### Batch contract
- **What flows:** `BatchRequest{puzzles []string}` → `BatchResult{results []BatchItem,
  solvedCount int, total int}`, `BatchItem{puzzle, solved, solveTimeMs, iterations,
  hardestTechnique}`. CRLF-safe parsing (trim `\r`, skip empty trailing lines). **Bounded:** the
  handler enforces `http.MaxBytesReader` on the body and a maximum `len(puzzles)` cap, rejecting an
  over-cap request with `413` / `invalid_input` before any solving begins.
- **Where declared:** `internal/api` (request/response) over `internal/solver` (per item).
- **Producers / consumers:** produced by the batch handler; consumed by curl / dashboard.
- **Versioning rule:** as the HTTP `/v1` contract.
- **Blinded surfaces:** none beyond the Solve contract's.
- **Deferred?** No — declared.

**Shared-infrastructure owners.** `cmd/server` owns observability (`log/slog`) and configuration
(env vars — `PORT`). `internal/api` owns the error envelope (`{error, code}`) used by every
handler. Transport is HTTP/JSON at the edge and in-process function calls internally. There is
**no** auth owner, **no** persistence owner, and **no** job/queue owner — those surfaces do not
exist by design (USERS refusals). No infrastructure surface is scattered.

## Components

- **`cmd/server/main.go` — process entry & wiring.** Builds the `http.ServeMux`, registers the
  `/v1` routes, mounts the embedded UI at `/`, listens on `$PORT`. Owns slog and config. Does
  NOT contain solving logic. Runtime: the Go binary itself. Consumes: nothing internal beyond
  wiring. Produces: the running server. Failure behavior: fails fast on bind error (bad `$PORT`);
  a panic in a handler is recovered by middleware and returned as a `500` `{error, code}`.
- **`internal/api` — HTTP handlers & contract.** Owns `contract.go` (the `/v1` types) and the
  `{error, code}` envelope. Each handler validates via `sudoku.Parse`, calls `solver`/`generator`,
  serializes `SolveResult`/`BatchResult`. Blinded from: solver internals (only the contract
  crosses). Produces: HTTP `/v1`, Batch contracts. Consumes: Solve, Generate contracts. Failure
  behavior: validation error → typed `invalid_input`; downstream panic → recovered `500`.
- **`internal/sudoku` — grid model & validation.** Owns `Grid`, `Candidates`, `Parse`. The trust
  boundary lives here. Produces/consumes: the Grid/Candidates contract. Failure behavior: `Parse`
  returns a typed error for any malformed or rule-violating input; never panics on user input.
- **`internal/solver` — the technique ladder.** The heart. Applies techniques cheapest-tier-first,
  row-major, over `Grid`/`Candidates`, emitting `[]Event` and counting the metric quartet. Owns
  the `SolveStatus` decision (semantics fixed by ADR-0011): `solved` (grid complete); `unsolvable`
  (an **in-tier** constructive contradiction — a cell driven to zero candidates — was *reached*; NOT
  a completeness claim that no solution exists); `stalled` (valid, no in-tier contradiction, no
  technique fires — deliberately covering too-hard, genuinely-unsolvable-but-not-constructively-
  refutable, and **non-unique** grids alike, since the solver must not solution-count); and
  `invalid_input` is decided upstream in `sudoku.Parse`. Blinded from: HTTP
  concerns. Uses model: none (pure Go, deterministic). Produces: Solve + Event contracts.
  Failure behavior: never guesses; on stall returns `status:"stalled"` with the partial event
  log. Module: `internal/solver/`, one technique per file under `techniques/`.
- **`internal/generator` — puzzle generation.** Builds a full solved grid by symmetry transforms
  of a base grid (no backtracking needed for the full grid), then digs clues, using an internal
  **backtracking solution-counter** to guarantee uniqueness and the `solver` as a difficulty
  oracle (the hardest technique the solver needs sets the grade). Blinded from callers: the
  backtracking counter is never surfaced. Produces: Generate contract. Failure behavior: bounded
  retry to hit a requested difficulty band; if it cannot after N attempts, returns the nearest
  achievable grade (documented in EVAL UC-3).

**Cross-component flows.** The **generation ↔ solve relationship** is the one place the two
epistemic rules meet: generation is *allowed* to backtrack (uniqueness counting), the solver is
*forbidden* to. The seam is the Generate contract — the generator consumes the solver as a pure
oracle and never leaks its own backtracking into the solver's path. The **batch fan-out** copies
the parsed `Grid` per puzzle into an independent goroutine; results are collected in input order.
The **replay verification** (a test-time flow, not a runtime one) re-executes each `Event` against
its pre-state to confirm the post-state, which is how the no-backtracking guarantee is mechanically
enforced (EVAL UC-2) rather than merely asserted. Current implementation gap an honest reader must
know: the labeled advanced fixture (AUDIT D-Q3) does not exist yet and is a Day-Zero build
prerequisite; until it lands, only the easy tier is eval-covered.

## Storage

**None.** The service is stateless by contract (USERS refusal). No database, no cache that
survives a request, no file writes at runtime. The only files read are compiled-in embedded UI
assets (`embed.FS`) and, in tests, the `puzzles.txt` / labeled fixtures under `testdata/`. There
is no schema, no migration mechanism, no access-control matrix — because there is no persistent
data. Transactional invariants are therefore trivially satisfied (nothing to split-brain).

## Observability

Primary signal is in-band: every solve response carries `solveTimeMs` (measured inside the
handler, solve-only), `eventCount`, `iterations` (main-loop scan passes), and `candidateChecks`
(total candidate-cell inspections) — the frozen metric quartet (ADR-0007) that is the entire
benchmark instrument. Secondary: `cmd/server` emits `log/slog` structured logs — request id,
endpoint, puzzle hash (for dedup, not identity), `status`, `solveTimeMs`. Model selection is N/A
(no models). Configuration is via env vars (`PORT`); no secrets at runtime. CI additionally
captures `go test -bench` numbers as the repeatable cross-iteration timing record.

## CI/CD topology

**Platform** — github-actions,vercel.
**Config file paths** — `.github/workflows/ci.yml` `.github/workflows/deploy.yml`.
**Secrets storage** — GitHub Actions Secrets: `VERCEL_TOKEN` (project-scoped, environment-guarded for production), `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID`.

**Triggers**
- `pr:opened` — run the test gate on every opened PR.
- `pr:updated` — re-run the test gate on new pushes to a PR.
- `push:master` — run the test gate on merges to master.
- `workflow-dispatch` — manual button that starts the gated production deploy.

**Gates**
- `test` · runs: `go test -race ./...` · pass/fail: exit code · trigger events: pr:opened, pr:updated, push:master. Mirrors CONTEXT.md §Test discipline.
- `vet` · runs: `go vet ./...` · pass/fail: exit code · trigger events: pr:opened, pr:updated, push:master.
- `build` · runs: `go build ./...` · pass/fail: exit code · trigger events: pr:opened, pr:updated, push:master. (Serves as the typecheck gate — the Go compiler is the type checker.)
- `coverage` · runs: `go test -coverprofile=cover.out ./... && go tool cover -func=cover.out` · pass/fail: total coverage ≥ 80% floor · trigger events: pr:opened, pr:updated, push:master. Mirrors CONTEXT.md §Test discipline coverage_command.

**Deploy topology**
- `production` · trigger: workflow-dispatch (manual button) · branch/tag: master · rollback: revert the offending commit then re-run the deploy workflow, or redeploy a prior Vercel build from the dashboard · env-vars: the `production` GitHub Environment holds `VERCEL_TOKEN`/`VERCEL_ORG_ID`/`VERCEL_PROJECT_ID` and a required-reviewer protection rule; the operator provisions them once.

**Deferred slots**
- `security-scan` — `govulncheck ./...` is deferred to `/nerdflow:impl` as an added gate (SECURITY.md provenance); not enforced at arch time.

**Manual steps requiring the operator's hands** (a coding agent cannot do these): create/link the
Vercel project and generate `VERCEL_TOKEN`; paste the three values into GitHub repo Secrets and
the `production` Environment; enable branch protection requiring the `test` check; configure the
`production` required-reviewer rule; approve each gated production deploy.

## Frontend Design Language

- **Surface** — embedded web SPA at `/` (`web/`, served via `embed.FS`): a 9×9 input grid to
  type or paste an 81-char puzzle, a Solve action, and a rendered solution + collapsible,
  technique-tagged event log.
- **Reference kit** — none available (`~/code/house-style` not present on this machine). Taste is
  defined inline below and pinned as ADR-0015; the first build phase touching the UI copies from
  this section verbatim.
- **Aesthetic** — "McKinsey-clean": near-monochrome, generous whitespace, one restrained accent,
  data-forward, no ornamentation. A tool, not a toy.
- **Copy recipe** — font: `system-ui, -apple-system, Segoe UI, Roboto, sans-serif` (NO web-font
  fetch — the page must be fully self-contained and fast, and must not hit an external host).
  Palette: text `#111` on background `#fafafa`; single accent `#1a56db` (the Solve action, and
  solved-cell digits); givens rendered in `#111`, solver-placed digits in the accent. Spacing:
  8px scale. Grid: the hero element — bold `2px` borders on the four 3×3 box seams, light `1px`
  borders on inner cells, square cells sized by `min()` for responsiveness. Motion: a single
  subtle fade on solution reveal; nothing else. Dark mode: deferred (light-only for v1).
- **Exceptions** — none; the grid and log are simple enough to hand-roll in a single `web/style.css`
  + `web/app.js` with no chart or component library.

## Known Tradeoffs

- **Advanced-tier eval depends on a not-yet-existing fixture.** The seed `puzzles.txt` is all
  easy tier (AUDIT D-Q3); the advanced techniques are unverified until a labeled per-tier fixture
  is curated at build. Mitigation: it is a named Day-Zero prerequisite in EVAL.md, not a silent
  assumption. Accepted because the operator supplies more puzzles post-build anyway.
- **Constructive-only tier will return `stalled` on the hardest published puzzles.** Puzzles
  above ~ER 8.5 (AI Escargot, Easter Monster class) need forcing chains/AIC, which are excluded
  (ADR-0001). Mitigation: this is correct behavior, not a bug — `status:"stalled"` is honest.
  Accepted as a direct consequence of the no-backtracking rule.
- **Cross-deployment speed is only comparable within one host class.** Vercel free-tier CPU is
  throttled and variable (AUDIT P1). Mitigation: serious benchmarking runs on localhost; the
  `/v1/health` host/version labels let the dashboard segment by host class. Accepted.
- **Intra-puzzle parallelism ships as a negative result, not a feature.** The PRD's "solve cells
  simultaneously for speed" won't beat single-threaded on a 9×9 (AUDIT P2). Mitigation: shipped
  and measured honestly behind a flag. Accepted; the measurement is itself the deliverable.
- **Generation uses backtracking internally.** The whole-product "no backtracking" ideal is
  scoped to the solver only (ADR-0003). Mitigation: the Generate contract blinds the counter; the
  solver path stays provably pure. Accepted as the pragmatic industry-standard approach.
- **UI is light-mode only, no external kit.** Dark mode and a design-system kit are deferred.
  Mitigation: the inline recipe is fully specified so the build is not left to improvise.
  Accepted — the operator asked for minimal design input.

### Security tradeoffs accepted at Phase 5c (append-only; Source: SECURITY.md)
Accepted 2026-07-20 after the Kaladin security review (VERDICT: PASS, no blocking). Each is safe
because the service has no data, no identity, and no runtime secret to protect, and request work is
bounded (`MaxBytesReader` + batch-length cap + sub-millisecond solves).
- **No caller authentication.** Public solver by charter; no data or side effects to guard. **Source:** SECURITY.md §F-1.
- **No per-caller rate limit.** No identity to key on; per-request work is bounded. **Source:** SECURITY.md §F-2.
- **No global concurrency ceiling.** Stateless bounded-work requests; concurrency is platform-managed. **Source:** SECURITY.md §F-3.
- **No VERCEL_TOKEN rotation procedure.** Project-scoped, instantly revocable; guards no runtime data. **Source:** SECURITY.md §F-4.
- **TLS min-version delegated to the platform (API + browser).** Vercel terminates HTTPS; no credentials/PII in transit. **Source:** SECURITY.md §F-5, §F-6.
- **No standalone ranked threat-model section.** AUDIT S1 + USERS §NOT-Do enumerate the surface; zero data assets make the implicit model adequate. **Source:** SECURITY.md §F-7.
- **No alertable security signals.** No data assets; slog to stdout; $0 demo without monitoring infra. **Source:** SECURITY.md §F-8.
