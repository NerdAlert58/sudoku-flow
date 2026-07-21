# Phase P-6 — CI/CD & Ship

**ID:** P-6 · **Status:** Not started · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

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

## Deliverable line
`Phase 6 ready for review` OR `Phase 6 blocked because: <one sentence>`.

## Health check
`GET https://<vercel-deployment-url>/v1/health -> 200 with body match /"ok"/`

## Rollback command
`vercel rollback ${LAST_DEPLOYMENT_URL}` (human-triggered; nerdflow never auto-executes rollback)

## Env vars required
- `PORT` (provided by the platform / shell)
- `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` (CI/deploy only, from GitHub Secrets)
