# Phase P-5 — Embedded UI

**ID:** P-5 · **Status:** Done (2026-07-21) · **Index:** [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)

> Completion: embedded McKinsey-clean SPA (embed.FS) + security headers/CORS + textContent-only rendering (F-11); jasnah PASS; human-verified in-browser (grid, event log, solve). Post-review fixes (operator-flagged): grid border geometry (uniform lines + restored outer frame), solveTimeMs high-res QPC timer (Windows), and a security-headers-test strengthening — all re-gated PASS. `go test -race` green (gcc installed).

> Designer pre-pass: SKIPPED — the `frontend-design` agent is unavailable this session, and ARCHITECTURE §Frontend Design Language / ADR-0015 is a fully-prescriptive inline recipe (no external kit to scaffold from). golanger builds the web assets verbatim from the recipe; human verification is the taste check.

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

**Built 2026-07-21 (baseline cb34bd4). All P-0..P-5 tests green; `go build`/`go vet`/`go test ./...` pass.**

- **Embed location — `internal/api/web/`, not repo-root `web/`.** `//go:embed` can only reference
  the embedding source file's own directory subtree (no `..`, no siblings). `api.UIHandler()` lives
  in package `api`, so the embed directive and the asset tree must be co-located under
  `internal/api/`. This is covered by the `internal/api/** (non-test)` allow-list (assets are
  non-test files) and is the idiomatic Go layout (serving code next to what it serves). New files:
  `internal/api/ui.go`, `internal/api/security.go`, `internal/api/web/{index.html,app.js,style.css}`.
- **Serving — `http.FileServerFS(fs.Sub(webAssets, "web"))`** (Go 1.22+, stdlib-forward). fs.Sub
  strips the `web` prefix so `/` -> `index.html`; content-type comes from extension detection
  (`.html`->text/html, `.js`->text/javascript, `.css`->text/css — all confirmed on this Windows host).
- **Middleware order — `SecurityHeaders(CORS(Recover(MaxBytes(routes))))`, then `logRequests` outermost.**
  SecurityHeaders sets the four F-10 headers before delegating, so they persist even through a
  recovered 500. Verified present on both `/` and `/v1/*` via curl.
- **CORS posture — same-origin-only (F-9).** `corsAllowedOrigins` is an empty enumerated allowlist;
  an arbitrary `Origin` is never reflected and a wildcard is never emitted, so the forbidden
  `*`-with-credentials pairing is structurally impossible. A future dashboard origin gets added as an
  explicit map entry. OPTIONS preflight returns 204 with no grant.
- **CSP is `'self'`-only** (`script-src`/`default-src` carry no `unsafe-inline`/`unsafe-eval`), which
  forces the JS and CSS to be external files — done. Grid `/` reveal `/v1/solve` are all relative
  refs; the no-external-origins test and a `grep -rniE 'https?://' web/` both return zero hits.
- **F-11 — app.js renders every response field and the echoed input via `textContent` /
  `createTextNode` / element-property assignment ONLY.** No `innerHTML`, `insertAdjacentHTML`, or
  `document.write` anywhere (the only literal "innerHTML" occurrences are prose in comments).
  `paintMetrics`/`paintLog` use `replaceChildren()` + built nodes; technique tags and witness cells
  are set via `textContent`. Render logic kept thin for human eyeball of the no-innerHTML boundary.
- **Design** copies ADR-0015 verbatim: system-ui (no web-font fetch), `#111`/`#fafafa`, single
  `#1a56db` accent (Solve button + solver-placed digits), 8px scale, 9x9 grid as hero with 2px 3x3
  box seams and 1px inner borders, `min()`-sized square cells, one fade on solution reveal, light-only.

## Deliverable line
`Phase 5 ready for review` OR `Phase 5 blocked because: <one sentence>`.

## Health check
`GET http://localhost:8080/v1/health -> 200 with body match /"ok"/`

## Rollback command
`(inherit from CONTEXT.md §Deployment discipline)`

## Env vars required
- `PORT`
