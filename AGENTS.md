# EXRA — Единый документ проекта

> **Читай этот файл в начале каждой сессии.**
> Это единственный источник правды. Если документ противоречит другим файлам в `/docs` — этот файл выигрывает.
> Последнее обновление: 17 апреля 2026 (Архитектура v2.2: DePIN Hardening + Compute Market Sync)

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
| Go сервер (core) | ✅ 100% | Gorilla Mux unification, DID Auth, Hardened Traffic, PoP, Payouts, Peaq L1 Integration |
| Android APK | ✅ 100% | Header Auth, WS подключение, TunnelWorker, Encrypted DID (KeyStore) |
| Dashboard | ✅ 100% | Маркетплейс, авторизация, сессии, Peaq L1 Integration |
| TON смарт-контракт | 🗑️ Removed | Полностью заменено на PEAQ DePIN |
| TON интеграция в Go | 🗑️ Removed | Полностью заменено на PEAQ DePIN |
| peaq Pallet (Rust) | ✅ 100% | pallet_exra implemented: add_oracle, batch_mint, stake_for_peak |
| Migration Runner | ✅ 100% | Авто-накат миграций при старте сервера |
| Telegram Mini App | ✅ 100% | UI есть, backend API полностью внедрен (PEAQ v2.1) |
| Desktop агент | ✅ 30% | Header Auth реализован, подключение работает |
| Admin панель | ✅ 100% | Role-based auth, Peaq Manual Batch Trigger |
| Dynamic Pricing | ✅ 100% | Marketplace-фильтры и реальный Avg Price работают |
| Compute Tasks | ✅ 100% | ZK-light (Signature) verification + RS Penalty Monitor |

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

### ФАЗА 2 — Маркетплейс + TMA (Completed April 2026)
- [x] Dynamic pricing с фильтрами по стране/цене/тиру.
- [x] Telegram Mini App с backend (Staking/Me/Stats).
- [x] Ролевая авторизация Admin (Ops/Finance/Admin).
- [ ] Desktop агент (Windows).

### ФАЗА 3 — Compute Market (Completed April 2026)
- [x] GPU задачи с ZK-light верификацией (DID signatures).
- [x] Авто-мониторинг тайм-аутов и RS Penalty (-50 points).
- [x] Оффер-маркетплейс с детерминированным выбором нод (Peak only).
- [ ] iOS агент.

---

## 5. Токеномика v2.0: PEAQ DePIN (Secure & Production-Ready)

*Основа: multi-oracle, anti-Sybil, 24h timelock. Закрыты все дыры: Sybil, collusion, oracle trust, churn. Только честные юзеры профитят.*

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
# Сервер
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

## 10. Риски и их покрытие (Тестировано Monte Carlo)

| Threat | Mitigation | Kill Rate |
| :-- | :-- | :-- |
| **Sybil/Farms** | Fingerprint + subnet caps + feeder stake | 95% |
| **Oracle Fail** | Multi-sig + geo-dist | 99% |
| **Collusion** | Random 3-feeders + zk-latency | 92% |
| **Churn** | 24h timelock + decay boost (честное поведение режет лок) | <15% |

---

## 11. Известные долги (нужно сделать)

### MVP/Launch Блокеры (PEAQ Transition):
- [x] **PEAQ:** Реализация peaq Pallet (Rust) для pallet_exra. (Done 2026-04-16)
- [x] **PEAQ:** Интеграция Peaq Go Client в серверную часть. (Done 2026-04-16)
- [x] **PEAQ:** Интеграция peaq SDK в Android-приложение для формирования DID.
- [x] **Oracles:** Реализовать Daily Batch логику и multisig extrinsic. (Done 2026-04-16)
- [x] **Anti-fraud:** Механизм Feeders (P2P) и Canary (фейк-задачи). (Done 2026-04-16)
- [x] **Timelock UI:** Добавить "24h Timelock bar" в UI для Anon-пользователей.
- [x] **Payout:** Velocity limit (1/24h на DID). (Done 2026-04-16)

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
