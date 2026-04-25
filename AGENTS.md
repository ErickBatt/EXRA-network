# EXRA — Единый документ проекта

> **Читай этот файл в начале каждой сессии.**
> С 23 апреля 2026 whitepaper (`EXRA White Paper_ Sovereign DePIN Infrastructure.pdf`) — главный продуктовый документ проекта.
> `AGENTS.md` — инженерный ledger: что реально реализовано в коде, что живёт только в ветках, и что ещё не production-ready.
> При конфликте по статусу реализации выигрывают код, тесты и [docs/REALITY_AUDIT_2026-04-23.md](docs/REALITY_AUDIT_2026-04-23.md).
> Последнее обновление: 25 апреля 2026 (waitlist-форма на лендинге: Server Action + Supabase + Resend email-нотификации)

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
| Go сервер (core + gateway) | 🟡 ~88% impl / ~68% prod | `go test ./...` в `server` green; A1/A3/B2/B3/C1/C2/D1/D3/E1/E2(min fix)/F1/G3 закрыты кодом, но остаются TMA P1, buyer-side traffic cross-check, IPv6 farms и peaq bridge mismatch |
| Android APK | 🟡 ~70% impl / ~55% prod | Peaq DID, WS, heartbeat, TunnelWorker, compute_result и новый toolchain есть; full production proof и завершённый anti-fraud path не доказаны |
| Dashboard / Marketplace | 🟡 ~75% impl / ~60% prod | Marketplace, buyer auth, TMA pages и buyer-api proxy есть; часть hardening ещё живёт в branch-only коммитах и рабочем дереве |
| TON смарт-контракт | 🗑️ Removed | Полностью заменено на PEAQ DePIN |
| TON интеграция в Go | 🗑️ Removed | Полностью заменено на PEAQ DePIN |
| peaq Pallet (Rust) | 🟡 ~85% impl / ~70% prod | `pallet_exra`, runtime wiring и тесты есть; реальная chain compatibility вне репо не доказана |
| Go ↔ peaq bridge | 🟡 ~45% impl / ~25% prod | RPC client и mock E2E есть, но payload'ы в `server/peaq/peaq_client.go` не совпадают с текущим pallet API |
| Migration Runner / Admin / Payouts | 🟡 ~80% impl / ~65% prod | Авто-накат миграций, admin payout flow, `mark-paid`, tx_hash audit trail и role auth есть |
| Telegram Mini App backend | 🟡 ~75% impl / ~55% prod | Cookie-session и ownership checks есть; P1 по secret fallback, SameSite, revocation, fingerprint и DID uniqueness остаются |
| Desktop агент | 🟡 ~30% impl / ~15% prod | Skeleton клиента, heartbeat, tunnel/compute simulation; далеко до production |
| Dynamic Pricing | 🟡 ~80% impl / ~65% prod | Marketplace-фильтры и Avg Price работают, но общая buyer-security hardening ещё не вся в `main` |
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

### 🔴 Reality Audit (2026-04-23) — что реально открыто после сверки кода и веток

**Полный тех-аудит:** [docs/REALITY_AUDIT_2026-04-23.md](docs/REALITY_AUDIT_2026-04-23.md)
**Исторический forensic-отчёт:** [AUDIT_MARKETPLACE_v2.4.1.md](AUDIT_MARKETPLACE_v2.4.1.md)

**Что реально подтверждено тестами на 2026-04-23:**
- [x] `cd server && go test -count=1 ./...` — полностью green
- [x] `server/gateway/stitch_test.go` — gateway green; A1/A3 не воспроизводятся
- [x] `server/handlers/matcher_concurrency_test.go` — legacy proof для B1 переведён в skip при наличии `AtomicClaimNode`
- [x] `server/hub/client_trust_test.go` — обновлён до regression-тестов по фактическому коду; universal canary hash больше не текущий баг

**Уже закрыто кодом на `main`, но старый AGENTS продолжал считать открытым:**
- [x] **A1:** `gateway/sessions.go::Stitch` переведён на `sync.Once` + `done`
- [x] **A3:** `gateway/bridge.go` выставляет `SetReadDeadline(90s)`
- [x] **B2:** `handlers/matcher.go` использует формулу `0.5*RS + 0.25*uptime + 0.25*priceFitness`
- [x] **B3:** `HoldBalance(...)` стоит до выдачи Gateway JWT
- [x] **C1:** `hub.cleanupLoop` теперь TTL-per-entry, а не bulk reset
- [x] **C2:** `subscribeRedis*` завёрнуты в reconnect loops
- [x] **D1:** Gateway signing path ушёл от старого hardcoded fallback в matcher flow
- [x] **D3:** `handlers/ws.go` использует origin whitelist через `WS_ALLOWED_ORIGINS`
- [x] **E1:** `feeder_report` требует подпись через `VerifyDIDSignature`
- [x] **E2:** universal canary literal убран; `CreateCanaryTask` генерирует per-task hash
- [x] **F1:** `oracle.ProcessOracleProposal` теперь верифицирует подпись до consensus
- [x] **G3:** gateway byte accounting и Redis billing settlement реализованы

**Частично закрыто, но нельзя считать fully done:**
- [~] **B1:** в production path есть `AtomicClaimNode(...)`, но нужен честный Redis-backed integration test
- [~] **E3:** `hub/client.go` теперь clamp'ит worker-reported bytes через `MaxTrafficPerSec`, но buyer-side counter cross-check в `models/session.go::FinalizeSession` ещё не реализован
- [~] **Canary anti-fraud:** серверный universal hash убран, но end-to-end binding к реальному proxy challenge и feeder-side traffic verification ещё требуют усиления

**Подтверждённо открыто на `main`:**
- [ ] **TMA P1:** см. секцию 15 (`TMA_SESSION_SECRET` fallback, SameSite, revoke, fingerprint, DID uniqueness)
- [ ] **E3 final hardening:** buyer-side traffic cross-check всё ещё отсутствует в `FinalizeSession`
- [ ] **E4:** `toSubnet24` не покрывает IPv6 фермы
- [ ] **Go ↔ peaq bridge:** `server/peaq/peaq_client.go` не совпадает с текущими `batch_mint/update_stats` extrinsic signatures pallet

**Важные branch-only коммиты, которые ещё не надо потерять:**
- [x] `claude/brave-lewin-2497be` уже поглощена `main`
- [x] Ряд high-priority commits из `claude/bold-carson-9596fe`, `claude/happy-einstein-60e24d` и `claude/agitated-burnell-563cba` был перепроверен cherry-pick'ами в отдельной integration-ветке
- [x] Большинство этих cherry-pick'ов оказались пустыми или конфликтовали только на уже обновлённых местах, то есть их содержательная часть уже присутствует в `main` или в текущем рабочем дереве
- [ ] Отдельной «волшебной ветки», которая делает проект 100%, не найдено

**Компоненты «бетон» (не трогать без причины):** `models/session.go::FinalizeSession`, `models/pop.go` (idempotencyKey), `models/fraud.go::FreezeNode`, `popChannel`, `DistributeReward`.

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

## 15. TMA Security — остаточные P1 (после hardening-PR 2026-04-18)

После закрытия P0 (cookie-session, initData TTL, ownership-checks, Sybil-лимит, rate-limit, удаление `X-Node-Secret` из прокси) остались **P1-риски**, которые нужно закрыть отдельными PR перед масштабированием аудитории TMA:

1. **`TMA_SESSION_SECRET` fallback** — [server/middleware/tma_auth.go:57-65](server/middleware/tma_auth.go) возвращает hardcoded dev-секрет при отсутствии env. **Fix:** `log.Fatal` если `GO_ENV=production` и `TMA_SESSION_SECRET` пуст. Деплой без него = все JWT подделываются.

2. **SameSite=Strict ломает Telegram iOS WebView** — текущая `Strict` cookie может не передаваться в in-app браузере Telegram iOS. **Fix:** перевести на `SameSite=Lax` + явная проверка `Origin`/`Referer` на мутирующих эндпоинтах (`/withdraw`, `/stake`, `/link-device`).

3. **Нет revocation JWT** — утечка cookie = 24h окно без возможности отозвать. **Fix:** добавить `jti` claim + Redis-blacklist, либо таблицу `tma_sessions(sid, revoked_at)` с проверкой в `TMAAuth` middleware.

4. **Ownership + DID lookup — два запроса вместо JOIN** — [tma.go](server/handlers/tma.go) делает `AssertDeviceOwnedByTelegram` + `SELECT did FROM nodes` последовательно. **Fix:** один запрос `JOIN tma_devices td ON td.device_id=n.device_id WHERE td.telegram_id=$1 AND td.status='linked'`.

5. **Approval-spam per-device** — текущий `ScopedRateLimit("tma-link", 0.05, 3)` per-IP легко обходится пулом IP для атаки на конкретный `device_id`. **Fix:** лимит «не более N pending approvals в час на device_id», независимо от источника.

6. **Fingerprint binding (Android ID + hw hash) — не реализован** — approval-flow НЕ сверяет `hw_hash` устройства с сохранённым в `nodes.hw_fingerprint`. Требование AGENTS.md §Security не выполнено. **Fix:** при WS-approval Android-клиент шлёт свежий fingerprint → сервер сверяет → mismatch = reject.

7. **`/auth` ответ содержит `telegram_id` в body** — после перехода на cookie это избыточная инфо-утечка. **Fix:** убрать поле из `writeAccountSummary`, клиент всё равно работает по сессии.

8. **`TmaStake` — нет гарантии уникальности DID** — если два `device_id` с одним DID окажутся привязаны к одному tg_id, возможен gamed-stake. **Fix:** `UNIQUE(did)` либо explicit check перед `UpgradeNodeToPeak`.

**Env для прода (обязательно):** `TELEGRAM_BOT_TOKEN`, `TMA_SESSION_SECRET` (min 32 bytes random), `TMA_API_BASE`. При отсутствии любого — отказываться стартовать.

