# exra Runbook

## Environments

- `dev` - local development.
- `stage` - pre-production validation.
- `prod` - production.

## Required variables

Backend (`server/.env`):

- `SUPABASE_URL`
- `PORT`
- `PROXY_SECRET`
- `NODE_SECRET`
- `ADMIN_SECRET`
- `RATE_PER_GB`
- `SOLANA_RPC_URL`
- `EXRA_MINT`
- `SOLANA_USD_PRICE`

Dashboard (`dashboard/.env.local`):

- `NEXT_PUBLIC_API_BASE_URL`

Android:

- `exra_WS_URL` in `android/app/build.gradle` `buildConfigField`.
- `exra_API_URL` in `android/app/build.gradle` `buildConfigField`.

## Startup order

1. Apply migrations from `server/migrations`.
2. Start backend.
3. Start dashboard.
4. Launch Android node app.

## Smoke checklist

1. `GET /health` returns `ok`.
2. WS node can register and stays online.
3. `/nodes` and `/nodes/stats` return valid payload.
4. Buyer flow works: register -> topup -> session -> end.
5. Payout flow works: earnings visible -> payout request -> approval.

## Tokenomics verification (current phase)

1. `traffic` event increases `nodes.traffic_bytes`.
2. Matching row appears in `node_earnings` with expected `earned_usd`.
3. Payout has no minimum threshold gate.
4. Payout request rejects amounts above earned balance.
5. Approved payout changes status to `approved`.
6. `reward_events` stores reason code and policy snapshot for each reward.
7. `/api/tokenomics/oracle/process` moves queue items from `pending` to `minted` (simulated mode).
8. `/api/tokenomics/payments/settle` creates `burn_events` and updates tokenomics stats.
9. `/api/payout/precheck` returns gas/rent/net breakdown.
10. If `TransferAmount <= 0`, payout is blocked with clear alert about gas insufficiency.

## Admin v1 operations

- Admin endpoints are namespaced under `/api/admin/*`.
- Required headers for admin API calls:
  - `X-Exra-Token: <ADMIN_SECRET>`
  - `X-Admin-Email: <admin_user_email>`
- Seed at least one active admin user in `admin_users` before enabling admin console.
