# Smoke Tests

Use this checklist to verify current MVP behavior across public API, buyer auth, and billing flow.

## Prerequisites

- exra server is running (`server` directory).
- At least one active node is registered and heartbeating.
- `PROXY_SECRET` is known.
- Canonical docs are in root: `AGENTS.md` and `ARCHITECTURE.md`.

## Automated Smoke Script

Run from `server/`:

`powershell -ExecutionPolicy Bypass -File scripts/smoke.ps1 -BaseUrl http://localhost:8080 -ProxySecret <PROXY_SECRET>`

The script performs:

1. `GET /health`
2. `GET /nodes` and `GET /nodes/stats`
3. Buyer registration
4. Balance top-up
5. Session start
6. Session end
7. Session end again (idempotency check)
8. Profile and sessions fetch

## Observability Smoke
1. `GET /health`: Verify `status: ok` and DB connectivity.
2. `GET /metrics`: Verify presence of `exra_` prefixes (hub, compute, pop counters).

## Compute Marketplace Smoke
1. Register a node with `arch: amd64` and `ram_mb: 8192` to get `Tier: compute`.
2. `POST /api/compute/submit`: Submit a task from a buyer with balance.
3. Verify task state moves to `assigned` in DB.
4. Verify node receives task via WebSocket.
5. Verify node reports `compute_result`.
6. Verify task moves to `completed` and node receives `reward`.

## Payout Precheck Smoke
1. Call `POST /api/payout/precheck` with `device_id`, `amount_usd`, `recipient_wallet`.
2. Verify response includes `fees.gas_fee_sol`, `fees.ata_rent_sol`, and `net_amount_usd`.
3. Verify insufficient-fee case returns error: `Баланса недостаточно для оплаты газа`.

## Expected Outcomes

- First session finalization returns `charged: true` (if `cost_usd > 0`).
- Second finalization of the same session returns `charged: false`.
- Session remains inactive after first end.
- Buyer balance never goes below zero.
- Public endpoints `/nodes` and `/nodes/stats` return valid JSON.
