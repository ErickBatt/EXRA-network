# Exra Product Audit (2026-04)

## Scope

This audit covers:
- backend auth/admin surfaces (`server`),
- web surfaces (`dashboard` + static HTML properties),
- logical product cohesion (landing -> auth -> marketplace -> operations).

Goal: identify what exists, what is missing, and what should be prioritized.

## Executive Summary

- Core protocol/backend progression is strong (offers, oracle queue, tokenomics guards, swap guard).
- Product operations are not production-safe yet due to auth/admin gaps.
- Frontend messaging/navigation is inconsistent across surfaces (TON/Solana and USDT/USDC mixed).
- A dedicated admin module with role-based access is required before full public scale.

## What Exists Today

### Backend and operations

- Shared-secret middleware is active (`X-Exra-Token` / bearer / API-key header parsing).
- Buyer API-key auth exists for buyer routes.
- Oracle operations exist:
  - queue list,
  - queue retry,
  - queue processor trigger,
  - tokenomics stats,
  - swap and payout workflows.
- Marketplace pipeline exists:
  - offer creation,
  - deterministic assignment,
  - locked session pricing,
  - end-session settlement logic.

### Frontend

- Next.js has main routes:
  - `/` landing,
  - `/auth`,
  - `/marketplace`,
  - `/tma`.
- Marketplace page already includes operational queue monitor controls.
- Additional standalone HTML files exist (`exra-landing.html`, `exra-token.html`, etc.) and are not fully synchronized with the Next app.

## Critical Gaps and Risks

## 1) Admin/Auth Risk (Critical)

- No separate admin identity model (no RBAC, no per-operator identity, weak auditability).
- Admin-level operations are guarded by shared secret, including paths that should be role-scoped.
- Operational secret entry in browser UX increases leakage risk.
- Some sync-style endpoints need tighter auth guarantees (identity-bound control, not just input payload trust).

Impact:
- high operational and security risk,
- weak trust posture for production operations,
- difficult incident attribution.

## 2) Product Cohesion Risk (High)

- Surfaces are not fully linked as one product journey.
- Some links are placeholders/dead.
- `/tma` discoverability is weak from main user paths.

Impact:
- conversion friction,
- user confusion about where to go next.

## 3) Messaging Consistency Risk (High)

- TON vs Solana wording remains mixed across properties.
- USDT/USDC/TON rails are inconsistently presented depending on page.

Impact:
- trust drop in user-facing narrative,
- support burden and onboarding confusion.

## 4) Docs/Operations Drift (Medium)

- Runbook/release docs still contain old naming in parts and do not yet represent a single admin operating model.

Impact:
- execution mistakes during go-live,
- slower onboarding for contributors/operators.

## Should Exra Have an Admin Panel?

Yes. For this protocol, admin capabilities are required for:
- oracle queue operations and incident response,
- payout oversight and exception handling,
- swap guard monitoring and emergency controls,
- observability and audit trail.

But it must be:
- authenticated with per-user identity,
- role-based,
- fully logged.

Shared-secret-only admin is acceptable for local MVP, not for production.

## Priority Plan (Recommended)

### P0 (must do before production)

1. Introduce dedicated admin auth boundary (server-side session/JWT verification).
2. Add RBAC roles: `admin_ops`, `admin_finance`, `admin_readonly`.
3. Move sensitive admin actions into protected admin routes only.
4. Remove production fallback defaults for secrets; fail-fast on missing critical env.
5. Lock identity-sensitive sync flows to authenticated identity.

### P1 (immediately after P0)

1. Build `/admin` module in Next app:
   - oracle queue monitor + retry,
   - swap guard status,
   - payout approvals,
   - basic incident panel.
2. Add action audit log table and viewer.
3. Add alert hooks (queue stuck, repeated retries, guard active too long).

### P2 (product polish and conversion)

1. Normalize copy and token/currency narrative across all properties.
2. Decide canonical surfaces (Next app as source of truth).
3. Keep static HTML as archive/design-only or remove from deploy.
4. Build navigation spine: landing -> auth -> marketplace -> tma/admin.

## Go/No-Go Criteria (Audit View)

Go-live should require all:
- admin actions are role-bound (not shared-secret in UI),
- audit logs exist for all privileged operations,
- no legacy chain messaging conflicts in public pages,
- release docs reflect current TON/tokenomics operations.

## Related Spec

Implementation plan and endpoint matrix: `docs/ADMIN_V1_SPEC.md`.
