# EXRA Documentation — v2.5.0

This directory contains the living technical documentation for EXRA.

## Source of Truth

`AGENTS.md` in the repository root is the canonical engineering ledger.
If any document conflicts with `AGENTS.md`, `AGENTS.md` wins.
For product intent, see `EXRA White Paper_ Sovereign DePIN Infrastructure.pdf`.

## Core Documents (Current)

- `ARCHITECTURE.md` — Runtime architecture: Go server, Android Resident IP tunnel, Feeder audit, TMA, Oracle flow, peaq integration. **Updated v2.5.0.**
- `RUNBOOK.md` — Operational runbook: env vars (peaq/EXRA stack), startup order, smoke checklist, anti-fraud SQL queries. **Updated v2.5.0.**
- `PROTOCOL_ECONOMY_SPEC.md` — Economic invariants and settlement rules for EXRA on peaq.
- `payout-logic.md` — Claim flow, timelock/tax rules, payout safety checks.
- `ADMIN_V1_SPEC.md` — Admin role model and operational APIs.
- `DEPLOYMENT_GUIDE.md` — Environment and rollout guidance.
- `RELEASE_CHECKLIST.md` — Pre-release and production verification checklist.
- `SMOKE_TESTS.md` — Manual smoke tests for critical user and operator paths.
- `PEAQ_RUNTIME_INTEGRATION.md` — Runtime crate integration and fund flow.
- `PALLET_EXRA_V2_4_1_SPEC.md` — Current pallet-level chain behavior and constraints.
- `TMA_MARKETPLACE_HARDENING_PLAN.md` — Security hardening plan; P0 + P1 applied (see AGENTS.md §15).
- `REALITY_AUDIT_2026-04-23.md` — Forensic gap analysis (code vs whitepaper vs branches).

## Version History

| Version | Date | Key Changes |
|---------|------|-------------|
| v2.5.0 | 2026-04-26 | Resident IP end-to-end, feeder audit active, IPv6 /48 Sybil, F3 mint rounding, HRW matcher, parallel PoP worker ×8, TMA P1 all closed |
| v2.4.1 | 2026-04-18 | Marketplace forensic audit closed (A1–G3), TMA P0 hardening |
| v2.4.0 | 2026-04-16 | peaq Pallet deploy testnet, Oracle multi-sig, Anti-fraud Feeders + Canary |

## Legacy and Historical Notes

- TON-era plans/specs removed from the active docs set.
- Historical materials remain under `docs/archive/` for reference only.
