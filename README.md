# sudoku-flow

A constructive, **no-backtracking** Sudoku solver, generator, and batch validator served as a
single Go binary — with an embedded web UI. Every solve is a mechanical proof: the solver only ever
places a digit or eliminates a candidate because a named human technique *forced* it, never by guess
-and-check. The full, replayable event log is the evidence.

- **Zero third-party dependencies** — pure Go standard library (`net/http` routing, `log/slog`,
  `embed.FS`).
- **One binary, two homes** — the same `cmd/server` reads `$PORT` and serves byte-identically on
  localhost and on Vercel (ADR-0009).
- **Self-labeling API** — `GET /v1/health` reports its Go/API version so multiple deployments can be
  compared honestly.

## The guarantee

- **Constructive-only solve path.** The solver applies a deterministic, cheapest-technique-first
  ladder (ADR-0012). It **never backtracks** on the solve path — no guessing, no solution-counting.
- **Four honest statuses** (ADR-0011): `solved`, `invalid_input` (malformed/rule-violating givens),
  `unsolvable` (an *in-tier constructive contradiction* — not an absolute no-solution claim), and
  `stalled` (valid but no in-tier technique fires — this deliberately includes above-tier,
  non-constructively-refutable, and non-unique puzzles).
- **The metric quartet** on every solve — `iterations`, `eventCount`, `candidateChecks`,
  `solveTimeMs` — is the benchmark instrument (ADR-0007), identical run-to-run for identical input.

> Generation uses backtracking *internally* for uniqueness (ADR-0003); the counter is blinded from
> the response. The "no backtracking" ideal is scoped to the **solve** path, which stays provably pure.

## API — `/v1`

All responses carry `"apiVersion": "1"`. Errors use `{ "error": "...", "code": "..." }`.

| Method & path | Request body | Success body (abridged) |
| --- | --- | --- |
| `GET /v1/health` | — | `{ status, goVersion, apiVersion }` |
| `POST /v1/solve` | `{ "puzzle": "<81 chars>" }` | `{ apiVersion, input, status, solved, solution, iterations, eventCount, candidateChecks, solveTimeMs, events[] }` |
| `POST /v1/generate` | `{ "difficulty": "easy\|medium\|hard\|expert" }` | `{ puzzle, difficulty, grade }` |
| `POST /v1/validate-batch` | `{ "puzzles": ["<81 chars>", ...] }` | `{ apiVersion, results[], solvedCount, total }` |

A puzzle string is 81 characters, row-major; use `0` or `.` for empty cells. Each `events[]` entry is
a replayable step: the technique that fired, the witness cells, the placement or eliminations, and the
grid after the step.

## Embedded UI

`GET /` serves a self-contained SPA (`web/`, embedded via `embed.FS`, ADR-0014): a 9×9 grid to type or
paste a puzzle, a Solve action, and the rendered solution with a collapsible, technique-tagged event
log. No external assets, no web fonts — it loads fully offline.

## Run locally

Requires Go 1.26+.

```bash
PORT=8080 go run ./cmd/server
# then open http://localhost:8080/
```

Solve a puzzle with `curl`:

```bash
curl -s http://localhost:8080/v1/solve \
  -H 'Content-Type: application/json' \
  -d '{"puzzle":"53..7....6..195....98....6.8...6...34..8.3..17...2...6.6....28....419..5....8..79"}'
```

Health check:

```bash
curl -s http://localhost:8080/v1/health   # -> {"status":"ok",...}
```

## Tests

```bash
go test ./...                                   # full suite
go test -race ./...                             # race detector (matches CI)
go test -coverprofile=cover.out ./... && go tool cover -func=cover.out | tail -1
```

## CI/CD

**`ci.yml`** runs on every PR to `master` and on pushes to `master`. Each gate is a separate,
independently-required status check (ADR-0017):

- `vet` — `go vet ./...`
- `build` — `go build ./...` (the Go compiler is the typecheck gate)
- `test` — `go test -race ./...`
- `coverage` — profile → LCOV (`coverage.lcov`), **failing the job below an 80% line-coverage floor**
- `govulncheck` — `govulncheck ./...` supply-chain scan (SECURITY.md F-13)

Every GitHub Action is pinned to a full commit SHA and no workflow uses `pull_request_target`
(SECURITY.md F-15). Configure branch protection on `master` to require these checks.

**`deploy.yml`** is the production deploy — **manual only** (`workflow_dispatch`, no auto-deploy on
merge, ADR-0016). It targets the `production` GitHub Environment, whose required-reviewer rule pauses
the run for human approval before it deploys to Vercel via the Vercel CLI.

### Deployment (Vercel Go server preset)

`vercel.json` uses the `@vercel/go` builder against `cmd/server/main.go`, so Vercel builds and runs the
*same* `main.go` that runs locally — a server binary listening on `$PORT` (ADR-0009 / A1). The
`builds`/`routes` shape is the current, explicit realization of the "Go server preset"; it is
preset-version-sensitive — if a future Vercel runtime changes the Go-server config surface, this is the
one file to revisit.

## Human setup steps (cannot be automated)

These require the operator's hands — a coding agent cannot do them:

- [ ] **Create/link the Vercel project** and generate a `VERCEL_TOKEN` (project-scoped).
- [ ] **Add repo Secrets** `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` (put them on the
      `production` Environment so they are environment-guarded).
- [ ] **Enable branch protection** on `master` requiring the `vet` / `build` / `test` / `coverage` /
      `govulncheck` checks before merge.
- [ ] **Configure the `production` Environment** with a required-reviewer protection rule.
- [ ] **Approve each gated deploy** when `deploy.yml` is dispatched.

## Layout

```
cmd/server/        # the single binary: $PORT, mux, middleware chain (main.go)
internal/api/      # the frozen /v1 JSON contract + per-route HTTP handlers
web/               # embedded SPA served at /
.github/workflows/ # ci.yml (test gate), deploy.yml (manual Vercel deploy)
vercel.json        # Go server preset
```

## Rollback

```bash
vercel rollback ${LAST_DEPLOYMENT_URL}   # human-triggered; never automated
```
