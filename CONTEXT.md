# CONTEXT — Start here

> Front door for this repo. Before editing any file listed as **frozen**, read the
> freeze policy below. This file — not CLAUDE.md — is the canonical, tool-agnostic
> contract, and it travels with the repo.

<!-- STATIC: hand-maintainable; identical across nerdflow projects. The block below
     this, between the nerdflow-status markers, is machine-generated — do not hand-edit it. -->

## Read order (for context)
1. `PRD.md` (or `SPEC.md`) — what & why
2. `AUDIT.md` — constraints the project must live with
3. `USERS.md` — who uses it, the use cases
4. `ARCHITECTURE.md` — topology, contracts, components (+ `docs/diagrams/architecture.svg`)
5. `DESIGN_DECISIONS.md` — ADR log
6. `EVAL.md` — how we know it works
7. `SECURITY.md` — security posture, rubric findings + resolutions
8. `COMPLIANCE.md` — applicable regulatory hats, per-hat rubric, surface-to-hat map
9. `IMPLEMENTATION_PLAN.md` / `docs/ROADMAP.md` — the delivery plan
10. `docs/sessions/INDEX.md` — build history

## Artifact registry
| Artifact | Produced by | Lifecycle | Who may write |
| --- | --- | --- | --- |
| `PRD.md` | /nerdflow-idea | frozen after arch gate | arch gate (append addenda only) |
| `AUDIT.md` `USERS.md` `ARCHITECTURE.md` `DESIGN_DECISIONS.md` `EVAL.md` | /nerdflow-arch | frozen after arch Phase 5 | see freeze policy |
| `SECURITY.md` | /nerdflow-arch Phase 5c | frozen after Phase 5c write / Phase 6 handoff | append-only ADR path (same as ARCHITECTURE.md diagram amendments) — new ADR appended in DESIGN_DECISIONS.md, referenced from a §Findings amendment |
| `COMPLIANCE.md` | /nerdflow-arch Phase 3e | frozen after Phase 5 write / Phase 6 handoff | append-only ADR path (same as SECURITY.md's) — new ADR appended in DESIGN_DECISIONS.md, referenced from a §Findings amendment |
| `docs/diagrams/architecture.{d2,svg}` | /nerdflow-arch | regenerated with ARCHITECTURE.md | arch / gated amendment |
| plan index + `docs/phases/` or `docs/features/` | /nerdflow-impl | frozen after impl Phase 5; brief `Status` + `Implementation notes` mutable | impl owns content; builder updates status/notes |
| `docs/sessions/<piece>__<sha7>.md` | /nerdflow-build | append-only | the building session |
| `docs/sessions/INDEX.md` | /nerdflow-build | coordinator-maintained scan layer | build coordinator |

## Freeze policy (canonical — nerdflow skills point here; they do not restate it)
- **"Frozen" = no _silent_ writes.** It does **not** mean no writes.
- **Permitted** on frozen arch docs: user-gated, append-only, provenance-marked
  amendments — a new ADR appended to `DESIGN_DECISIONS.md` carrying
  `Status: Accepted (<date>, build-time amendment)` + `Source: build session <id>`;
  a re-rendered diagram paired with a provenance ADR. The only in-place edit to an
  accepted ADR is the one-line `Superseded by ADR-NNNN` status flip.
- **Forbidden:** silent edits, in-place rewrites of accepted ADRs, editing a frozen
  input to match the code.
- This file **declares** the contract; it does **not** enforce it. Enforcement lives
  in the nerdflow skills' hard rules + the gated-amendment path (/nerdflow-build
  Phase 5b.4).

<!-- BEGIN nerdflow-status -->
## Current status (generated 2026-07-21T11:16:16Z, baseline c43bec2)
> Derived from artifact presence + phase completion. If this disagrees with the
> files, **the files win** — re-run the relevant nerdflow skill to refresh.

- **Stage:** build in progress
- **Frozen now:** PRD.md, AUDIT.md, USERS.md, ARCHITECTURE.md, DESIGN_DECISIONS.md, EVAL.md, SECURITY.md, COMPLIANCE.md, IMPLEMENTATION_PLAN.md + docs/phases/
- **Mutable now:** piece `Status:` fields and `## Implementation notes` (during build); `docs/sessions/*` are append-only
- **Build:** 3 session log(s); 1 build-time ADR amendment(s) promoted
<!-- END nerdflow-status -->

## Role bindings

- builder: agents:golanger
- test-author: agents:golanger

Rationale: the PRD requires the golanger agent to write all Go code (it is itself under
evaluation). Both source and tests are authored by golanger subagents at /nerdflow:build;
the coordinator orchestrates, runs gates, and updates status only — it writes no Go.

## Test discipline

test_command: go test ./...
coverage_command: go test -coverprofile=cover.out ./... && gcov2lcov -infile=cover.out -outfile=coverage.lcov
coverage_report: coverage.lcov (lcov)

## CI/CD

cicd_platform: github-actions,vercel
cicd_config_files: .github/workflows/ci.yml .github/workflows/deploy.yml
cicd_triggers: push:master pr:opened pr:updated workflow-dispatch
cicd_gates: test coverage build security-scan deploy
cicd_deploy_hook: manual

## Deployment discipline

health_check: GET http://localhost:8080/v1/health -> 200 with body match /"ok"/
rollback_command: vercel rollback ${LAST_DEPLOYMENT_URL}
env_var_source: PORT provided by platform ($PORT on Vercel, shell locally); VERCEL_TOKEN/VERCEL_ORG_ID/VERCEL_PROJECT_ID in GitHub Actions Secrets

## Cleanup discipline

cleanup_cadence: manual
cleanup_mode: report
cleanup_categories: all
cleanup_category_weights: default
cleanup_p0_threshold: 0.6
cleanup_p1_threshold: 0.2
cleanup_thresholds: default
