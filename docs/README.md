# Exra Documentation

This directory contains the living technical documentation for Exra.

## Source of Truth

`AGENTS.md` in the repository root is the canonical project document.
If any document conflicts with `AGENTS.md`, `AGENTS.md` wins.

## Core Documents (Current)

- `ARCHITECTURE.md` - Runtime architecture for Go server, Dashboard, TMA, Oracle flow, and peaq integration.
- `PROTOCOL_ECONOMY_SPEC.md` - Economic invariants and settlement rules for EXRA on peaq.
- `payout-logic.md` - Claim flow, timelock/tax rules, and payout safety checks.
- `ADMIN_V1_SPEC.md` - Admin role model and operational APIs.
- `RUNBOOK.md` - Operational runbook for deployment and routine maintenance.
- `DEPLOYMENT_GUIDE.md` - Environment and rollout guidance.
- `RELEASE_CHECKLIST.md` - Pre-release and production verification checklist.
- `SMOKE_TESTS.md` - Manual smoke tests for critical user and operator paths.
- `PEAQ_RUNTIME_INTEGRATION.md` - Runtime crate integration and fund flow.
- `PALLET_EXRA_V2_4_1_SPEC.md` - Current pallet-level chain behavior and constraints.
- `TMA_MARKETPLACE_HARDENING_PLAN.md` - Security hardening plan and applied controls.

## Legacy and Historical Notes

- Old TON-era plans/specs were removed from the active docs set.
- Historical materials remain under `docs/archive/` for reference only.
