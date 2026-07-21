# Compliance posture — sudoku-flow

**Applicable hats:** N/A
**PRD SHA pinned:** d65ae9cf520bf98f2e88c9802baf76d95092be5e15b33be0e2a5c841f19a7613
**Reviewed on:** 2026-07-20

## Rationale for N/A

`sudoku-flow` is a stateless Sudoku-solving HTTP service. It touches **no personal data** anywhere:
no user accounts, no authentication, no identity, no persistence of any kind, no PII, no payment
data, no health or education records, no children's data. The only input is an 81-character puzzle
string; the only output is a solved grid, performance counters, and a deduction log. No data is
stored, transmitted to third parties, or retained across requests. None of the eight shipped
compliance regimes (PII, PCI-DSS, HIPAA, GDPR, CCPA, SOC2, FERPA, COPPA) has any applicable surface.

Confirmed with the operator during `/nerdflow:arch` Phase 3e. `/nerdflow:impl` Phase 0 preflight
will see `Applicable hats: N/A` and skip its compliance branches.
