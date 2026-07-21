# Phase P-5 — Embedded UI

**ID:** P-5 · **Status:** Not started · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

## Goal
A minimal, self-contained, McKinsey-clean web SPA embedded in the binary via `embed.FS`: enter/paste
an 81-char puzzle, solve it against `/v1/solve`, and see the solution + the technique-tagged event log
— with browser-surface security controls in place.

## Entry gate
P-1 `Done` (`POST /v1/solve` exists and returns the contract).

## Dependencies
- P-1 — the `/v1/solve` endpoint and the `/v1` response contract the UI renders.

## Allow-list (source)
- `web/**` — static UI assets (HTML/CSS/JS)
- `internal/api/**` (non-test) — security-header middleware, CORS decision, static file serving via `embed.FS`
- `cmd/server/**` (non-test) — mount the embedded UI at `/`

## Allow-list (tests)
- `internal/api/*_test.go` — header/CORS/serving tests

## Read-only context
- ARCHITECTURE.md §Frontend Design Language → the embedded-SPA surface bullet (kit path, aesthetic, copy recipe — copied verbatim); §Components → `cmd/server`, `internal/api`; §Contracts → HTTP /v1
- DESIGN_DECISIONS.md §ADR-0014 (embed.FS UI), §ADR-0015 (inline design language)
- USERS.md §UC-1 (the UI is a demo surface for solve)
- SECURITY.md §F-9 (CORS posture), §F-10 (security headers/CSP), §F-11 (UI output-encoding)

## Compliance requirements
None — COMPLIANCE.md declares `Applicable hats: N/A`.

## CI/CD requirements
None — CI/CD wiring lands in P-6.

## Suggested steps
1. Build the SPA per ARCHITECTURE.md §Frontend Design Language: system-ui font (no web-font fetch), `#111` on `#fafafa`, one `#1a56db` accent, 8px scale, the 9×9 grid as the hero (bold box seams), single solve-reveal fade.
2. Wire it to `POST /v1/solve`; render the solution + collapsible event log using `textContent` (never `innerHTML` of response/input data).
3. Serve the assets from `embed.FS` at `/`; add security-header middleware (CSP with `script-src` disallowing `unsafe-inline`/`unsafe-eval`, HSTS, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`).
4. Decide and implement the CORS posture (same-origin-only, or an enumerated allowlist for the future dashboard) — never reflect arbitrary `Origin`, never `*`-with-credentials.

## Acceptance criteria
- **AC-1:** Loading `/` in a browser serves the embedded SPA (no external network fetches — no web fonts, no CDN), and entering a valid 81-char puzzle and pressing Solve renders the solved grid and the technique-tagged event log from `/v1/solve`. **Source:** USERS.md §UC-1; ARCHITECTURE.md §Frontend Design Language.
- **AC-2:** The UI renders all API response data and echoed input via `textContent` (or equivalent safe DOM APIs); no response/input data is inserted via `innerHTML`. **Source:** SECURITY.md §F-11.
- **AC-3:** Responses from `/` and `/v1/*` carry a CSP whose `script-src` disallows `unsafe-inline`/`unsafe-eval`, plus HSTS with a non-trivial `max-age`, `X-Frame-Options: DENY` (or `frame-ancestors 'none'`), and `X-Content-Type-Options: nosniff`. **Source:** SECURITY.md §F-10.
- **AC-4:** An explicit CORS policy is implemented — same-origin-only or an enumerated Origin allowlist; the server never reflects an arbitrary `Origin` and never sends `Access-Control-Allow-Origin: *` together with credentials. **Source:** SECURITY.md §F-9.
- **AC-5:** The rendered UI matches the design language (near-monochrome, one accent, grid-as-hero, system-ui) with no external assets loaded. **Source:** DESIGN_DECISIONS.md §ADR-0015.

## Automated checks
```bash
go build ./...
go vet ./...
go test -race ./...
```
Expected: header-presence test, CORS test, and embedded-serving test pass. (Visual match is a human check.)

## Test command
`(inherit from CONTEXT.md §Test discipline)`

## Coverage command
`(inherit)`

## Coverage report
`(inherit)`

## Test-exempt lines
- `web/*.js:L1-L999 — browser UI script; behavior verified by human smoke check, not Go coverage`
  (the test-verifier reviews this exemption; keep the JS thin and the solve/render logic simple)

## Manual smoke checks
```bash
PORT=8080 go run ./cmd/server
# open http://localhost:8080/ in a browser; paste a puzzle; press Solve
curl -sI localhost:8080/ | grep -iE 'content-security-policy|x-frame-options|x-content-type-options|strict-transport'
```

## Human verification
1. Open `/`, paste a `puzzles.txt` line, solve — confirm the grid fills and the event log lists techniques. Why it matters: the deployable demo must be usable.
2. Open dev-tools Network tab — confirm zero external requests (no fonts/CDN). Why it matters: self-contained + CSP-clean.
3. Eyeball the design — near-monochrome, one blue accent, grid is the hero. Why it matters: the McKinsey-clean taste bar (ADR-0015).

## Regression check
Re-run P-0 + P-1 (+ any landed) automated checks.

## Exit gate
- `/` serves the embedded SPA with zero external network fetches; a puzzle solves end-to-end in the browser.
- All response/input data rendered via safe DOM APIs (no `innerHTML` of data).
- CSP / HSTS / X-Frame-Options / X-Content-Type-Options headers present on `/` and `/v1/*`.
- An explicit non-reflecting CORS policy is in force.
- `go build`/`go vet`/`go test -race` all pass.

## Implementation notes (filled in by the builder)
> Record decisions and cross-cutting discoveries here.

## Deliverable line
`Phase 5 ready for review` OR `Phase 5 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
