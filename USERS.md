# USERS — sudoku-flow

> Source-of-truth rule: ARCHITECTURE.md must trace every capability back to a use case in
> this document. If a capability cannot point to a UC here, it should not exist.

## The User

**The NerdFlow operator.** The person running the `/nerdflow` pipeline to build this repo and
then benchmarking the Go API it produces, across successive NerdFlow iterations.

- **Setting:** Claude Code CLI on this repository, driving `/nerdflow:arch` → `impl` → `build`,
  then evaluating the generated Go service from a terminal.
- **Volume:** repeated end-to-end builds of this same app — each a fresh NerdFlow iteration —
  compared against prior runs for solve timing and code quality.
- **Tools today:** Claude Code CLI, a terminal, GitHub, `go test` / `go test -bench`, `curl`,
  and (for a deployment) the Vercel dashboard.
- **Constraints — cannot tolerate being wrong about:** a returned solution that violates
  Sudoku rules, or any solution reached by guessing / backtracking. Either is a failure
  regardless of how fast it ran. Will abandon the tool the moment it silently cheats the
  logic-only guarantee — the guarantee, not the speed, is the product.

**Why this user, narrowly defined.** Choosing "the operator benchmarking NerdFlow" — rather
than "a Sudoku hobbyist" or "a puzzle-app end user" — forecloses an entire consumer surface:
no accounts, no saved games, no human-facing hints, no leaderboards, no difficulty-curve
onboarding. It foregrounds the opposite: *measurable, reproducible, honest* solving. The
event log, the frozen iteration/timing counters, the stable versioned contract, and the
`solved:false` honesty all exist to serve cross-iteration comparison, not entertainment. This
narrowing is what lets the architecture stay a lean stateless service instead of an app.

## The Workflow

### The 90 seconds before the system enters the day

A NerdFlow build of `sudoku-flow` has just finished. `go test ./...` is green and the server
binary is running on `localhost:8080`. The operator wants to know two things about this
iteration: *does it solve honestly, and how does it compare to the last build?* Without the
system, the options are: (1) hand-read the generated Go and guess at quality — slow and
subjective; (2) mentally trace a solve — impractical; (3) diff line counts between builds
with no correctness anchor — misleading, because fewer lines that cheat the logic rule is a
regression, not an improvement. Each option costs confidence: there is no consistent,
measurable artifact that says "this build is correct AND this is how it performed."

### The system enters the day

The operator runs `curl -X POST localhost:8080/v1/solve -d '{"puzzle":"<81 chars, 0=blank>"}'`
(or pastes the string into the embedded UI at `/`). In well under a second the response
returns: the solved grid, `status` (`solved` / `invalid_input` / `unsolvable` / `stalled`),
the frozen metric quartet (`solveTimeMs`, `eventCount`, `iterations`, `candidateChecks`), and
an ordered, technique-tagged, replayable event log. The operator reads the log to confirm the
solution was reached by *logic* — every step names its technique — and reads the metrics to
compare against the prior iteration. For a whole set they POST `/v1/validate-batch` with the
25-line `puzzles.txt` and read `solvedCount`.

**End-of-session boundary:** the HTTP response closes the interaction. **No state persists** —
no saved puzzle, no solve history, no session identity. Every request is fully independent and
reproducible; a second identical request yields a byte-identical trace.

## Use Cases

### UC-1. Solve
**Prompt shape:** `POST /v1/solve` with `{"puzzle": "<81-char string, 0 or . = blank>"}`.
**What the system does:** validates the input, runs the constructive-only technique ladder
cheapest-tier-first, and returns the solved grid + `status` + the metric quartet. Status semantics
(precise, per ADR-0011): `invalid_input` = malformed/rule-violating givens (decided before solving);
`unsolvable` = the constructive tier *reached* a zero-candidate contradiction (a partial detector,
NOT a claim the puzzle has no solution in the abstract); `stalled` = valid, no in-tier contradiction,
but the tier cannot finish — deliberately covering "too hard for the tier," "genuinely unsolvable but
not constructively refutable," AND "non-unique" alike, because the solver must not run the
solution-counting that would separate them. It never guesses.
**Why an API is right (vs. an interactive app):** the deliverable is a machine-callable,
benchmarkable contract that a dashboard can hit uniformly across multiple deployments. An
interactive-app shape would bury the comparison surface behind a human UI.

### UC-2. Explain (event log)
**Prompt shape:** the same `POST /v1/solve` — the log is always included.
**What the system does:** returns an ordered list of events; each names its technique, the
witness cells/candidates that justify it, and its effect (a placement or candidate
eliminations), with the grid state after. Replaying the events reproduces the solution.
**Why this shape is right:** the event log is the *only mechanical proof* that the solve was
logic-only. A bare solution grid could have been produced by a hidden backtracker; a replayable,
technique-forced trace cannot. This UC is the load-bearing enforcement of the project's #1 rule.

### UC-3. Generate
**Prompt shape:** `POST /v1/generate` with `{"difficulty": "easy|medium|hard|expert"}`.
**What the system does:** produces a valid, uniquely-solvable puzzle graded at (or nearest to)
the requested difficulty, returned as an 81-char string plus its assigned grade. Uniqueness is
guaranteed by an internal backtracking counter (generation-path only — see refusals).
**Why an endpoint is right:** generation feeds the solver its own graded inputs for testing and
gives the operator difficulty-controlled material beyond the all-easy seed set. It is a batch
utility, not an interactive design tool.

### UC-4. Batch validate
**Prompt shape:** `POST /v1/validate-batch` with `{"puzzles": ["<81>", …]}`.
**What the system does:** solves each puzzle (one goroutine per puzzle), returning per-puzzle
`solved` / `solveTimeMs` / `iterations` / `hardestTechnique` plus `solvedCount` and `total`.
CRLF-safe when the list came from a `.txt` file.
**Why a batch endpoint is right:** it automates the "solve every puzzle in the list logic-only"
success criterion — the operator's core validation loop — in one call.

### UC-5. Parallel solve (exploratory, honest)
**Prompt shape:** UC-4 with parallelism enabled (goroutine-per-puzzle), plus a flagged
intra-puzzle scan-parallel variant used only for measurement.
**What the system does:** solves a batch concurrently (real, linear speedup), and separately
runs an intra-puzzle scan-parallel solve whose benchmark is published as a *measured negative
result* — it does not beat the single-threaded loop on a 9×9.
**Why this shape is right:** the honest parallelism story is at the puzzle/request level; the
intra-puzzle experiment is valuable precisely as a documented "we measured it and it doesn't
pay" NerdFlow demonstration, not as a speed claim.

## What the System Will NOT Do

- **Will NOT guess or backtrack on the solve path.** Logic deduction only; if the technique
  tier is insufficient it returns `status:"stalled"`. Enforced by the UC-2 replay test.
- **Will NOT return a fabricated or partial-but-guessed solution.** An honest `solved:false`
  with the events it *did* legitimately achieve, never an invented completion.
- **Will NOT persist** any puzzle, solution, session, or history. The service is stateless by
  contract.
- **Will NOT implement auth, accounts, or multi-tenancy.** No identity surface exists.
- **Will NOT use uniqueness-assuming techniques (Unique Rectangles, BUG) on solve input.**
  They can mis-"solve" a non-unique POSTed grid; excluded entirely from the solver.
- **Will NOT ship the React comparison dashboard** in this repo. It is a separate future repo
  that consumes this API's frozen contract.
- **Will NOT claim intra-puzzle cell parallelism as a speedup.** It is shipped only as a
  measured negative result.

## Capability → Use Case Traceability

| Capability | Justified by | Required data / dependencies |
|---|---|---|
| Puzzle parse + validate (81-char, `0`=blank, `.` accepted, CRLF-safe) | UC-1, UC-4 | none |
| Constructive-only technique ladder (Singles→Wings + basic fish + simple colouring) | UC-1, UC-2, UC-4 | frozen technique tier (ADR-0002) |
| Technique-tagged, replayable event log | UC-2 | technique ladder |
| Frozen metric quartet (solveTimeMs, eventCount, iterations, candidateChecks) | UC-1, UC-4, UC-5 | frozen metric definitions (ADR-0007) |
| Difficulty grader (hardest-tier-required, Sudoku-Explainer ordering) | UC-3, UC-4 | technique ladder + named grade bands |
| Puzzle generator (backtracking uniqueness counter + clue digger) | UC-3 | generation-path backtracking ruling (ADR-0003) |
| Batch runner (goroutine-per-puzzle) | UC-4, UC-5 | technique ladder; per-goroutine Grid copy |
| HTTP/JSON `/v1` contract + `/v1/health` | UC-1–4 | none |
| Embedded minimal UI | demo convenience for UC-1 | `/v1` contract |

**Capabilities the architecture should NOT include (no UC justifies them yet):**
- Auth / accounts / sessions — no UC; refused.
- Persistence / solve history — no UC; refused.
- Human-facing hint/teaching mode — no UC (operator, not learner).
- Forcing chains / Nishio / AIC / ALS techniques — excluded by the constructive-only ruling
  (ADR-0001); they resemble the banned assume-and-revert pattern.
- Unique Rectangles / BUG — excluded (ADR-0004).
- Intra-puzzle cell parallelism as a product feature — measurement-only (ADR-0006).

## Demo-Data Reality Check

`puzzles.txt` (25 puzzles, all naked-single / easy tier, `0`=blank, CRLF) exists today and
demos UC-1, UC-2, and UC-4 immediately — but it validates **only the easy tier**. Passing all
25 proves the plumbing and the singles path, not the advanced techniques the project scopes.
To close this gap a **labeled advanced fixture set** — with ≥3 puzzles *requiring* **each** shipped
technique (hidden singles, locked candidates, naked subsets, hidden subsets, X-wing, swordfish,
jellyfish, XY-wing, XYZ-wing, W-wing, simple colouring — no shipped technique left unverified), plus
grids that exercise `stalled`, in-tier-refutable `unsolvable`, non-unique (expected `stalled`), and
`invalid_input` — is curated during build and is a Day-Zero prerequisite for EVAL.md (see EVAL's
per-technique ship gate). UC-3 generation provides a second, difficulty-controlled
source of inputs once the solver exists. No answer key is needed: the backtracking brute-force
test oracle labels ground-truth solutions.
