# Session: P-5
Date: 2026-07-21
Agent: subagent (golanger — test-author + builder + operator-flagged fixes)
Piece / Brief: P-5 (docs/phases/P-5-ui.md)
Baseline SHA -> Head: cb34bd4 -> 2dc7dc5

## Accomplished
- Embedded McKinsey-clean web SPA (`internal/api/web/{index.html,app.js,style.css}`) served via `embed.FS` at `/` (`UIHandler` = `//go:embed all:web` + `fs.Sub` + `http.FileServerFS`). 9×9 grid-as-hero, system-ui (no web fonts), `#111` on `#fafafa`, one `#1a56db` accent, technique-tagged collapsible event log + metric quartet.
- Security middleware: `SecurityHeaders` (CSP `'self'`-only — no unsafe-inline/eval; HSTS; X-Frame-Options DENY; nosniff — F-10) and `CORS` (same-origin-only, non-reflecting, never `*`-with-credentials — F-9), wrapped around `/` and all `/v1/*` in `cmd/server`.
- F-11: `app.js` renders ALL response/input data via `textContent`/`createTextNode`/element-property only — zero `innerHTML`/`insertAdjacentHTML`/`document.write` of data (jasnah confirmed by source read).

## Decisions made (and why)
- Embed assets at `internal/api/web/` (not repo-root `web/`) — `//go:embed` can only reference its own subtree, and `UIHandler` is in package `api`. Within the `internal/api/**` allow-list.
- CSP `'self'`-only forces external JS/CSS files (no inline `<script>`/`<style>`) — the security posture drives the asset layout.
- CORS same-origin-only via an empty enumerated allowlist (a future dashboard origin is one explicit entry) — structurally impossible to emit `*`-with-credentials.
- Designer pre-pass SKIPPED — `frontend-design` agent unavailable; ADR-0015 is a fully-prescriptive inline recipe (no external kit); golanger built the assets verbatim; human verification (in-browser screenshots) was the taste check.

## Deviations from the frozen plan
- None from the signatures/recipe. Added a small "Clear" button (cosmetic).
- **Operator-flagged post-review fixes** (see below) — the initial build passed the automated gates + jasnah, but human-verification (screenshots) surfaced a grid-border defect and a solveTimeMs=0 display; both were fixed and re-gated before Done.

## Post-review fixes (operator-flagged; all re-gated PASS)
1. **Grid borders** (`web/style.css`, test-exempt, human-verified): `.grid` was inheriting `box-sizing: border-box` (from the global `*` rule) → its 9 fixed columns (9×cell) overflowed the content-box (9×cell−4px) by 4px, so in the SOLVED state the readonly cells' gray background painted over the right/bottom outer border (looked missing); and per-cell all-4-side borders doubled interior lines. Fixed: `.grid { box-sizing: content-box }` (outer border sits strictly outside the columns) + cells use `border-top`+`border-left` only (single/uniform interior lines; the `.grid` container owns the right/bottom outer edge). Re-screenshotted: uniform lines, full 2px outer frame on all four sides in blank AND solved states.
2. **solveTimeMs high-res timer** (Option A — user-chosen): on Windows, `time.Now()` resolution is ~0.5ms (measured), so a single ~18µs solve (measured via 1000-solve average) read `solveTimeMs: 0`. Added a build-tagged high-resolution timer — `hitime_windows.go` (`QueryPerformanceCounter` via stdlib `syscall`+`unsafe`, NO external dep; freq read once; divide-by-zero guarded) + `hitime_other.go` (`time.Now()` ns fallback) + `hitime_test.go` (sanity: elapsed over a 2ms sleep is >0 and bounded; monotonic). `solve.go` now times solve-only via `hiNow()`/`hiElapsedMs`. `solveTimeMs` now reads a real sub-ms value (~0.03–0.07ms) on Windows; `contract.go` unchanged (same float64 field). NB: this touched P-1's `solve.go` — a cross-cutting measurement fix committed with P-5.
3. **security-headers test strengthening** (jasnah RUBRIC_GAP): `security_headers_test.go` now reads `rr.Result().Header` (the flushed snapshot a real client receives) instead of the live `rr.Header()` map — so it now catches a "headers set after next" ordering regression it previously missed (jasnah empirically confirmed).

## Test evidence
- Red-state capture (Phase 5.0): api compile-red on undefined `UIHandler`/`SecurityHeaders`/`CORS`; solver/sudoku/generator green.
- Green-state capture (Phase 5.1): `go test -count=1 ./...` green (all 6 P-5 tests + hitime test + P-0..P-3); `go test -race ./internal/api/` green (gcc installed this session).
- Coverage report: coverage.lcov (lcov), 92.6% total. SecurityHeaders 100%, UIHandler 75% (unreachable fs.Sub panic path), CORS 50% (unused enumerated-allowlist branch in same-origin mode — exempt), SolveHandler 100%, hitime qpcCount/hiNow/hiElapsedMs 100%, readQPCFrequency 80% (defensive freq<=0 branch).
- Test-exempt lines applied: `web/*.js` (browser UI — human-verified). jasnah confirmed the exemption does not hide an XSS sink (read app.js).
- test-verifier verdict: PASS (initial) → PASS (re-gated after the 3 fixes). jasnah mutation/empirical checks: header non-reflection, CSP no-unsafe-inline, F-11 no-innerHTML (source read), and the post-next header-ordering regression now caught.
- Compliance evidence: none (COMPLIANCE.md `Applicable hats: N/A`).

## Deployment evidence
- **Target:** manual (CONTEXT.md `cicd_deploy_hook: manual`).
- **Deploy command run:** none — Phase 5c skipped.
- **Health check:** N/A — deploy skipped (manual/CI at P-6).
- **Rollback command (unrun):** N/A — deploy skipped.
- **Env vars propagated:** none (only `$PORT`).
- **Deviations:** none. `deployment: SKIP (cicd_deploy_hook: manual)`.

## Leanness review
- **RIGOR:** basic
- **Findings:** none — "Lean already. Ship." (UIHandler is stdlib fs.Sub+FileServerFS; headers are direct Sets; the QPC build-tag split is the approved timer fix; the route/chain is mandated wiring).
- **Net removable:** 0
- **Disposition:** advisory-only (nothing to apply).
- **Raw report:** `RIGOR: basic` / `Lean already. Ship.` / `net: -0 lines possible.`

## Open / next session
- **P-4 (batch/parallelism)** is the last piece before P-6 ship — and its `-race` gate is now fully working locally (gcc/WinLibs installed this session; `go test -race` verified green).
- Non-blocking followups (jasnah RUBRIC_GAP, for a future /nerdflow:cleanup): CORS allowlist-grant branch untested (ships when a dashboard origin is added); `TestUIHandler_NoExternalOrigins` scans only index.html (not app.js/style.css); F-11 has no automated Go guard (inherent to browser JS, review-guarded).
- KB note worth capturing: sub-ms wall-clock timing on Windows needs QPC, not `time.Now()` (recurs for any microsecond-scale benchmark).
