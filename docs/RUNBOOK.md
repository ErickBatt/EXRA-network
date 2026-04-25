# EXRA Runbook — v2.5.0 (2026-04-26)

## Environments

- `dev` — local development (Redis + Supabase local или remote dev project).
- `stage` — pre-production validation.
- `prod` — production (exra.space).

---

## Required Environment Variables

### Backend (`server/.env`)

```env
# Network
PORT=8080
SUPABASE_URL=postgres://...
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=

# Secrets (обязательно поменять в production!)
NODE_SECRET=
PROXY_SECRET=
ADMIN_SECRET=

# Gateway JWT (EdDSA Ed25519 — hex-encoded)
GATEWAY_JWT_PRIVATE_KEY=
GATEWAY_JWT_PUBLIC_KEY=

# TMA — при отсутствии сервер не стартует в production
TELEGRAM_BOT_TOKEN=
TMA_SESSION_SECRET=          # min 32 random bytes, hex
TMA_API_BASE=

# peaq / DePIN
PEAQ_RPC=ws://127.0.0.1:9944
ORACLE_NODES=3
FEEDER_STAKE_MIN=10
TIMELOCK_ANON=24h
PEAQ_ORACLE_SEED=            # sr25519 seed сервиса

# Tokenomics
EXRA_MAX_SUPPLY=1000000000
POP_EMISSION_PER_HEARTBEAT=0.00005
RATE_PER_GB=0.30
EXRA_POLICY_FINALIZED=true

# Security
WS_ALLOWED_ORIGINS=https://app.exra.space,https://dashboard.exra.space
GO_ENV=production            # включает log.Fatal на отсутствующих секретах
```

### Landing (`landing/.env.local`)

```env
SUPABASE_URL=https://<ref>.supabase.co
SUPABASE_SERVICE_ROLE_KEY=
RESEND_API_KEY=re_...        # email-нотификации waitlist на ilya.khotin@exra.space
```

### Dashboard (`dashboard/.env.local`)

```env
NEXT_PUBLIC_API_BASE_URL=https://api.exra.space
```

### Android (`android/app/build.gradle` buildConfigField)

```groovy
buildConfigField "String", "EXRA_WS_URL",  '"wss://api.exra.space/ws"'
buildConfigField "String", "EXRA_API_URL", '"https://api.exra.space"'
buildConfigField "String", "EXRA_NODE_SECRET", '"<node_secret>"'
```

---

## Startup Order

1. Apply migrations: `server/migrations/*.sql` (автоматически при старте сервера).
2. Start Redis.
3. Start backend: `cd server && go run .`
4. Start dashboard: `cd dashboard && npm run dev`
5. Launch Android node app (или установить APK с `android/releases/`).

---

## Smoke Checklist

1. `GET /health` → `{"status":"ok"}`.
2. WS node регистрируется (`register` message) → получает `{"type":"registered"}`.
3. `GET /nodes` и `GET /nodes/stats` → валидный JSON без IP-полей.
4. Heartbeat (подписанный `did + timestamp + sig`) → PoP reward в `node_earnings`.
5. Buyer flow: `POST /api/buyer/register` → `POST /api/matcher/offer` → `GET /proxy` → `POST /api/session/finalize`.
6. TMA: `/api/tma/auth` → cookie установлена; `/api/tma/me` → device stats без `telegram_id` в body.
7. Feeder audit: сервер шлёт `feeder_audit` → Android отвечает `feeder_report` с подписью → `feeder_reports` запись в БД.
8. Admin: `GET /api/admin/stats` с `X-Exra-Token` → OK.

---

## Tokenomics Verification

1. Heartbeat → `node_earnings` запись + `pop_reward_events` (idempotent при повторе).
2. `DistributeReward`: `workerReward + referralReward + treasuryReward = effectiveAmount` (8dp rounding, нет дрейфа).
3. `FinalizeSession`: E3 cross-check — если gateway > 2× worker → cap at worker (log `[E3] GATEWAY OVERBILL`).
4. `/oracle/batch` → `oracle_mint_queue` `pending → minted`.
5. `/claim/{did}` → 24h timelock для Anon, мгновенно для Peak.
6. `payout_requests`: velocity limit 1/24h на DID.

---

## Migration Notes (v2.5.0)

| Migration | Содержание |
|-----------|-----------|
| 029 | TMA hardening: `tma_revoked_sessions`, `nodes.hw_fingerprint` |
| 022 | Waitlist (`waitlist` table, применить в Supabase Dashboard) |

После применения миграций:
```sql
-- Проверить что migration_log содержит все файлы
SELECT filename FROM migration_log ORDER BY applied_at DESC LIMIT 10;
```

---

## Anti-Fraud Operational Checks

```sql
-- Frozen nodes последние 24h
SELECT device_id, freeze_reason, frozen_at FROM nodes
WHERE status = 'frozen' AND frozen_at > NOW() - INTERVAL '24 hours';

-- Sybil subnet clusters (>3 нод на /24)
SELECT SUBSTRING(ip, 1, LENGTH(ip) - POSITION('.' IN REVERSE(ip))) AS subnet,
       COUNT(*) AS cnt
FROM nodes WHERE active = true AND last_heartbeat > NOW() - INTERVAL '10 minutes'
GROUP BY subnet HAVING COUNT(*) >= 3 ORDER BY cnt DESC;

-- E3 cross-check mismatches (gateway vs worker)
SELECT id, bytes_used, worker_bytes_reported,
       bytes_used::float / NULLIF(worker_bytes_reported, 0) AS ratio
FROM sessions WHERE ended_at > NOW() - INTERVAL '1 hour'
  AND worker_bytes_reported > 0
  AND ABS(bytes_used - worker_bytes_reported)::float / worker_bytes_reported > 0.1;
```

---

## Admin v1 Operations

- Endpoints: `/api/admin/*`
- Required headers: `X-Exra-Token: <ADMIN_SECRET>` + `X-Admin-Email: <email>`
- Все действия логируются в `admin_audit_logs`.
- Seed хотя бы одного admin в `admin_users` перед включением консоли.

---

## peaq Bridge (Pre-Mainnet)

> ⚠️ Go ↔ peaq bridge (`server/peaq/peaq_client.go`) требует live-chain сверки перед mainnet. Payload'ы `batch_mint` / `update_stats` могут не совпадать с текущим pallet API.

1. Запустить локальную peaq ноду: `./target/release/peaq-node --dev`
2. Сверить extrinsic signatures: `cargo test -p pallet-exra`
3. Обновить payload'ы в `server/peaq/peaq_client.go` под актуальный runtime metadata.
