# Security posture — sudoku-flow

**Application class:** hybrid: backend-api + web-app
**Rubric version:** reference/security-hats/backend-api.md @ 0ecd4b15ace7f56985e86daa6af040da491c22d4; reference/security-hats/web-app.md @ 0ecd4b15ace7f56985e86daa6af040da491c22d4
**Reviewed on:** 2026-07-20
**Reviewer verdict:** PASS-with-deferred (raw Kaladin VERDICT: PASS, no BLOCKING; 8 findings accepted as tradeoffs, 7 routed to impl as acceptance criteria)

Kaladin classified all 59 rubric items (29 backend-api + 30 web-app, deduped conceptually): 8 covered
with citations, 35 not-applicable-by-charter (no auth / no persistence / no outbound calls / no
sensitive flow — each documented, none skipped silently), 8 accepted tradeoffs, and 8 rubric items
routed to impl across 6 deferred acceptance criteria, plus one out-of-rubric CI/CD supply-chain
finding. No item was gap-blocking. The one real control the design needs — allowlist validation of
the single untrusted input (the 81-char puzzle string) at the `sudoku.Parse` boundary before any
solver code runs — is firmly specified and backstopped by `http.MaxBytesReader`, a batch-length cap,
a typed `{error, code}` envelope, panic-recovery middleware, and externalized secrets.

## Findings

### F-1 No caller authentication on any ingress
- **Rubric item:** API-01 — Authentication on ingress
- **Severity:** low
- **Applies to:** USERS.md §What the System Will NOT Do; ARCHITECTURE.md §Summary (trust boundary)
- **Finding:** every endpoint is unauthenticated and public.
- **Resolution:** accepted-tradeoff (ARCHITECTURE.md §Known Tradeoffs). Deliberate: no data, no identity, no persistent side effects; a public solver by charter. Anonymous-abuse risk is bounded by F-2/F-3's caps.

### F-2 No per-caller rate limit
- **Rubric item:** API-10 — Rate limiting / abuse control
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Contracts (Batch)
- **Finding:** no per-caller request throttle.
- **Resolution:** accepted-tradeoff. No identity to key a limit on; per-request work is bounded by `MaxBytesReader` + the batch-length cap + sub-millisecond solves. $0 demo with nothing to exfiltrate. (Load-bearing: it is the presence of the batch cap that keeps this a tradeoff, not a blocker.)

### F-3 No global concurrency / backpressure ceiling
- **Rubric item:** API-11 — Resource-exhaustion / concurrency control
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Parallelism
- **Finding:** no first-party global in-flight-request ceiling.
- **Resolution:** accepted-tradeoff. Stateless, bounded-work requests; concurrency is platform-managed on Vercel and a single operator-owned process on localhost.

### F-4 No secret-rotation procedure for VERCEL_TOKEN
- **Rubric item:** API-22 — Secret lifecycle / rotation
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §CI/CD topology
- **Finding:** no documented rotation cadence for the deploy token.
- **Resolution:** accepted-tradeoff. Token is project-scoped and instantly revocable/regenerable from the Vercel dashboard; guards no runtime data; demo with no SLA.

### F-5 TLS min-version / plaintext-rejection not first-party-pinned (API)
- **Rubric item:** API-23 — Transport security
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Summary (trust boundary)
- **Finding:** TLS posture delegated to the platform, not pinned in first-party config.
- **Resolution:** accepted-tradeoff. Vercel terminates HTTPS by default; no credentials or PII in transit; min-version left to the platform.

### F-6 TLS posture not first-party-stated (browser surface)
- **Rubric item:** WEB-04 — Transport security (browser)
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Summary (trust boundary)
- **Finding:** same as F-5 for the SPA surface.
- **Resolution:** accepted-tradeoff. Vercel HTTPS; no session cookies or user data to protect on the wire.

### F-7 No explicit ranked threat-model section
- **Rubric item:** WEB-11 — Documented threat model
- **Severity:** low
- **Applies to:** AUDIT.md §Security S1
- **Finding:** no standalone ranked threat-model document.
- **Resolution:** accepted-tradeoff. AUDIT S1 enumerates the single untrusted input + single secret, and USERS §NOT-Do enumerates the refused surfaces; zero data assets make the implicit model adequate.

### F-8 No named alertable security signals
- **Rubric item:** WEB-24 — Security monitoring / alerting
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Observability
- **Finding:** no alertable security events defined.
- **Resolution:** accepted-tradeoff. No data assets; `slog` to stdout on Vercel/localhost; $0 demo without monitoring infra.

### F-9 CORS posture not specified for the browser SPA → API calls
- **Rubric item:** WEB-03 — CORS policy
- **Severity:** medium
- **Applies to:** ARCHITECTURE.md §Contracts (HTTP /v1)
- **Finding:** no stated CORS decision; the future React dashboard will call cross-origin.
- **Resolution:** deferred-to-impl (piece TBD-cors-posture)
- **Acceptance signal:** an explicit CORS decision is implemented — same-origin-only, or an enumerated Origin allowlist for the future dashboard; the server never reflects arbitrary `Origin` and never sends `*` with credentials.

### F-10 Security response headers (CSP/HSTS/X-Frame-Options/X-Content-Type-Options) not emitted
- **Rubric item:** WEB-05, WEB-12, WEB-13 — Security headers
- **Severity:** medium
- **Applies to:** ARCHITECTURE.md §Frontend Design Language; DESIGN_DECISIONS.md ADR-0015
- **Finding:** ADR-0015 claims a "CSP-friendly UI" but no CSP header is actually declared — a property, not an emitted control.
- **Resolution:** deferred-to-impl (piece TBD-security-response-headers)
- **Acceptance signal:** the embedded-UI/edge emits a CSP whose `script-src` disallows `unsafe-inline`/`unsafe-eval`, HSTS with a non-trivial `max-age`, `X-Frame-Options: DENY` (or `frame-ancestors 'none'`), and `X-Content-Type-Options: nosniff`.

### F-11 UI output-encoding of API/echoed data not specified
- **Rubric item:** WEB-09 — Output encoding / DOM XSS
- **Severity:** medium
- **Applies to:** ARCHITECTURE.md §Frontend Design Language
- **Finding:** the SPA renders API responses and the echoed input string; unsafe DOM insertion would be a reflected-XSS vector.
- **Resolution:** deferred-to-impl (piece TBD-ui-output-encoding)
- **Acceptance signal:** the UI renders all response data and echoed input via safe DOM APIs (`textContent`), never `innerHTML` of response/input data.

### F-12 POST content-type not enforced
- **Rubric item:** API-14 — Content-Type enforcement
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Contracts (HTTP /v1)
- **Finding:** handlers do not restrict the request Content-Type.
- **Resolution:** deferred-to-impl (piece TBD-content-type-enforcement)
- **Acceptance signal:** POST handlers accept only `application/json` and reject other content types with `415`.

### F-13 Dependency/supply-chain scanning deferred
- **Rubric item:** WEB-15, WEB-22 — Dependency vulnerability management
- **Severity:** medium
- **Applies to:** DESIGN_DECISIONS.md ADR-0017; ARCHITECTURE.md §CI/CD topology (Deferred slots)
- **Finding:** `govulncheck` is a named deferred CI slot, not yet a gate.
- **Resolution:** deferred-to-impl (piece TBD-govulncheck-gate)
- **Acceptance signal:** a `govulncheck ./...` CI gate is added, `go.sum` is committed, and a new-dependency vetting rule is stated.

### F-14 /v1/generate difficulty enum not validated
- **Rubric item:** API-13 — Input schema validation (boundary completeness)
- **Severity:** low
- **Applies to:** ARCHITECTURE.md §Contracts (Generate)
- **Finding:** the puzzle-string boundary is strong, but the `difficulty` field's enum validation is not described.
- **Resolution:** deferred-to-impl (piece TBD-generate-input-validation)
- **Acceptance signal:** `/v1/generate` rejects unknown `difficulty` values with a typed `invalid_input`.

### F-15 CI/CD pipeline attack surface (out-of-rubric — Kaladin RUBRIC_GAP-1)
- **Rubric item:** none (rubric blind spot; the single most material residual surface)
- **Severity:** medium
- **Applies to:** ARCHITECTURE.md §CI/CD topology
- **Finding:** a leaked/abused `VERCEL_TOKEN` deploys attacker code to the demo domain; the rubrics have no item for GitHub Actions workflow-injection, `pull_request_target` exposure, or third-party action pinning.
- **Resolution:** deferred-to-impl (piece TBD-ci-supply-chain)
- **Acceptance signal:** all GitHub Actions are pinned to full commit SHAs; no workflow uses `pull_request_target` (or any untrusted-input trigger) with access to secrets; the token stays project-scoped.

## Noted rubric gaps (minor, materially nil here — recorded so they are not silent assumptions)
- **RUBRIC_GAP-2:** no item covers `embed.FS`/`http.FileServerFS` path-traversal — low risk (stdlib FileServerFS is path-clean).
- **RUBRIC_GAP-3:** resource items cover body/rate but not algorithmic-complexity DoS from a crafted puzzle maximizing `candidateChecks` — bounded to nil by the tiny 9×9 domain + capped technique tier.
