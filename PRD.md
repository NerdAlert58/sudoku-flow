# PRD — sudoku-flow

## Origin note
Flow is testing the updates we've made to NerdFlow and to the new agent for repo-specific
code writing. I want to build another Sudoku app that has the puzzles.txt as part of it.
The objective is to have NerdFlow write the entire repo — a backend API written in Golang
that uses the Golanger agent to do the code writing, following the same spec-driven
development NerdFlow normally does, and hopefully using Golanger for the test-driven
development aspects too. The goal: a Go repo that can create a new puzzle and solve Sudoku
puzzles at varying difficulty levels because it implements all the known logic-based
algorithms for solving Sudoku. I'd also like to explore parallelization with workers so
multiple cells can potentially be solved simultaneously for speed. No guesswork at all —
all logic-based. No going to the first open cell, suggesting a number, attempting a solve,
and backtracking. Absolutely no backtracking. I want to deploy it somewhere simple and
cheap via CI/CD, minimal setup and teardown since I won't leave it up long — mostly I'll
play with it on localhost, but it must be deployable so I can compare it. Part of the point
is an API shaped so a separate React repo can pull from multiple deployments of apps like
this one, letting me compare solution correctness and speed across NerdFlow iterations to
show improvement over time.

## Goal
Let the NerdFlow operator POST any valid Sudoku and get back a provably logic-only solution
with a full solve-event log, fast enough to benchmark NerdFlow iterations against each other.

## User
**The NerdFlow operator (you).** You run the nerdflow slash commands in Claude Code CLI,
driving arch → impl → build on a fresh repo, and you expect the orchestrator to employ the
Golanger agent as a subagent so context is preserved on a small app. Your day-to-day tools
are the Claude Code CLI, the nerdflow commands, a terminal, and GitHub. You direct the work
and step away (lunch, overnight) while the AI builds continuously until the product is done.

You cannot tolerate the solver being wrong or cheating: a returned solution that violates
Sudoku rules, or any solution reached by backtracking / trial-and-error guessing, is a
failure regardless of speed. You will judge iterations of NerdFlow on solve-timing
consistency for identical puzzles and on code-quality improvement (fewer lines, cleaner) —
but only when correctness and the logic-only constraint are fully preserved.

## What the user does today
Before this exists, you validate NerdFlow's new Golanger agent by hand: feeding it work,
eyeballing whether the generated Go is any good, and having no consistent, measurable Go
artifact to benchmark successive NerdFlow iterations against. You want a repeatable target —
give the API a puzzle list, watch it solve every puzzle with pure logic, and later hand it
fresh lists to confirm the logic generalizes.

## Intervention moment
You POST an unsolved puzzle (the same 81-character string format used per line in
puzzles.txt) to the running Go API — on localhost, or on a deployment — and read back JSON:
the solved grid plus metadata (iteration count, solve time, solved boolean) and a
step/event-by-event solve log. A separate React comparison dashboard pulling from multiple
deployments is a possible future, second intervention moment — out of scope for this repo.

## Use cases
- **UC-1.** POST 81-char unsolved grid → JSON: solved grid + `{iterations, solveTimeMs, solved: bool}`.
- **UC-2.** POST a grid → ordered solve-event log; each deduction tagged with the technique
  used (naked single, hidden single, X-wing, etc.) and the cell(s) affected, ending in the
  full solution. Iteration count reflects total work (e.g. cell scans), not just the
  productive deduction steps.
- **UC-3.** Generate a new puzzle at a requested difficulty level → return unsolved grid.
- **UC-4.** Batch-validate a list of puzzles (like puzzles.txt) → report which were solved
  logic-only.
- **UC-5.** (Exploratory) Parallel/worker solving of independent cells/candidate-eliminations
  to minimize solve time.

## In scope
- A Go backend HTTP API accepting an 81-char puzzle string and returning solution + metadata + event log.
- A full suite of logic-based solving techniques (all known non-backtracking algorithms).
- Difficulty-graded puzzle generation.
- Batch validation against puzzles.txt (all 25 must be solvable by the finished app).
- Exploratory worker-based parallelization for solve speed.
- An optional simple, modern, "McKinsey-clean" web UI to enter/view a puzzle, so a deployment
  is playable without extra spec input.
- CI/CD that runs the app's own tests, blocks merge on failure, and gates deploy behind a
  manual step.

## Non-goals (explicit refusals)
- Will NOT use backtracking or any trial-and-error / guessing approach — logic-based deduction only.
- Will NOT return a guessed or partial-but-fabricated solution; if pure logic stalls, it
  reports `solved: false` rather than guessing.
- Will NOT persist puzzles or state across requests (stateless API).
- Will NOT implement auth or multi-tenancy.
- Will NOT ship the React comparison dashboard in this repo (future, separate repo).
- Will NOT reference or be informed by any other Sudoku app the operator has written — this
  iteration must stand on its own so cross-iteration comparison stays clean.

## Success criteria
- Solves every puzzle in the provided lists (all 25 in puzzles.txt at minimum) with zero backtracking.
- Every solution is reproducible from its event log, and each event names the technique used.
- Each solve returns an iteration/step count and wall-clock solve time.
- Any puzzle not solvable by pure logic is flagged `solved: false` rather than guessed.
- The API contract is stable enough that an external client (future React dashboard) can call
  multiple deployments uniformly and compare correctness + speed.
- Comparison axes for NerdFlow iterations: consistent solve timing on identical puzzles, and
  reduced code size / improved quality without loss of correctness or violation of the logic-only rule.

## Constraints
- **Regulatory:** None.
- **Integration target:** Standalone API; any client meeting the endpoint contract may call it.
  Optional simple built-in UI. Future external React dashboard consumes multiple deployments.
- **Tech stack:** Latest Go. The Golanger agent must write the Go code (this is itself under
  evaluation), employed as a subagent. HTTP framework left to Golanger's stdlib-forward default;
  no forbidden deps specified.
- **Deployment:** Cheap/free-tier, ephemeral, minimal setup/teardown; mostly localhost. CI/CD
  runs the test suite, blocks merge until green, then a manual-gated deploy step to "production"
  (assume Vercel free/cheap tier). Exact Go-on-Vercel shape (serverless functions vs. server) is
  a design decision for /nerdflow:arch.
- **Budget envelope:** Near-zero. Keep it free for now; no custom domain until the idea earns it.
- **Team:** Solo — you directing, NerdFlow + Golanger doing the build.

## Deadline
No fixed calendar deadline. AI-driven continuous build until the product is complete and the
task can be called done; the operator directs pacing (including AFK/overnight runs).

## Domain context
Sudoku constraint-propagation solving using the full family of human-logic techniques —
naked/hidden singles, locked candidates (pointing/claiming), naked/hidden pairs-triples-quads,
X-wing, swordfish, XY-wing, and other known logic-based algorithms — explicitly excluding
backtracking and any guess-then-revert strategy. Difficulty grading derives from which tier of
technique a puzzle requires.

## Data / fixtures available today
- puzzles.txt: 25 puzzles, one 81-char grid per line (already committed as the seed). The
  finished app must solve all of these logic-only.
- Additional puzzle lists will be supplied by the operator only AFTER the implementation is
  complete, as independent testing — not during the build.
- Correctness is validated by Sudoku-rule conformance reached via logged logic steps (no
  external known-solution answer file is assumed).

## Reference materials
- None — deliberately. No other apps or prior solutions are to inform this build, to keep
  cross-iteration comparison clean.
