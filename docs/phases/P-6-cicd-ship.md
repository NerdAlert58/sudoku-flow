# Phase P-6 — CI/CD & Ship

**ID:** P-6 · **Status:** In progress (started 2026-07-21, baseline 42de783) · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

> Pre-gate piece (CI/CD wiring + docs; no new app tests — the CI workflow runs the existing suite). Test-coverage gate (5.0/5b.6/5b.7) skipped. Some ACs (real CI run, deploy) require human-only external setup (Vercel token, GitHub secrets, branch protection).

## Goal
The project ships: GitHub Actions test gate blocks merge, a manual-gated Vercel deploy exists,
`vercel.json` uses the Go server preset, supply-chain hardening (govulncheck + SHA-pinned actions) is
in place, and the README documents run/deploy.

## Entry gate
P-0..P-5 `Done` (there is a complete, tested application to gate and deploy).

## Dependencies
- P-0..P-5 — the full application (server, solver, generator, batch, UI) that CI gates and deploys.

## Allow-list (source)
- `.github/workflows/**`
- `vercel.json`
- `README.md`
- `.gitignore`

## Allow-list (tests)
- (none — this phase wires CI/CD and docs; it runs the existing test suite, it does not author new app tests)

## Read-only context
- ARCHITECTURE.md §CI/CD topology (platform, config paths, secrets, triggers, gates, deploy topology); §A1 (Vercel Go server preset)
- DESIGN_DECISIONS.md §ADR-0016 (CI/CD platform), §ADR-0017 (gate set + 80% coverage floor + govulncheck deferred here)
- SECURITY.md §F-13 (govulncheck gate), §F-15 (Actions SHA-pinning, no `pull_request_target`)
- CONTEXT.md §Test discipline (test/coverage commands), §CI/CD, §Deployment discipline

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
This is the plan's mandated CI/CD piece.
- **AC-C1:** Every path in `ARCHITECTURE.md §CI/CD topology`'s Config file paths exists in the repo (`.github/workflows/ci.yml`, `.github/workflows/deploy.yml`). **Source:** ARCHITECTURE.md §CI/CD topology.
- **AC-C2:** A real `pr:opened`/`pr:updated` event on this piece's PR produces pass/fail signals for every gate in `ARCHITECTURE.md §CI/CD topology`'s Gates — `test` (`go test -race`), `vet`, `build`, `coverage` (evidence: link to the CI run URL and job summary). **Source:** ARCHITECTURE.md §CI/CD topology.
- **AC-C3:** The `coverage` gate uses the same LCOV parser + diff-line floor as local `/nerdflow:build` Phase 5b.6 (Go coverage converted to LCOV), enforcing the 80% floor. **Source:** ARCHITECTURE.md §CI/CD topology.

## Suggested steps
1. `ci.yml`: on `pr:opened`/`pr:updated`/`push:master`, run `go vet`, `go build`, `go test -race -coverprofile`, convert coverage to LCOV, enforce the 80% floor, and run `govulncheck ./...`.
2. `deploy.yml`: `workflow_dispatch` (manual) targeting a `production` GitHub Environment (required-reviewer), deploying via the Vercel CLI with `VERCEL_TOKEN`/`VERCEL_ORG_ID`/`VERCEL_PROJECT_ID`.
3. `vercel.json`: `"framework":"go"` (Go server preset) so the same `main.go` serves the deployment.
4. Pin every GitHub Action to a full commit SHA; ensure no workflow uses `pull_request_target` with secret access.
5. README: how to run locally (`PORT=8080 go run ./cmd/server`), the `/v1` endpoints, and the manual deploy + required human setup steps.

## Acceptance criteria
- **AC-1:** `.github/workflows/ci.yml` and `.github/workflows/deploy.yml` and `vercel.json` exist. **Source:** ARCHITECTURE.md §CI/CD topology.
- **AC-2:** On a PR, CI runs `go vet`, `go build`, `go test -race`, and coverage with an enforced 80% floor, each producing a pass/fail check that gates merge (evidence: CI run URL). **Source:** DESIGN_DECISIONS.md §ADR-0017.
- **AC-3:** CI runs `govulncheck ./...` as a gate and `go.sum` is committed. **Source:** SECURITY.md §F-13.
- **AC-4:** Every GitHub Action in both workflows is pinned to a full commit SHA, and no workflow uses `pull_request_target` (or any untrusted-input trigger) with access to `VERCEL_TOKEN`. **Source:** SECURITY.md §F-15.
- **AC-5:** The production deploy is gated behind a manual `workflow_dispatch` and the `production` Environment's required-reviewer rule (no auto-deploy on merge). **Source:** DESIGN_DECISIONS.md §ADR-0016; ARCHITECTURE.md §CI/CD topology.
- **AC-6:** `vercel.json` selects the Go server preset so the deployed binary is the same `cmd/server` that runs locally. **Source:** ARCHITECTURE.md §A1 / ADR-0009.
- **AC-7:** README documents local run, the `/v1` endpoints, and the human-only setup steps (Vercel token, GitHub secrets, branch protection, required-reviewer). **Source:** ARCHITECTURE.md §CI/CD topology (manual steps).

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
# CI itself is the check: the PR's Actions run must show all gates green
```

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
# after opening the PR:
gh pr checks   # all gates green
```

## Human verification
1. Open a PR and confirm the required `test`/`coverage`/`vet`/`build` checks appear and gate merge. Why it matters: the merge gate is the point of CI.
2. Trigger `deploy.yml` and confirm it pauses for manual approval before deploying. Why it matters: your explicit manual-gate requirement.
3. Confirm the deployed `/v1/health` responds and self-labels. Why it matters: the deployment is real and identifiable.

## Regression check
Re-run all prior phases' automated checks — CI must run the full suite green.

## Exit gate
- `ci.yml`, `deploy.yml`, `vercel.json` exist; `go.sum` committed.
- A PR shows `go vet`/`go build`/`go test -race`/coverage(≥80%)/`govulncheck` all producing gating signals (CI run URL recorded).
- Every Action pinned to a commit SHA; no `pull_request_target` with secret access.
- Deploy is manual-gated (`workflow_dispatch` + required reviewer); no auto-deploy on merge.
- `vercel.json` uses the Go server preset.
- README documents run, endpoints, and human setup steps.

## Implementation notes (filled in by the builder)
> Record decisions and cross-cutting discoveries here.

**Session 2026-07-21 (P-6 build, baseline 42de783, branch phase/p-6-cicd-ship).** CI/CD wiring +
docs only; no Go source, tests, `go.mod`, or contract touched. Files written strictly within the
source allow-list: `.github/workflows/ci.yml`, `.github/workflows/deploy.yml`, `vercel.json`,
`README.md`, `.gitignore`.

Decisions:
- **Gate = one job each (not steps-in-one-job).** `ci.yml` runs `vet` / `build` / `test` /
  `coverage` / `govulncheck` as five separate jobs so each surfaces as its own required status
  check, letting branch protection require each gate individually (AC-2, ARCHITECTURE §Gates). The
  phase text said "distinct named step"; separate jobs are the strictly stronger reading and satisfy
  both. Cheap here — pure-stdlib build/test is fast.
- **Action SHAs pinned (F-15), fetched live from the GitHub API:**
  `actions/checkout` → `11bd71901bbe5b1630ceea73d27597364c9af683` (v4.2.2);
  `actions/setup-go` → `3041bf56c941b39c61721a86cd11f3bb1338122a` (v5.2.0);
  `actions/setup-node` → `39370e3970a6d050c480ffad4ff0ed4d3fdee5af` (v4.1.0). All 12 `uses:` verified
  `@<40-hex>`. Each carries a trailing `# vX.Y.Z` comment.
- **`go-version-file: go.mod`** on every setup-go job — single source of truth, no version drift vs.
  the `go 1.26` directive.
- **`permissions: contents: read`** at the top of both workflows (least privilege). `deploy.yml`
  uses `workflow_dispatch` only (no auto-deploy) + `environment: production` (required-reviewer
  gate). No `pull_request_target` anywhere; `VERCEL_TOKEN` scoped per-step via `env:`.
- **Coverage floor** parses `go tool cover -func=cover.out | tail -1` (field 3, strip `%`) and fails
  via `awk` when `< 80`; LCOV emitted with `gcov2lcov` (mirrors CONTEXT §Test discipline). Parse
  logic verified locally against a real profile: repo total = **92.2%** → PASS.
- **`vercel.json`** uses the `@vercel/go` builder against `cmd/server/main.go` with a catch-all
  route — the concrete, current realization of ADR-0009's "Go server preset" (runs `package main` as
  a `$PORT` server; there is no literal `"framework":"go"` value in Vercel's framework enum). Strict
  JSON forbids comments, so the preset-version-sensitivity note lives in README §Deployment + here:
  if a future Vercel Go runtime changes the server-config surface, `vercel.json` is the one file to
  revisit.
- **`go.sum`:** the module has **zero third-party requires** (pure stdlib), so no `go.sum` is
  generated or committed — there is nothing for it to record. AC-3's "`go.sum` committed" is
  vacuous here, not a miss; `govulncheck` still gates (it scans the Go toolchain/stdlib) and becomes
  a real dependency gate the instant a `require` is added.

Local verification (all green, no Go changed): `go build ./...`, `go vet ./...`, `go test ./...`
(api/generator/solver/sudoku all ok). Both workflows parse as valid YAML (jobs enumerated), no tab
indentation; `vercel.json` is valid JSON. `govulncheck ./...` ran clean → **No vulnerabilities
found** (exit 0).

Requires human GitHub/Vercel setup to verify (cannot be automated): AC-2 real PR CI run, AC-4 branch
protection wiring, AC-5 required-reviewer approval + real deploy, AC-6 live Vercel build, AC-7 the
deployed `/v1/health` responding. See README §Human setup steps.

## Deliverable line
`Phase 6 ready for review` OR `Phase 6 blocked because: <one sentence>`.

## Health check
`GET https://<vercel-deployment-url>/v1/health -> 200 with body match /"ok"/`

## Rollback command
`vercel rollback ${LAST_DEPLOYMENT_URL}` (human-triggered; nerdflow never auto-executes rollback)

## Env vars required
- `PORT` (provided by the platform / shell)
- `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` (CI/deploy only, from GitHub Secrets)
