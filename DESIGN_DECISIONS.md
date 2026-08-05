# Design Decisions — sudoku-flow

ADR-style record of every architectural decision fixed during `/nerdflow:arch` on 2026-07-20.
Ordered by dependency: foundational rulings first.

## ADR-0001: Logic is constructive-only; forcing chains and Nishio are banned
**Status:** Accepted (2026-07-20)
**Context:** The PRD's #1 rule is "absolutely no backtracking / no guess-then-revert." Domain
research (AUDIT) found that the strongest logic techniques — forcing chains, Nishio, AIC — are
*logically sound* but *operationally* do exactly what the ban forbids: assume a candidate,
propagate, revert on contradiction. "All known logic techniques" and "no trial-and-error" are
therefore partially contradictory.
**Decision:** The solver uses only **constructive / direct** eliminations — techniques whose
every step is a positive deduction from present state, never "assume X, see if it breaks."
Forcing chains, Nishio, and all contradiction-based reasoning are excluded.
**Alternatives considered:** (a) Allow sound chains up to AIC/ALS — rejected: much more code
(fights ADR-0002's leanness axis) and AIC still reads as guess-shaped in the event log. (b) Allow
everything except literal backtracking — rejected: violates the spirit of the operator's rule and
would make the event log unfalsifiable as a no-guessing proof.
**Consequences:** Caps the technique tier (ADR-0002). Some very hard puzzles will honestly return
`status:"stalled"` (ARCHITECTURE Known Tradeoffs). Makes the event log a clean, positive-deduction
trace — which is what lets EVAL UC-2 mechanically prove the guarantee.

## ADR-0002: Technique tier stops at Singles→Wings + basic fish + simple colouring
**Status:** Accepted (2026-07-20)
**Context:** The PRD says "implement all known logic algorithms," but that set is open-ended
(dozens of ALS/chain variants that never fire on realistic puzzles), and it directly conflicts
with the PRD's own leanness comparison axis (fewer/cleaner lines). Research showed a ~12-technique
library clears essentially all published puzzles including the seed 25.
**Decision:** Implement a defined, ordered ladder: naked single, hidden single, locked candidates
(pointing/claiming), naked/hidden pairs-triples-quads, X-wing, swordfish, jellyfish, XY-wing,
XYZ-wing, W-wing, simple colouring. "All known algorithms" is reframed as "a defined library
sufficient to solve the target puzzle class."
**Alternatives considered:** (a) subsets + X-wing only — rejected: too weak, stalls on genuinely
hard puzzles. (b) extend to ALS/AIC — rejected: inconsistent with ADR-0001 and bloats the codebase.
**Consequences:** Bounds solving power and code size. Defines exactly which puzzles legitimately
stall. Sets the grade bands (ADR-0013). Requires an advanced fixture to test the upper tiers
(EVAL / AUDIT D-Q3), since the seed set only exercises singles.

## ADR-0003: No-backtracking bans the solve path only; generation may backtrack internally
**Status:** Accepted (2026-07-20)
**Context:** Difficulty-graded generation classically needs a backtracking solution-counter to
guarantee a unique solution — in direct tension with the no-backtracking rule.
**Decision:** The ban governs the **benchmarked solver**. The **generator** may use a standard
backtracking uniqueness-counter internally; it is an unbenchmarked utility. The Generate contract
blinds the counter so it never leaks into the solver's guarantee.
**Alternatives considered:** (a) logic-only generation via dig + logic-oracle that reverts rejected
digs — rejected as more complex/slower and still a form of search. (b) drop generation from v1 —
rejected: UC-3 is wanted and generation supplies graded test inputs the seed set lacks.
**Consequences:** Keeps the thing being measured provably pure while enabling UC-3. Introduces the
one place the two epistemic rules meet, mediated by the Generate contract (ARCHITECTURE
cross-component flows). Cross-links ADR-0001.

## ADR-0004: Uniqueness-assuming techniques (Unique Rectangles, BUG) are excluded
**Status:** Accepted (2026-07-20)
**Context:** URs/BUG only produce correct results if the grid is guaranteed to have exactly one
solution. UC-1 accepts arbitrary POSTed grids, which may be non-unique.
**Decision:** Exclude URs/BUG from the solver entirely.
**Alternatives considered:** include them gated behind a validated-unique flag — rejected: requires
a solution-count validation path (reintroducing backtracking on the solve side) for marginal power.
**Consequences:** The solver never mis-"solves" a non-unique grid. Simpler codebase. Generation
still guarantees uniqueness via its own counter (ADR-0003) — a separate path.

## ADR-0005: Deploy to Vercel as a demo; benchmark on localhost
**Status:** Accepted (2026-07-20)
**Context:** Vercel Hobby has a hard 10s per-request cap and throttled, variable CPU (AUDIT P1),
which collides with large-batch (UC-4) and makes deployed speed comparison noisy. The operator
mostly runs on localhost and wanted cheap/free ephemeral deployment.
**Decision:** Vercel free tier is the zero-cost, shareable demo (via the Go server preset — same
`main.go`). All serious timing, parallelism, and large-batch benchmarking runs on localhost.
**Alternatives considered:** (a) Cloud Run free tier — real server, no cap, but a Dockerfile + GCP
project of setup; deferred as available if deployed benchmarking is later wanted. (b) both platforms
— rejected as premature.
**Consequences:** Batch on Vercel must be size-bounded. Cross-deployment speed is host-class-scoped
(ARCHITECTURE Known Tradeoffs). Keeps setup trivial and cost at $0.

## ADR-0006: Parallelism is goroutine-per-puzzle; intra-puzzle parallelism is a measured negative result
**Status:** Accepted (2026-07-20)
**Context:** A 9×9 solve is sub-millisecond and its productive deductions are a sequential cascade;
intra-puzzle cell parallelism cannot amortize goroutine overhead (AUDIT P2).
**Decision:** Ship real parallelism as one goroutine per puzzle in the batch path. Also build a
flagged intra-puzzle scan-parallel variant and publish its benchmark as a *measured negative
result*, not a speed claim.
**Alternatives considered:** (a) batch-only, skip the experiment — rejected: loses the honest
"we measured it" NerdFlow demonstration. (b) feature intra-puzzle parallelism as a speedup —
rejected: research says it loses to single-threaded.
**Consequences:** UC-5 delivers useful linear scaling plus an honest experiment. Requires
`go test -race` in CI (the only concurrency in the system). Per-goroutine Grid copy → zero shared
mutable state (ARCHITECTURE Grid contract).

## ADR-0007: Freeze the benchmark metric quartet
**Status:** Accepted (2026-07-20)
**Context:** Cross-iteration comparison is the project's raison d'être; if "iteration" is
undefined the benchmark axis is uncalibrated.
**Decision:** Every solve reports four frozen counters: `solveTimeMs` (wall clock, measured inside
the handler, solve-only); `eventCount` (productive deductions in the log); `iterations` (main-loop
scan passes — one cheapest-first sweep of the ladder per pass); `candidateChecks` (total
candidate-cell inspections). Primary comparison = `solveTimeMs` + `eventCount`; the other two are
diagnostic. Definitions are frozen with the `/v1` contract.
**Alternatives considered:** (a) scan-passes + events only — rejected: loses the operator's
"looked at cells N times" signal. (b) candidate-inspections as the single headline — rejected: most
implementation-sensitive, noisiest across differently-structured builds.
**Consequences:** Cross-NerdFlow-iteration numbers are comparable. Ties the metric definitions into
the frozen contract; changing them is a `/v2` event.

## ADR-0008: HTTP layer is stdlib net/http
**Status:** Accepted (2026-07-20)
**Context:** ~4 endpoints, JSON in/out, stateless; Go 1.22+ `ServeMux` supports method+path
routing. The golanger agent is stdlib-forward.
**Decision:** Use stdlib `net/http`; no chi/gin/echo.
**Alternatives considered:** a framework — rejected: adds a dependency for middleware/route trees
this service does not need, and works against the leanness axis.
**Consequences:** Zero HTTP dependencies. Cross-links ADR-0009.

## ADR-0009: One binary serves localhost and Vercel via the Go server preset
**Status:** Accepted (2026-07-20)
**Context:** Vercel's Go server preset runs a standard binary listening on `$PORT`, so no
serverless-function adapter is needed (AUDIT A1).
**Decision:** A single `cmd/server/main.go` (`http.ListenAndServe(":"+os.Getenv("PORT"), mux)`)
serves both localhost and Vercel byte-identically.
**Alternatives considered:** `/api/*.go` serverless functions + rewrites — rejected: an extra
adapter layer that makes local and deployed diverge, hurting comparison fidelity.
**Consequences:** `main.go` must read `$PORT` (no hardcoded port). Localhost and deployment are
identical. Cross-links ADR-0005, ADR-0008.

## ADR-0010: Versioned `/v1` contract with a self-labeling health endpoint
**Status:** Accepted (2026-07-20)
**Context:** A future React dashboard compares *multiple* deployments, so the JSON shape must be
identical across iterations (AUDIT A5).
**Decision:** Prefix routes with `/v1`, include an `apiVersion` field in responses, and expose
`GET /v1/health` returning `{status, goVersion, apiVersion}` so a deployment self-identifies.
Additive change → optional field; breaking change → `/v2`.
**Alternatives considered:** unversioned routes — rejected: silent drift would invalidate
cross-iteration comparison.
**Consequences:** The `/v1` shape is frozen after Phase 6. Cross-links ADR-0007 (metrics live in
this contract).

## ADR-0011: `solved:false` is refined into four explicit statuses
**Status:** Accepted (2026-07-20)
**Context:** The PRD boolean conflates three genuinely different outcomes; debugging why a solve
didn't complete needs them distinguished.
**Decision:** Responses carry `status ∈ {solved, invalid_input, unsolvable, stalled}` alongside a
`solved` boolean, with these precise, non-overclaiming semantics:
- `invalid_input` = malformed or rule-violating **givens** — length ≠ 81, illegal characters, or a
  duplicate digit in a row/column/box among the givens. Decided at `sudoku.Parse`, before the solver
  runs. This is the *only* status that is a completeness claim about validity, and it is decidable.
- `unsolvable` = during constructive solving a cell is driven to **zero candidates** — an **in-tier
  constructive contradiction**. This is emphatically **NOT a completeness claim** that the puzzle has
  no solution in the abstract; it is "the constructive tier reached a contradiction." A genuinely
  no-solution puzzle whose contradiction is *not* reachable by the in-tier techniques will **not**
  reach `unsolvable`.
- `stalled` = the grid is valid, no constructive contradiction was reached, but no technique in the
  tier fires and the grid is incomplete. This bucket **deliberately conflates three cases the
  constructive-only solver cannot separate**: (i) solvable but above the tier; (ii) genuinely
  unsolvable but not constructively refutable in-tier; (iii) **non-unique** (valid givens, multiple
  solutions). The solver does not — and by ADR-0001/0004 must not — run the solution-counting needed
  to tell these apart.

**Alternatives considered:** (a) two outcomes / boolean-only — rejected: less diagnostic. (b) make
`unsolvable` a true completeness claim — rejected: that requires exhaustive solution-counting
(backtracking on the solve path), which ADR-0001 forbids; the honest contract is a partial detector
plus an explicit `stalled` catch-all.
**Consequences:** The contract is diagnostic without overclaiming. The operator benchmarks
`stalled`-vs-`unsolvable` understanding that `unsolvable` means "constructively refuted in-tier" and
`stalled` is the honest, deliberately-mixed remainder. EVAL fixtures for `unsolvable` must therefore
be constructed so the contradiction *is* in-tier reachable (EVAL notes this and the under-sampling
it implies); a non-unique grid is expected to return `stalled`, never `unsolvable`/`invalid_input`.
`stalled` is the honest face of ADR-0001/0002's tier cap. Cross-links ADR-0001, ADR-0004.

## ADR-0012: Deterministic technique order — cheapest-first, row-major
**Status:** Accepted (2026-07-20)
**Context:** Reproducibility run-to-run is required or the benchmark is meaningless.
**Decision:** Apply techniques cheapest-tier-first; within a technique, scan cells/units
row-major. No randomness in the solve path.
**Alternatives considered:** any-order / first-found — rejected: non-reproducible traces.
**Consequences:** Identical input → byte-identical event log and metric quartet. Underpins EVAL
UC-1/UC-2 golden and replay tests.

## ADR-0013: Difficulty grade = hardest technique required, Sudoku-Explainer ordering
**Status:** Accepted (2026-07-20)
**Context:** Difficulty must be well-defined relative to a named, ordered technique set.
**Decision:** Grade a puzzle by the highest tier the solver was *forced* to use (no cheaper move
available at that moment), named against Sudoku Explainer's tier ordering, bucketed into Easy /
Medium / Hard / Expert.
**Alternatives considered:** clue-count or ad-hoc labels — rejected: not reproducible or externally
comparable.
**Consequences:** Grades are externally meaningful and drive UC-3 generation targeting. Depends on
ADR-0002's ladder.

## ADR-0014: Minimal UI embedded in the binary via embed.FS
**Status:** Accepted (2026-07-20)
**Context:** The operator wanted an optional simple viewer with minimal design input; a separate
frontend build/repo is unwarranted.
**Decision:** Serve a small static SPA from `web/` via `embed.FS` at `/`, from the same binary.
**Alternatives considered:** no UI (rejected: deployment isn't viewable/playable); separate
frontend repo (rejected: premature, that's the future dashboard).
**Consequences:** One artifact serves API + UI everywhere. Design language is ADR-0015.

## ADR-0015: Inline "McKinsey-clean" design language (no external kit)
**Status:** Accepted (2026-07-20)
**Context:** `~/code/house-style` is not present; the operator asked to keep design input minimal.
**Decision:** Adopt the inline recipe in ARCHITECTURE §Frontend Design Language — system-ui sans
(no web-font fetch), near-monochrome `#111`/`#fafafa` + one `#1a56db` accent, 8px spacing, grid as
the hero, light-mode only, no external assets.
**Alternatives considered:** a design-system kit (none available); dark mode (deferred).
**Consequences:** Self-contained, fast, CSP-friendly UI. The build copies this recipe verbatim
rather than improvising. Cross-links ADR-0014.

## ADR-0016: CI/CD platform — GitHub Actions test gate + Vercel manual deploy
**Status:** Accepted (2026-07-20)
**Context:** The operator wants CI that runs the app's own tests, blocks merge on failure, then a
manual-gated deploy to a cheap/free target (Vercel).
**Decision:** GitHub Actions runs the test gate on PRs and master; a manual `workflow_dispatch`
plus a `production` Environment required-reviewer rule gates the Vercel deploy. See ARCHITECTURE
§CI/CD topology.
**Alternatives considered:** GitLab CI / auto-deploy on merge — rejected: repo is on GitHub and the
operator explicitly wants a manual deploy gate.
**Consequences:** Merge is blocked on green tests; production ships only on human approval. Some
setup steps require the operator's hands (tokens, protection rules). Cross-links ADR-0005, ADR-0017.

## ADR-0017: CI/CD gate set — test(-race), vet, build, coverage(80%); security-scan deferred
**Status:** Accepted (2026-07-20)
**Context:** Which checks must pass before merge, for a stateless Go solver whose correctness and
race-freedom are the whole product.
**Decision:** Gates are `go test -race ./...`, `go vet ./...`, `go build ./...`, and coverage with
an **80%** floor. `govulncheck` (security-scan) is deferred to `/nerdflow:impl` as an added gate.
Typecheck is subsumed by `build` (the Go compiler is the type checker).
**Alternatives considered:** no coverage floor (rejected: the operator ratified 80%); include
security-scan now (deferred: arch-time review already covers the tiny surface; scan lands at impl).
**Consequences:** `-race` is mandatory because ADR-0006 introduces goroutines. Coverage floor keeps
the benchmarked solver honestly tested. Cross-links ADR-0006, ADR-0016.

## ADR-0018: Two-tier per-technique ship gate — exact-hardest where isolable, fires-and-sound where not
**Status:** Accepted (2026-07-21, build-time amendment)
**Source:** build session P-2
**Context:** EVAL.md's per-technique ship gate (introduced to close the AUDIT §D-Q3 coverage gap, and
made blocking by the Halliday review) requires ≥3 fixtures that *require* each of the 11 shipped
techniques as the **exact hardest** step (floor: stalls without it; ceiling: solves with it). At P-2
build, the solver was independently verified sound (no technique ever removes the oracle's true
digit across all fixtures) and complete-for-ladder, yet only 14/35 sourced fixtures satisfied the
strict gate. The failure is a domain reality, not a solver defect: several constructive techniques —
**jellyfish** above all, and the exact-ceiling wings/subsets — are near-redundant (a jellyfish is
almost never a puzzle's *unique* bottleneck within a constructive-only ladder; when it applies, a
cheaper technique usually also cracks the puzzle, or chains beyond the ladder are required). Sourced
SudokuWiki example puzzles merely *contain* the featured technique; their true hardest step is
higher. Generate-and-grade (now feasible using the built solver as the difficulty oracle) resolves
the common techniques but cannot isolate the near-redundant ones. This is the same diminishing-returns
reality the domain research flagged (X2) and that ADR-0002 accepted when it bounded the ladder.
**Decision:** Split the ship gate into two tiers, recorded per technique in `testdata/advanced/MANIFEST.md`:
- **Isolable techniques** — keep the strict gate unchanged: ≥3 fixtures each, floor (stalls capped
  below) + ceiling (solves capped at) proving the technique is the exact required hardest step.
- **Un-isolable techniques** (empirically determined: generate-and-grade cannot produce ≥3
  exact-ceiling puzzles within the constructive ladder) — accept ≥1 fixture on which the technique
  **fires** (emits a witnessed `Event` in the solve log) and **every one of its eliminations passes
  the replay soundness check** (never removes a candidate that is the true solution digit). This
  proves the implementation is exercised and provably non-guessing, without the impossible
  requirement that it be a puzzle's unique bottleneck.
Which techniques fall in the second tier is justified by the generate-and-grade evidence recorded in
the MANIFEST, not chosen for convenience.
**Alternatives considered:** (a) drop the near-redundant techniques (jellyfish etc.) from the ladder
(ADR-0002 amendment) — rejected: the operator chose to keep the full ladder; the techniques are
implemented and sound. (b) grind generate-and-grade against the strict gate for all 11 — rejected:
likely unsatisfiable for jellyfish regardless of effort, burning build rounds to re-reach this point.
**Consequences:** Preserves the gate's real intent — no shipped technique ships unexercised or
unsound, and the no-backtracking replay proof holds for **every** technique. Relaxes only the
"unique bottleneck" clause for techniques where it is domain-infeasible. The solver still implements
the full ADR-0002 ladder. EVAL.md's strict gate remains authoritative for isolable techniques; this
ADR supersedes it only for the MANIFEST-listed un-isolable set. Cross-links ADR-0002 (ladder),
ADR-0013 (grading), EVAL §Datasets and fixtures.

## ADR-0019: Seed corpus sectioned by tier — singles ORIGINAL (25) frozen; advanced MEDIUM/HARD/VERY-HARD (30) added
**Status:** Accepted (2026-07-31, post-build amendment)
**Source:** post-ship benchmark-corpus growth
**Context:** `puzzles.txt` served two roles at once: the D-Q3 **singles-only** acceptance fixture (every
one of the 25 seeds solves by naked/hidden singles alone) and the project's solve-timing benchmark
input. The load-bearing no-backtracking proof `TestAC3_Solver_ReplayFromInputProvesNoBacktracking`
depends on the singles-only property directly — its replay `switch` accepts only `naked_single` /
`hidden_single` and hard-fails (`default:`) on any advanced technique. Growing the corpus with harder
puzzles (to benchmark the solver across difficulty tiers — the project's stated purpose) therefore
could **not** simply append lines: advanced puzzles fire ladder techniques (locked candidates, pairs,
X-wing, …) that AC-3's singles-tier proof rejects by design. D-Q3's "25 singles seeds" is a real
invariant baked into AC-1/AC-3/AC-4/AC-6 and the api-layer AC tests, not a cosmetic label.
**Decision:** Section `puzzles.txt` by tier with `# === NAME ===` headers and route tests by section:
- **ORIGINAL (unlabeled) — 25 singles seeds:** unchanged and frozen. `loadPuzzles`/`loadSeed` return
  exactly this section, so AC-1/AC-3/AC-4/AC-6 and the batch/handler ACs keep their `== 25` assertions
  and singles-tier bodies verbatim. The D-Q3 proof is preserved intact, not rewritten.
- **MEDIUM / HARD / VERY-HARD — 30 advanced puzzles:** generator-produced (`/v1/generate` at
  medium/hard/expert), each verified 81-char, unique (D-Q2), and logic-solvable. Proven no-backtracking
  by a new `TestAdvancedSeeds_ReplayProvesNoBacktracking` that routes each through the existing
  `replayAdvancedProvesForced` helper (every placement forced, every elimination sound against the
  brute-force oracle, solution == oracle) — the same proof the per-technique fixtures use (ADR-0018).
All `puzzles.txt` loaders (`loadPuzzles`, `loadSeed`, `benchGrids`) skip `#`/blank lines so headers
never parse as grids; the benchmark loader spans the full corpus.
**Alternatives considered:** (a) rewrite AC-3 to accept all 13 techniques over a merged 55-puzzle set —
rejected: collapses the clean singles-tier proof and changes the meaning of the load-bearing test for
no benefit. (b) keep the 30 in a sibling `puzzles_extended.txt`, leaving `puzzles.txt` at 25 — viable
and lower-churn, but the operator chose to grow the seed file itself; sectioning achieves that while
keeping the singles proof frozen.
**Consequences:** One corpus file now carries both tiers with test coverage that *strengthens* (advanced
no-backtracking is now proven over 30 real graded puzzles, not only synthetic per-technique fixtures).
D-Q3 remains authoritative for the ORIGINAL section. Cross-links ADR-0001/0012 (no-backtracking),
ADR-0002 (ladder), ADR-0018 (advanced fires-and-sound proof), AUDIT §D-Q2/D-Q3, EVAL §Datasets and fixtures.

## ADR-0020: Vercel deploy via a serverless Handler adapter + public `app` package (corrects ADR-0009)
**Status:** Accepted (2026-08-04, post-ship correction)
**Source:** first real Vercel deploy
**Context:** ADR-0009 assumed `@vercel/go` runs `cmd/server/main.go` as a long-lived Go server (the "Go
server preset"). That premise is **false**, discovered on the first deploy. Two distinct failures:
1. `@vercel/go` only builds **serverless functions** — it requires a file exporting
   `func Handler(w http.ResponseWriter, r *http.Request)` and rejected `package main`/`func main()`
   outright: *"Could not find an exported function in cmd/server/main.go"*. It never ran arbitrary servers.
2. After adding an entrypoint, `@vercel/go` compiles it as a **module-less `command-line-arguments`
   build**, so the entrypoint file **cannot import an `internal/` package**: *"use of internal package
   … not allowed"*. Critically, a local `go build ./...` does **not** reproduce this — Go retains the
   file's real module path locally — so the CI/build gates pass green while the Vercel build fails. Only
   a real deploy exercises this path.
**Decision:** Keep the "one codebase serves localhost and Vercel through one shared handler" intent, but
implement it correctly:
- **`api/index.go`** (`package handler`) — the serverless entrypoint exporting `func Handler(w, r)`,
  delegating to a package-init-built `app.NewHandler()`. Imports **only the public `app` package**.
- **`app` package** (public, in-tree) — exposes `NewHandler() http.Handler`, composing the full stack
  (`logRequests → SecurityHeaders → CORS → Recover → MaxBytes → routes`), moved out of `package main`.
  It imports `internal/api` for the leaf handlers + middleware — allowed, because `app` is a real in-tree
  package (the `internal/` rule only bars the module-less entrypoint, not a normal package one hop down).
- **`cmd/server/main.go`** — unchanged behavior; `run()` now builds via `app.NewHandler()` for local
  `ListenAndServe`. `internal/api` leaf handlers/middleware are untouched.
- **`vercel.json`** targets `api/index.go`.
Behavior is byte-identical across both entrypoints (same middleware order, routes, access log, body cap).
**Alternatives considered:** (a) run the binary as a server on Vercel — impossible; Vercel is
serverless-only. (b) move all packages out of `internal/` so the entrypoint can import them directly —
rejected: a large, invasive change to the frozen layout when a single thin public `app` package suffices.
**Consequences:** The app deploys as a Vercel serverless function; the same code still runs as a local
server via `cmd/server`. **Verified live** on the first successful deploy: `/v1/health` → 200,
`/v1/solve` → solved, SPA served, at `https://sudoku-flow.vercel.app`. Standing caveat recorded: local
Go builds cannot catch `@vercel/go`'s module-less `internal/` restriction — a real deploy is the only
check for that class of change. Supersedes ADR-0009's deploy mechanism; cross-links ADR-0008 (stdlib
routing), ADR-0016 (Vercel manual deploy).

## ADR-0021: UI feature — `GET /v1/puzzles` catalog endpoint + `grade` on `/v1/solve` (additive)
**Status:** Accepted (2026-08-04, post-ship feature)
**Source:** embedded-UI enhancement (puzzle dropdown, solve stepper, statistics)
**Context:** The embedded SPA gained three capabilities: a dropdown to load the test-file puzzles, a
step-through view of the solve event log, and a statistics window (technique histogram, counts,
difficulty). Two of these need data the API did not expose: the browser cannot read the repo-root
`puzzles.txt`, and the solve response carried no difficulty grade. The stepper and the rest of the
statistics derive entirely from data already in the `/v1/solve` response (`events[]` with `gridAfter`,
witnesses, placements/eliminations, plus the metric quartet), so only two **additive** API changes
were required.
**Decision:**
- **New `GET /v1/puzzles`** — returns the catalog as `{sections:[{name,puzzles:[...]}, …]}` with display
  names `Original / Medium / Hard / Very Hard`. The data is **`//go:embed`ed** (`internal/api/puzzles.txt`,
  a byte-identical copy of the repo-root fixture) — never read from the filesystem, because the Vercel
  serverless runtime has no working-directory access (same lesson as ADR-0020). A drift-guard test
  (`bytes.Equal` of the embedded copy vs `../../puzzles.txt`) fails loudly if the two ever diverge. The
  copy is deliberately NOT placed under `internal/api/web` (that embed.FS is served by the `GET /`
  file server, which would expose it as a raw download).
- **`grade` field on `SolveResponse`** (`json:"grade"`, no `omitempty`) — populated on the solved path via
  the existing `solver.Grade` (ADR-0013). Tradeoff accepted: `solver.Grade` re-solves internally (the
  technique→band map is unexported), so the solved path solves twice; sub-millisecond and off the timed
  `solveTimeMs` window. A future cleanup could export the band mapping or have `Grade` accept a `Result`.
- **Frontend** (`internal/api/web/*`, not a frozen artifact): dropdown, event stepper with cell
  highlighting (placement/witness/elimination), and the statistics window incl. a ladder-ordered
  technique histogram. All DOM writes remain `textContent`/`createElement` only (SECURITY.md **F-11** —
  no `innerHTML`). The grid border scheme was also reworked to be symmetric (every cell owns its
  right+bottom 1px interior line; all four outer edges and the two box seams a uniform 2px), fixing an
  earlier top/left-vs-right/bottom unevenness.
**Consequences:** Both changes are purely additive — no existing endpoint behaviour or field changed, so
prior AC tests are untouched. New tests cover the catalog handler, the embed drift guard, and the `grade`
field. Verified locally end-to-end (dropdown → solve → grade/histogram/stats → step highlights). Cross-links
ADR-0008 (stdlib routing), ADR-0013 (grading), ADR-0020 (serverless embedding), SECURITY.md F-11.
