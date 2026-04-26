# EXRA — Единый документ проекта

> **Читай этот файл в начале каждой сессии.**
> С 23 апреля 2026 whitepaper (`EXRA White Paper_ Sovereign DePIN Infrastructure.pdf`) — главный продуктовый документ проекта.
> `AGENTS.md` — инженерный ledger: что реально реализовано в коде, что живёт только в ветках, и что ещё не production-ready.
> При конфликте по статусу реализации выигрывают код, тесты и [docs/REALITY_AUDIT_2026-04-23.md](docs/REALITY_AUDIT_2026-04-23.md).
> Последнее обновление: **26 апреля 2026 — v2.5.1** (TMA earnings SQL fix, Android hw_hash отправка, CSRF Origin-gate, корректировка §15)

---

## 1. Что мы строим

**Exra** — децентрализованная сеть где любое устройство (телефон, ПК, планшет) шарит интернет-трафик и вычислительные мощности, и получает крипто-вознаграждение.

Покупатели (арбитражники, парсеры, AI-стартапы) платят за доступ к пулу IP через маркетплейс. Цена формируется рынком.

**УТП:**
- **Масштаб:** Миллионы анонимных нод как "пушечное мясо" + эволюция в Peak.
- **Работает на любом устройстве** — сканирует железо и выбирает режим. Слабое → прокси трафик. Мощное → GPU задачи.
- **Безопасность:** Anti-fraud >95% (P2P feeders + multi-oracle + slashing). Sybil-защита.
- **UX:** Быстрый старт <10s, без seed/KYC на входе. Окупаемость 6–9 мес.
- Минимальный вывод $1, быстро, в PEAQ/USDT. Фокус: Индия, ЮВА, Латинская Америка, Африка.

**Конкуренты:** Honeygain, Grass Network, PacketStream, Vast.ai

---

## 2. Стек технологий

| Компонент | Технология |
|-----------|-----------|
| Сервер (Oracle) | Go + gorilla/mux + gorilla/websocket |
| База данных | Supabase (PostgreSQL) |
| Кэш / PubSub | Redis |
| Блокчейн | **peaq Pallet (Rust)** *(Legacy: TON Jetton TEP-74)* |
| Системы ID | peaq DID (ECDSA, Fingerprint: Android ID + hw hash) |
| Android-нода | Kotlin, Foreground Service + peaq SDK |
| Desktop-нода | (в разработке) |
| Dashboard | Next.js |
| Telegram Mini App | (в разработке) |
| Метрики | Prometheus |
| Контейнеры | Docker + docker-compose |

---

## 3. Честный статус компонентов

> Статусы отражают реальность, а не планы.

| Компонент | Статус | Что реально работает |
|-----------|--------|---------------------|
| Go сервер (core + gateway) | 🟢 ~95% impl / ~82% prod | `go test -count=1 ./...` green (все пакеты). E3 buyer-side cross-check, E4 mandatory PoP sig, G2 IPv6 /48 subnet, F3 mint rounding (roundExra 8dp), HRW matcher tiebreaker, parallel PoP worker (×8) — всё закрыто. Открыто: Go ↔ peaq bridge payload mismatch |
| Android APK | 🟢 ~92% impl / ~85% prod | Resident IP tunnel end-to-end (правильные заголовки, sr25519 "substrate" context, двунаправленный pipe без data loss). Feeder audit активен (performFeederCheck → signed feeder_report). Live stats (tunnels, bytes, GearScore, credits) в UI. Emulator guard, BootReceiver, battery optimization. |
| Dashboard / Marketplace | 🟡 ~75% impl / ~62% prod | Marketplace, buyer auth, TMA pages, buyer-api proxy, HRW node selection есть. Buyer-side hardening в `main`. |
| TON смарт-контракт | 🗑️ Removed | Полностью заменено на PEAQ DePIN |
| TON интеграция в Go | 🗑️ Removed | Полностью заменено на PEAQ DePIN |
| peaq Pallet (Rust) | 🟡 ~85% impl / ~70% prod | `pallet_exra`, runtime wiring и тесты есть; реальная chain compatibility вне репо не доказана |
| Go ↔ peaq bridge | 🟡 ~45% impl / ~25% prod | RPC client и mock E2E есть, но payload'ы в `server/peaq/peaq_client.go` не совпадают с текущим pallet API |
| Migration Runner / Admin / Payouts | 🟢 ~85% impl / ~72% prod | Авто-накат миграций, admin payout flow, `mark-paid`, tx_hash audit trail, role auth. Migration 029 (TMA hardening) применена. |
| Telegram Mini App backend | 🟢 ~90% impl / ~78% prod | Cookie-session + TMAAuth. TMA P1 (#3–#8) закрыты: JWT revocation (jti + blacklist), JOIN ownership check, device-level rate limit (≥3/ч), fingerprint binding, telegram_id убран из `/auth`, DID uniqueness в TmaStake. |
| Desktop агент | 🟡 ~30% impl / ~15% prod | Skeleton клиента, heartbeat, tunnel/compute simulation; далеко до production |
| Dynamic Pricing | 🟢 ~85% impl / ~72% prod | HRW fnv32a tiebreaker в matcher, Avg Price, фильтры по стране/тиру — всё в `main` |
| Compute Tasks | 🟡 ~75% impl / ~55% prod | Dispatch/result path, ZK-light attestation и timeout monitor есть; продовая изоляция и экономика не доказаны |

> Детальная сверка whitepaper ↔ main ↔ branch-only коммиты: [docs/REALITY_AUDIT_2026-04-23.md](docs/REALITY_AUDIT_2026-04-23.md)

---

## 4. Фазы разработки

### ФАЗА MVP (текущая) — PEAQ DePIN Migration + Прокси + PoP
**Цель:** Масштаб, защита от сибилов, первые честные пользователи, реальные деньги, реальные токены на peaq.

**Что нужно до запуска v1:**
- [x] **Chain:** peaq Pallet deploy в testnet. (Done 2026-04-16)
- [x] **Oracles:** Реализовать 3 Oracle-ноды на Go + Redis PubSub (multisig extrinsic, daily batch). (Done 2026-04-16)
- [x] **Anti-Fraud:** Реализовать логику P2P Feeders и Canary-проверок (10k sim nodes тесты). (Done 2026-04-16)
- [x] **DID & Security:** Android peaq SDK (DID подписи), UI с timelock bar (24h для анонимов). (Done 2026-04-17)
- [x] **Наследие:** Header Auth и TOCTOU защита выплат (SELECT FOR UPDATE) работают.

### ФАЗА 2 — Маркетплейс + TMA (Implemented, not fully hardened)
- [x] Dynamic pricing с фильтрами по стране/цене/тиру.
- [x] Telegram Mini App с backend (Staking/Me/Stats).
- [x] Ролевая авторизация Admin (Ops/Finance/Admin).
- [ ] Desktop агент (Windows).

### ФАЗА 3 — Compute Market (Implemented, not production-proven)
- [x] GPU задачи с ZK-light верификацией (DID signatures).
- [x] Авто-мониторинг тайм-аутов и RS Penalty (-50 points).
- [x] Оффер-маркетплейс с детерминированным выбором нод (Peak only).
- [ ] iOS агент.

---

## 5. Токеномика v2.0: PEAQ DePIN (Target Spec from Whitepaper)

*Это целевое состояние из whitepaper. Для фактической инженерной готовности см. секции 3, 11, 15 и `docs/REALITY_AUDIT_2026-04-23.md`.*

### 5.1 Token Model (Immutable Pallet)
```
SYMBOL: EXRA        MAX_SUPPLY: 1,000,000,000 EXRA (9 decimals)
PREMINE: 0          EPOCH: 100M supply-based halving (×0.5)
TAIL: 0             POLICY: finalized=true в production (необратимо)
```

- **RS Mult (Reputation Score):** GearScore → on-chain Reputation Score (0–1000). Reward = base × epoch × RS_mult (0.5–2.0).
- **Treasury:** 20% rewards + 25% anon tax → peaq DeFi swaps (circuit breaker). От 25% налога с анонимов runway обеспечивается на 2+ года.

### 5.2 Identity Tiers (peaq DID)
| Tier | Req. | RS Base | Tax | Timelock | Priority | Stake |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| **Anon** | Machine ID | 200–500 (×0.5) | 25% | 24h | Low | 0 |
| **Peak** | KYC/VC + Stake | 700–1000 (×1.5–2.0) | 0% | 0h | High | 100 EXRA (slashable) |

- **Onboarding:** В приложении Exra → автоматический peaq DID (Ecdsa keypair). Fingerprint: Android ID + аппаратный хэш (запрет эмуляторов).
- **Upgrade:** Инстантный переход в Tier Peak при стейкинге 100 EXRA + VC (KYC biometric).

### 5.3 Work Cycle (Off-Chain Logs)
1. Подключение WS (NODE_SECRET + DID sig).
2. Heartbeat каждые 5 мин: `PoP credits = 0.00005 × RS_mult`.
3. Трафик/Задачи: `credits = (GB × $0.30) × RS_mult`. Рассчитывается только `verified_bytes`.
4. В UI: Real-time "Pending Credits" (хранение в Supabase/Redis).

**GearScore (GS) Formula** (Считается сервером, деградация -10%/неделю при неактивности):
```
GS = 0.4*Hardware(Power) + 0.3*Uptime(99%→990) + 0.2*Quality(verified_bytes/reported) + 0.1*Feeder_Trust
RS_mult = min(GS/500, 2.0)  // Anon max 0.5x
```

### 5.4 Anti-Fraud: Feeders + Canary (P2P + Server)
- **Canary:** 5% задач = фейки (серверные health-check пробы). Провал → `GS=0`, сгорают кредиты за день.
- **Feeders (P2P Audit):**
  - **Assign:** Случайный выбор сервером ноды с GS>300, вне текущей подсети, stake>10 EXRA.
  - **Work:** 10% пропускной способности/нода/день тратится на прокси-проверку других нод.
  - **Reward:** +20% к PoP за честные репорты.
  - **Slashing:** Ложно-положительные/негативные проверки: штраф -5% стейка.
  - **Collusion Block:** Мульти-фидеры (минимум 3 на проверку), zk-proof задержки.

### 5.5 Daily Oracle Batch (Multi-Sig Secure)
**Поток (каждый день в 00:00 UTC):**
1. 3 Oracle-ноды (Go, геолокационно распределены) собирают данные WS и логи.
2. Cross-audit: Большинство (2/3) должно подтвердить отсутствие фрода (flag votes).
3. При фроде: Сжигание фродерских кредитов (аттестация on-chain).
4. Калькуляция: `Credits → EXRA` (с учетом `RS_mult`).
5. **peaq Pallet extrinsic:** `batch_mint(DID→EXRA, proofs)`.
   - *Gas:* ~\$0.01 за 10k нод (1 транзакция). Вычитается при выводе клиентом.
   - *Dispute:* Если консенсуса <2/3 → фриз и ручной аудит (пишется в лог администратора).
   - *Proofs:* Кредиты подписаны DID + VC для Peak.

### 5.6 Payouts (Tiered Gates)
`MIN_PAYOUT: $1 USDT/PEAQ (gas<5%)` | `VELOCITY: 1/24h на DID`

| Event | Anon | Peak |
| :-- | :-- | :-- |
| **Post-Batch** | 24h timelock + 25% tax | Мгновенно (Instant) |
| **UI** | "Audit OK → Holding 24h" + прогресс-бар | "Ready: Claim" |
| **Flow** | DID wallet auto-init + перевод | Same |

*Decay Boost (ускорение):* При честной работе 7 дней → timelock уменьшается на -4ч/день (до 0h максимума).

---

## 6. Структура проекта

```
/exra
  AGENTS.md              ← этот файл (единый источник правды)
  docker-compose.yml

  /server                ← Go, ядро системы (Оракулы)
    main.go              ← все роуты, инициализация
    /handlers            ← HTTP хендлеры
    /hub                 ← менеджер WebSocket соединений, Redis pub/sub
    /models              ← вся бизнес-логика и SQL
    /middleware          ← auth, rate limit, CORS, security headers
    /migrations          ← SQL миграции
    /peaq                ← (NEW) интеграция с peaq network (заменит /ton)
    /ton                 ← (Legacy) код TON — заморожено
    /config              ← загрузка .env
    /db                  ← подключение к PostgreSQL

  /landing               ← Next.js 15 лендинг (exra.space)
    app/
      page.tsx           ← компоновка секций
      globals.css        ← design tokens + .form-input
      actions/
        waitlist.ts      ← Server Action: Supabase insert + Resend email
    components/
      waitlist-section.tsx ← форма Early Access (3 роли + Konami easter egg)
      ...                ← остальные секции лендинга

  /android               ← Kotlin нода + peaq SDK (DID/WS)
  /dashboard             ← Next.js покупательский UI
  /desktop               ← (в разработке)
  /docs                  ← вспомогательные спеки (устаревшие)
```

---

## 7. API роуты

### Публичные
```
GET  /health
GET  /nodes               → PublicNode[] (без IP, без device_id)
GET  /nodes/stats
GET  /ws                  → WebSocket для нод (требует NODE_SECRET)
GET  /ws/map              → live map stream
```

### Нода (NODE_SECRET + DID sig)
```
POST /api/node/register
POST /api/node/{id}/heartbeat
GET  /api/node/tunnel
POST /api/node/set-referrer
GET  /api/nodes
GET  /api/nodes/market-price
```

### Покупатель (API ключ покупателя)
*(Без изменений: /api/buyer/*, /proxy и другие endpoints)*

### Oracle / Service
```
POST /oracle/batch        → Daily sync batch flow
POST /claim/{did}         → Выплаты для DID с учетом tax и timelock
POST /api/compute/node/result → (NEW) Сдача результата с DID подписью
POST /api/tma/stake       → (NEW) Стейкинг 100 EXRA для перехода в Peak
POST /api/payout/precheck → (Legacy)
```

### Admin v1 (ролевой JWT)
*(Метрики, очередь payout, инциденты - требует реализации авторизации)*

---

## 8. Переменные окружения

```env
# ── Go-сервер (server/.env) ──────────────────────────────────────────────
PORT=8080
SUPABASE_URL=postgres://...
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=

# Секреты (менять в production!)
NODE_SECRET=
PROXY_SECRET=
ADMIN_SECRET=

# PEAQ / DePIN
PEAQ_RPC=ws://127.0.0.1:9944
ORACLE_NODES=3
FEEDER_STAKE_MIN=10
TIMELOCK_ANON=24h
PEAQ_ORACLE_SEED=...

# Токеномика
EXRA_MAX_SUPPLY=1000000000
POP_EMISSION_PER_HEARTBEAT=0.00005
RATE_PER_GB=0.30
EXRA_POLICY_FINALIZED=true

# ── Лендинг (landing/.env.local) ────────────────────────────────────────
SUPABASE_URL=https://<ref>.supabase.co        # REST API URL, не postgres://
SUPABASE_SERVICE_ROLE_KEY=                    # service_role key (bypass RLS)
RESEND_API_KEY=re_...                         # опционально; email на ilya.khotin@exra.space
```

> **⚠️ Сервер выдаёт WARN при старте если секреты равны дефолтным значениям.**

---

## 9. Критические бизнес-правила (нельзя нарушать)

1. **No Mint w/o Proof:** Выплаты (Credits → EXRA) только через подписанную аттестацию (DID + oracle sigs). Заменило правило "Нет выплаты без verified usage".
2. **Sybil Caps:** Допускается максимум 5 DID на 1 IP в 24 часа. Лимит feed-проверок внутри одной подсети — 10%.
3. **Slashing:** Потеря стейка >20% за повторный фрод → полный отзыв (revoke) DID.
4. **Treasury Lock:** Требуется минимум 10% резерва treasury. Срабатывает DEX circuit breaker (пауза свапов), если волатильность >10%.
5. **No Mint Over Limit:** `minted_total + amount <= max_supply` — жёсткий cap.
6. **Policy finalized — необратимо:** После установки `EXRA_POLICY_FINALIZED=true` экономику нельзя менять.
7. **Payout TOCTOU:** Баланс проверяется только внутри транзакции с `SELECT FOR UPDATE`.
8. **Privacy:** IP нод не раскрывается публично. Публичный API возвращает только `PublicNode` без IP.
9. **Audit All:** Каждое действие админа пишется в `admin_audit_logs` и peaq events.

---

## 10. Риски и их покрытие (Target Model / не считать proof-of-production)

| Threat | Mitigation | Kill Rate |
| :-- | :-- | :-- |
| **Sybil/Farms** | Fingerprint + subnet caps + feeder stake | 95% |
| **Oracle Fail** | Multi-sig + geo-dist | 99% |
| **Collusion** | Random 3-feeders + zk-latency | 92% |
| **Churn** | 24h timelock + decay boost (честное поведение режет лок) | <15% |

---

## 11. Известные долги (нужно сделать)

### ✅ Закрыто на `main` по состоянию на 2026-04-26 (v2.5.0)

**Полный тех-аудит (история):** [docs/REALITY_AUDIT_2026-04-23.md](docs/REALITY_AUDIT_2026-04-23.md)
**Forensic-отчёт:** [AUDIT_MARKETPLACE_v2.4.1.md](AUDIT_MARKETPLACE_v2.4.1.md)

**Тесты:** `go test -count=1 ./...` — green на всех пакетах (gateway, handlers, hub, middleware, models, tests).

**Закрыто в sessions 2026-04-18 — 2026-04-26:**
- [x] **A1/A3/B2/B3/C1/C2/D1/D3/E1/E2/F1/G3** — аудит v2.4.1 (18 апреля)
- [x] **TMA P1 (#3–#8)** — JWT revocation (jti + `tma_revoked_sessions`), JOIN ownership check, device-level spam limit, fingerprint binding, telegram_id removed from `/auth`, DID uniqueness в TmaStake (коммит `921d50a6`, 25 апреля)
- [x] **E3** — двухсторонний cross-check в `FinalizeSession`: worker > gateway → use worker; gateway > 2×worker → cap at worker (защита покупателя) (коммит `5b758171`)
- [x] **E4** — `verifyPopSignature` обязателен для `heartbeat` и `pong`; WS-level pong без PoP-награды (подтверждено в `hub/client.go`)
- [x] **G2 (IPv6 /48 Sybil)** — `toSubnetPrefix` в `fraud.go` и `feeder.go`: IPv4=/24 LIKE, IPv6=/48 `inet(ip) << $1::inet`
- [x] **F3 (float truncation)** — `roundExra(v) = math.Round(v×1e8)/1e8` в `DistributeReward`; `effectiveAmount`, `referralReward`, `treasuryReward` округляются, `workerReward` = remainder
- [x] **P2 HRW** — `calculateBidScore` + `fnv32a(sessionID+nodeID)/MaxUint32 × 0.05` tiebreaker; hot-node распределение
- [x] **P2 Parallel PoP** — `StartPopWorker` с `semaphore(8)` вместо serial
- [x] **Resident IP (Android P1)** — `TunnelWorker`: правильные заголовки `X-Device-ID/X-Device-Sig`, подпись только `sessionId`, ожидание обоих pipe-потоков (`done.size >= 2`)
- [x] **sr25519 signing context** — `IdentityManager.signData` использует `"substrate"` context (совместимость с Go `schnorrkel.NewSigningContext`)
- [x] **Feeder audit (Android P2)** — `feeder_audit` handler активен: `performFeederCheck` → подпись `"$assignmentId:$targetDeviceId:$verdict"` → `feeder_report`
- [x] **Live stats UI (Android P2)** — Stats card (TUNNELS, PROXIED, GEARSCORE, CREDITS); `broadcastLocalStats` каждые 30s
- [x] **targetDeviceID в feeder WS** — `BroadcastFeederTask` пробрасывает `target_device_id` в WS-пакет

**Остаётся открытым:**
- [ ] **Go ↔ peaq bridge** — `server/peaq/peaq_client.go` payload'ы не совпадают с текущими `batch_mint/update_stats` extrinsic signatures pallet. Нужна live-chain сверка перед mainnet.
- [ ] **B1 Redis integration test** — `AtomicClaimNode` в production path есть, но честного Redis-backed integration теста нет.
- [ ] **G2-deep (ASN Sybil)** — ASN-level фермы (несколько /48 от одного AS) не детектируются без MaxMind/IPAPI интеграции.
- [ ] **Canary end-to-end** — серверный hash убран, но binding к реальному proxy challenge + feeder-side traffic verification требуют усиления.
- [ ] **Desktop агент** — skeleton, далеко до production.

**Компоненты «бетон» (не трогать без причины):** `models/session.go::FinalizeSession`, `models/pop.go` (`idempotencyKey`, `roundExra`, `DistributeReward`), `models/fraud.go::FreezeNode`, `popChannel`.

### MVP/Launch Блокеры (PEAQ Transition):
- [x] **PEAQ:** Реализация peaq Pallet (Rust) для `pallet_exra` есть в репо.
- [~] **PEAQ:** Go Client в сервере подключён, но по Reality Audit его payload'ы не совпадают с текущим pallet API. Нельзя считать это 100% done до live-chain сверки.
- [x] **PEAQ:** peaq SDK в Android-приложение для формирования DID интегрирован.
- [~] **Oracles:** Daily Batch логика, подписи и consensus есть; реальный on-chain mint path всё ещё требует сверки против runtime metadata.
- [~] **Anti-fraud:** Feeders и Canary реализованы на минимально рабочем уровне; следующий незакрытый шаг — end-to-end challenge binding и buyer-side traffic cross-check.
- [x] **Timelock UI:** Добавить "24h Timelock bar" в UI для Anon-пользователей.
- [x] **Payout:** Velocity limit (1/24h на DID). (Done 2026-04-16)

### Лендинг
- [x] Waitlist Early Access форма: 3 роли (Tester/Investor/Buyer), Konami-пасхалка Ghost Node, Server Action → Supabase + Resend. Миграция: `migrations/022_waitlist.sql`
- [ ] Применить миграцию `022_waitlist.sql` в Supabase Dashboard
- [ ] Добавить `RESEND_API_KEY` в прод-деплой лендинга (vercel env / docker env)
- [ ] Верифицировать домен `exra.space` в Resend для отправки с `noreply@exra.space`

### Важные (Phase 2 & 3 Done)
- [x] Marketplace: фильтр нод по стране/цене в `/api/nodes`
- [x] Compute Tasks: реальная верификация результатов (ZK-light)
- [x] Task Monitor: Фоновая очистка просроченных задач
- [ ] Desktop агент (Windows)
- [ ] iOS агент (DEPRECATED - Removed from roadmap)
- [ ] Runbook: описать PEAQ deployment, удалить TON переменные

### Legacy/Выполнено (Phase 1/TON):
- [x] Security: WebSocket Header Auth (DoS защита)
- [x] Security: Traffic Boundary (защита от фейковых отчетов)
- [x] Migration runner: автоматическое применение `migrations/*.sql`
- [x] [Deprecated] TON: TEP-74 соответствие

---

## 12. Запуск локально

```bash
# 1. Зависимости
cd server && go mod download

# 2. БД (Supabase или локальный PostgreSQL)
# Применить миграции вручную: server/migrations/*.sql

# 3. Redis
docker run -p 6379:6379 redis:alpine

# 4. .env файл
cp .env.example .env
# Заполнить переменные

# 5. Сервер
cd server && go run .

# 6. Тесты
go test -timeout 60s ./middleware/... ./models/... ./hub/... ./tests/...
```

---

## 13. Соглашения для разработки

- **Новая фича = задача в этом файле** (секция 11) прежде чем писать код
- **Статусы в секции 3 обновлять** при каждом значимом изменении
- **Секреты не коммитить.** `.env` в `.gitignore`
- **Бинари не коммитить.** `*.exe`, `*.bin` — собирать локально
- **Каждый эндпоинт требует:** валидации входных данных, обработки ошибок без раскрытия деталей, логирования
- **DB операции:** использовать транзакции для любых multi-step операций
- **Публичные эндпоинты:** только `PublicNode` struct, никогда `Node` напрямую

---

## 14. Android Toolchain (Kotlin 2.x Migration)

**Стек после миграции 2026-04-17:**
- Kotlin: `2.2.10` (root `build.gradle`, plugin `org.jetbrains.kotlin.android`)
- Compose Compiler Plugin: `org.jetbrains.kotlin.plugin.compose:2.2.10` (в Kotlin 2.x — отдельный плагин, `composeOptions.kotlinCompilerExtensionVersion` больше не используется)
- Android Gradle Plugin: `9.1.1`, Gradle wrapper `9.3.1`
- Compose BOM: `2024.12.01`
- kotlinx-coroutines-android: `1.9.0`
- SDK ноды: `store.silencio:peaqsdk:1.0.15` + `silenciopeaqsdk-native:2.1.4` (пакеты `dev.sublab.*`, НЕ Nova `io.novasama.*`)

**Почему миграция обязательна:** бинарники `store.silencio:peaqsdk:1.0.15` скомпилированы Kotlin-метадатой `2.3.0` — Kotlin 1.9 компилятор физически не может их прочитать.

**Фиксы в `app/build.gradle` (нельзя откатывать без причины):**
- `configurations.all { exclude group: 'org.bouncycastle', module: 'bcprov-jdk15on' }` — убирает дубликаты классов BouncyCastle (web3j 4.9.5 тянет старый `jdk15on:1.68`, наш явный `bcprov-jdk15to18:1.78.1` конфликтовал).
- `freeCompilerArgs += '-Xskip-metadata-version-check'` в `kotlinOptions` — страховка на случай если SDK обновит metadata быстрее компилятора.
- Явные пины `dev.sublab:common-kotlin:1.0.0` и `dev.sublab:sr25519-kotlin:1.0.1` — версии `1.1.x` в Maven Central не существуют.

**Импорты в Kotlin-коде ноды:** использовать `dev.sublab.encrypting.keys.KeyPair`, НЕ `io.novasama.substrate_sdk_android.*` (последние остались от старого Nova SDK).

---

## 15. TMA Security — P1 hardening (2026-04-25, коммит `921d50a6` + 2026-04-26 v2.5.1)

После закрытия P0 (cookie-session, initData TTL, ownership-checks, Sybil-лимит, rate-limit, удаление `X-Node-Secret` из прокси) все **P1-риски закрыты**:

1. ✅ **`TMA_SESSION_SECRET` fallback** — `tma_auth.go` делает `log.Fatal` если `GO_ENV=production` и секрет пуст или равен дефолту.

2. ✅ **CSRF: SameSite=None + `TMARequireOrigin`** — cookie остаётся `SameSite=None` (требование Telegram iframe; Lax не работает в cross-site frame). Защита перенесена на серверный Origin/Referer-whitelist (`TMA_ALLOWED_ORIGINS`). Мутирующие эндпоинты (`/withdraw`, `/stake`, `/link-device`, `/lots/*`, `/push-token`) обёрнуты в `TMARequireOrigin` (v2.5.1, 26 апреля). При пустом whitelist — WARN в лог, доступ открыт (dev-режим).

3. ✅ **JWT revocation (jti + blacklist)** — `jti` в JWT claims; таблица `tma_revoked_sessions`; проверка в `TMAAuth` middleware. Migration 029 добавлена.

4. ✅ **Ownership + DID — один JOIN** — `requireDeviceOwnershipAndDID` делает один `JOIN tma_devices ON device_id ... WHERE telegram_id=$1 AND status='linked'`.

5. ✅ **Approval-spam per-device** — ≥3 pending approvals в час на `device_id` → 429, независимо от IP.

6. ✅ **Fingerprint binding** — при WS `link_response` Android-клиент шлёт `hw_hash` (v2.5.1, 26 апреля: добавлено `IdentityManager.getHardwareFingerprint()` в `sendLinkResponse`); сервер сверяет с `nodes.hw_fingerprint`; mismatch = reject. Первое подключение записывает fingerprint.

7. ✅ **`telegram_id` убран из `/auth`** — поле удалено из `writeAccountSummary`.

8. ✅ **DID uniqueness в TmaStake** — explicit check перед `UpgradeNodeToPeak`; дублирование DID блокируется.

9. ✅ **TMA earnings SQL fix (v2.5.1, 26 апреля)** — `writeAccountSummary`, `TmaMe`, `TmaStake` использовали `oracle_batches.oracle_id = device_id` (сравнение device_id юзера с ID одного из 3 оракулов) → `batched_usd` всегда был 0, кнопка Withdraw disabled, апгрейд в Peak недостижим. Исправлено на `JOIN node_earnings e ON e.batch_id = b.id WHERE e.device_id = X AND b.status = 'applied'`.

**Env для прода (обязательно):**
- `TELEGRAM_BOT_TOKEN`, `TMA_SESSION_SECRET` (min 32 bytes random), `TMA_API_BASE` — без них сервер не стартует (`log.Fatal`).
- `TMA_ALLOWED_ORIGINS` — comma-separated whitelist (например, `https://app.exra.space,https://exra.space`). Без него CSRF-защита не активна (только WARN). **Обязательно для production.**

