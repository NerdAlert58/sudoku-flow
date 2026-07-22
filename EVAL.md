# Evaluation Strategy — sudoku-flow

This document defines how we know the system meets the bar. Acceptance is not subjective; every
UC has a measurable success signal, a threshold, and a regression trigger. `/nerdflow:impl`
treats this as a hard input — every piece that ships a UC must include eval-coverage in its
acceptance criteria, and is not Done until its eval target hits the ship threshold.

## Eval matrix

| UC | Success signal | How measured | Dataset / fixture | Ship threshold | Regression trigger |
|---|---|---|---|---|---|
| **UC-1 Solve** | Returns a rule-valid, correct solution for every solvable input; honest `status` otherwise | Golden-file test (output == brute-force oracle solution) + property test (solution satisfies all 27 row/col/box constraints) | `puzzles.txt` (25) + labeled advanced fixture | 25/25 seed solved AND 100% of labeled fixtures at-or-below the target tier solved; `invalid_input` fixtures all return `invalid_input`; in-tier-refutable `unsolvable` fixtures all return `unsolvable`; **non-unique and above-tier fixtures return `stalled`** (per ADR-0011 — the solver does not claim these `unsolvable`) | any seed puzzle fails; a solvable fixture returns a non-`solved` status; or a status mismatch against the fixture's labeled expected status |
| **UC-2 Explain** | The event log replays *from the original input all the way to the returned solution* — every placement *forced* by its named technique given prior state, no unexplained cell (the mechanical no-backtracking proof) | Replay property test: starting from the parsed input grid, apply each `Event` in order against its pre-state, assert each recorded post-state, and assert the **final grid is byte-identical to the returned `solution` with zero cells placed by anything other than a named, witnessed technique** | `puzzles.txt` + labeled advanced fixture | 100% of `solved` responses replay input→solution with zero unexplained placements and a byte-identical final grid | any event fails replay, the replayed final grid ≠ returned `solution`, or any placement lacks a valid witness (this failure = a hidden guess) |
| **UC-3 Generate** | Generated puzzle is valid, uniquely solvable, and graded at (or nearest to) the requested difficulty | Property test: brute-force counter confirms exactly one solution; solver's hardest-required technique lands in the requested band | generated on the fly (seeded for reproducibility in tests) | 100% of generated puzzles are unique; ≥90% hit the requested difficulty band (nearest-achievable with bounded retry) | any generated puzzle is non-unique, or band-hit rate drops below 90% |
| **UC-4 Batch** | Correct per-puzzle results + summary; CRLF-safe parsing of a `.txt`-sourced list | Golden test over `puzzles.txt` submitted as one batch; assert per-item results match single-solve results | `puzzles.txt` | `solvedCount == 25` and every item matches its single-solve result | `solvedCount` drops below 25, or a CRLF/trailing-newline parse regression appears |
| **UC-5 Parallel** | Goroutine-per-puzzle batch is race-free and produces results identical to serial | `go test -race` on the batch path + a benchmark comparing serial vs. parallel batch; assert result-equality | `puzzles.txt` + labeled advanced fixture | `-race` clean AND parallel results byte-identical to serial | race detector fires, or parallel and serial results diverge |

## Datasets and fixtures

### `puzzles.txt` (seed — exists today)
- **Location:** repo root, `C:\Users\Scott\Git\sudoku-flow\puzzles.txt`.
- **Size / coverage:** 25 puzzles, one 81-char line each, empty cell = `0`, CRLF line endings,
  no trailing newline, 24–29 clues each. All unique and well-formed. **All solvable by naked
  singles alone — easy tier only.**
- **Owner / curation:** committed seed; treated as a smoke test and the UC-1/UC-4 happy path.
- **Version control:** committed; changes are reviewed like code.

### Labeled advanced fixture (must be curated before P-0 — Day-Zero prerequisite)
- **Location:** `testdata/advanced/` (per-tier files), created during build.
- **Ship gate — every shipped ADR-0002 technique must have required coverage.** For **each** of the
  eleven non-trivial shipped techniques — hidden single, locked candidates (pointing/claiming),
  naked subset, hidden subset, X-wing, swordfish, **jellyfish**, XY-wing, **XYZ-wing**, **W-wing**,
  **simple colouring** — at least **3 puzzles that *require* that technique** (cannot be solved by
  any cheaper tier). A technique with zero required puzzles is an unverified capability and **fails
  this eval**: no shipped technique ships unproven. (Naked single is covered by the seed set.)
- **Status coverage:** additionally include (a) ≥3 grids that **stall** the constructive-only tier
  (exercise `status:"stalled"`), (b) ≥3 grids with an **in-tier-reachable constructive contradiction**
  (exercise `status:"unsolvable"` — see the note below), (c) ≥3 **non-unique** grids, each labeled
  expected `stalled` (per ADR-0011 the solver returns `stalled`, NOT `unsolvable`, for non-unique),
  and (d) ≥3 **malformed / rule-violating** grids labeled expected `invalid_input`.
- **`unsolvable` caveat (ADR-0011):** the constructive tier can only emit `unsolvable` when it
  *reaches* a zero-candidate cell. `unsolvable` fixtures must be constructed so the contradiction is
  in-tier reachable; genuinely-unsolvable grids whose contradiction is not constructively reachable
  legitimately return `stalled`, and are **under-sampled by design** — this is a known limit, not a
  test to force.
- **Why it exists:** the seed set exercises none of the advanced techniques the project scopes
  (AUDIT D-Q3). Without this fixture, passing `puzzles.txt` gives false confidence that the upper
  tiers work. This closes the coverage gap for **every** shipped tier deliberately rather than
  inheriting it silently.
- **Owner / curation:** the build author curates; each line is annotated with its hardest-required
  technique and its expected `status` (the labels). Ground-truth solutions come from the brute-force
  oracle.
- **Version control:** committed under `testdata/`; additions reviewed like code.

## Judges and rubrics

**None.** The system is fully deterministic — no LLM, no LLM-as-judge. Correctness is decided by
exact comparison to a computed ground truth and by structural property assertions, not by a rubric
or a model's opinion.

## Ground-truth process

Ground truth is *computed*, not hand-labeled: a **backtracking brute-force solver lives in test
code only** (never in the shipped solve path — consistent with ADR-0003/ADR-0001) and produces the
unique solution for any well-formed grid. New advanced-fixture cases are added by (1) selecting or
constructing a puzzle that requires a specific technique tier, (2) running the brute-force oracle to
record its unique solution and confirm uniqueness, (3) annotating the line with its hardest-required
technique. Disagreements about a puzzle's "hardest required technique" are resolved by running the
solver with techniques disabled above the claimed tier and confirming it still solves — if it does,
the claimed tier is too high. Labels and solutions live under `testdata/` in version control.

## How EVAL.md is consumed

Every piece in the delivery plan that ships a UC must cite the relevant Eval matrix row in its
acceptance criteria. A piece is not Done until its eval target hits the ship threshold — "build
complete" alone does not satisfy. The UC-2 replay test is non-optional and load-bearing: it is the
only mechanical enforcement of the project's #1 rule (no backtracking), so a build that ships the
solver without a passing replay test has not satisfied UC-1 or UC-2 regardless of other green tests.
