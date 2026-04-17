# Exra Admin v1 Spec

## Goal

Deliver a production-safe admin module for Exra protocol operations with:
- secure authentication,
- role-based authorization,
- auditable privileged actions,
- minimal but sufficient operational tooling.

## Non-Goals (v1)

- Full enterprise IAM.
- Fine-grained policy engine per endpoint argument.
- Full BI/data warehouse dashboards.

## Roles and Permissions

Define three roles:

- `admin_ops`
  - view tokenomics status,
  - view/retry oracle queue,
  - trigger oracle process job,
  - view health/metrics summary.

- `admin_finance`
  - everything in `admin_ops` (read),
  - approve/reject payout requests,
  - view treasury/swap guard state.

- `admin_readonly`
  - read-only views for queue, tokenomics, payouts, and incidents.

Optional super-role: `admin_root` (break-glass only).

## Authentication Model

## 1. Identity source

- Use authenticated user identity from web auth layer.
- Server validates signed token/session and loads role from DB.

## 2. Session/token requirements

- Short-lived access token/session.
- Refresh handled by auth provider/session middleware.
- All admin endpoints require authenticated identity.

## 3. Secret hygiene

- `PROXY_SECRET` and `NODE_SECRET` remain for service-to-service/node flows.
- Never use `PROXY_SECRET` entry in admin UI.
- In non-dev environments: fail startup if critical secrets are missing.

## Authorization Rules

Add middleware:
- `RequireAdminRole(anyOf...)`.

Authorization matrix:

- `GET /api/admin/tokenomics/stats` -> ops, finance, readonly
- `GET /api/admin/oracle/queue` -> ops, finance, readonly
- `POST /api/admin/oracle/queue/{id}/retry` -> ops, finance
- `POST /api/admin/oracle/process` -> ops, finance
- `GET /api/admin/payouts` -> finance, readonly
- `POST /api/admin/payout/{id}/approve` -> finance
- `POST /api/admin/payout/{id}/reject` -> finance
- `GET /api/admin/incidents` -> ops, finance, readonly

## API Design (v1)

Namespace all privileged routes under `/api/admin/*`.

Response contract:
- include `request_id`,
- include `actor_id`,
- include `timestamp`.

Error contract:
- `401`: unauthenticated,
- `403`: authenticated but missing role,
- `409`: state conflict (already processed/retried),
- `422`: validation.

## Data Model Additions

## 1. `admin_users`

- `id` (uuid),
- `email` (unique),
- `role`,
- `is_active`,
- `created_at`,
- `updated_at`.

## 2. `admin_audit_logs`

- `id` (bigserial),
- `actor_id` (uuid),
- `actor_email`,
- `role`,
- `action` (e.g. `oracle.retry`, `payout.approve`),
- `resource_type`,
- `resource_id`,
- `request_id`,
- `ip`,
- `user_agent`,
- `payload_redacted` (jsonb),
- `result` (`success|error`),
- `error_text`,
- `created_at`.

## UI Scope (`/admin`)

Pages:
- `/admin` dashboard (status cards + alerts),
- `/admin/oracle-queue`,
- `/admin/payouts`,
- `/admin/incidents`,
- `/admin/audit` (readonly log viewer).

Minimum widgets:
- queue backlog by status,
- retries in last 1h/24h,
- swap guard state,
- pending payout count,
- recent critical errors.

## Security Requirements

- CSRF protection for state-changing actions.
- Rate limit admin mutation endpoints.
- Redact secrets in logs.
- Require explicit confirmation for destructive/financial actions.
- Idempotency key for retry/approve operations where possible.

## Migration Plan

### Phase A - Backend boundary

1. Create `/api/admin/*` endpoints (initial wrappers around existing handlers).
2. Add admin auth + role middleware.
3. Create `admin_users` and `admin_audit_logs` migrations.
4. Add action logging on all admin mutations.

### Phase B - Frontend admin module

1. Implement `/admin` pages and navigation.
2. Remove raw secret input from marketplace admin controls.
3. Gate admin UI by role, not by static token.

### Phase C - Hardening

1. Remove old privileged routes from non-admin namespaces.
2. Enforce startup fail-fast for missing critical env in stage/prod.
3. Add smoke tests for role permissions and audit-log writes.

## Acceptance Criteria

- Every privileged action is executed by authenticated admin identity.
- Every privileged action is logged to `admin_audit_logs`.
- No admin action requires manual shared-secret entry in browser.
- Role separation works (`ops` cannot approve payouts; `readonly` cannot mutate).
- Docs and runbook reflect the new admin operating model.

## Test Plan

## API tests

- `401` when unauthenticated.
- `403` when wrong role.
- `200/204` for allowed role.
- audit log row created on mutation.

## E2E tests

- admin login -> queue retry -> audit log visible,
- finance login -> payout approve -> audit log visible,
- readonly login -> mutation blocked.

## Rollout Notes

- Keep old routes temporarily behind feature flag for migration window.
- Announce operator workflow change before cutover.
- After cutover, revoke UI usage of `PROXY_SECRET` for admin tasks.
