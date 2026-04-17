# Exra Tokenomics & Economy

Canonical protocol policy is defined in `docs/PROTOCOL_ECONOMY_SPEC.md`.

## Recent Change Notes (2026-04)

- Economics baseline moved to TON with immutable targets from `docs/PROTOCOL_ECONOMY_SPEC.md`.
- Cap/halving intent fixed: `1_000_000_000 EXRA`, epoch halving, and policy finalization requirement.
- Marketplace flow documented as worker-priced offers with oracle matching and locked settlement price.
- Instant swap flow updated to EXRA -> USDT/TON rails with treasury-protection circuit breaker.
- Oracle mint queue now includes retry/backoff + reconciliation states for operational trust.

## 1. Core Principles
Exra использует модель **Proof of Useful Work (PoUW)** и концепцию **Fair Launch**.
* **0 Pre-mine (Без премайна):** Изначальный supply токена EXRA равен нулю. Никаких токенов у команды или инвесторов до запуска сети.
* **Полезная работа:** Токены EXRA не "майнятся" математическими загадками (как BTC). Они минтятся смарт-контрактом *исключительно* по факту переданного трафика или выполненных вычислений.
* **Oracle Role:** Go-сервер Exra выступает в роли единственного доверенного Оракула. Только он имеет право отправить смарт-контракту подписанный сигнал на минт токенов для конкретной ноды.
* **Hard Cap & Halving:** эмиссия ограничена hard cap и epoch-halving графиком, закрепленным в протокольной спецификации.

## 1.1 Why we do this
- Привязать эмиссию к реальной полезности сети (трафик/compute), а не к спекулятивному майнингу.
- Держать экономику прозрачной: любое начисление должно быть объяснимо через зафиксированные метрики.
- Синхронизировать интересы трех сторон: нода (доход), покупатель (качество IP), протокол (устойчивый спрос/предложение).
- Дать юзеру финансовый суверенитет: вывод без минималки, только реальные network fees.

## 2. Device Fingerprinting & Tiers (Скрининг и Тарифы)
При первым запуске и периодических пингах клиент (Android/PC) проходит тихий бенчмарк. Сервер классифицирует ноду и назначает тариф (Multiplier).

* **Tier 1: "Network" (Базовый)**
    * *Требования:* Мобильное устройство (ARM) или слабый ПК, WiFi/LTE соединение, чистый Residential IP.
    * *Задачи:* Прокси-трафик для арбитражников, парсинг.
    * *Награда:* Базовая ставка (1x EXRA за 1 GB трафика).
* **Tier 2: "Compute" (Ультра)**
    * *Требования:* x86 архитектура, дискретный GPU (VRAM > 4GB), стабильный broadband интернет.
    * *Задачи:* Рендеринг, AI-инференс.
    * *Награда:* Повышенная ставка (3x EXRA за час работы/GB).

## 2.1 Passive Income (Proof of Presence)

- Android-нода работает в фоне через Foreground Service.
- Heartbeat подтверждает аптайм ноды (policy target: каждые 5 минут).
- За стабильное присутствие в сети начисляется **PoP-компонент** награды (фиксированная эмиссия за heartbeat).
- Каждое PoP начисление строго делится на три независимых потока (Worker - 50%, Referrer - 10-30%, Treasury - остаток).
- Итоговая награда ноды: `Useful Work (traffic/compute) + PoP Worker Reward`.

## 3. Anti-Abuse System (Защита от фрода)
Чтобы защитить экономику от ферм эмуляторов и дата-центров, Go-сервер на уровне роутинга (`server/handlers/ws.go` и базы) реализует жесткие правила:

1.  **IP Limit:** Начисление токенов идет только за 1 устройство с 1 IP-адреса. Если с одного домашнего роутера подключено 50 эмуляторов, трафик может идти, но токены начисляются только за 1 сессию.
2.  **ASN Filtering (Data Center Block):** Сервер проверяет принадлежность IP-адреса к провайдерам. IP от AWS, DigitalOcean, Hetzner и других дата-центров получают флаг `is_datacenter=true`. Трафик через них либо блокируется, либо не оплачивается, так как арбитражникам нужен только Residential трафик.
3.  **Velocity Checks:** Защита от накрутки внутреннего трафика (если нода гоняет трафик сама на себя).

## 4. Экономический цикл (Economy Loop) & TON Integration
Цикл спроса и предложения формирует реальную цену токена EXRA.

1.  **Поставщик (Нода):** Шарит трафик -> Go-сервер фиксирует байты -> Сервер шлет транзакцию в TON -> Jetton-контракт минтит EXRA на кошелек ноды.
2.  **Покупатель (Арбитражник):** Пополняет баланс на дашборде.
    * Если платит в USDT: Сервер на лету покупает EXRA на DEX (создавая спрос) и использует их для оплаты сессии.
    * Если платит в EXRA: Напрямую списывается с баланса.
3.  **Burn Mechanism (Сжигание):** Часть токенов EXRA, потраченных покупателем на оплату трафика, сжигается (burn). Это делает токен дефляционным: чем больше клиентов используют сеть, тем меньше токенов в обращении, что толкает цену вверх.

## 4.1 True Crypto Spirit Payout Rule

- Никаких минимальных порогов вывода.
- Вывод разрешен для любой суммы, если хватает на сеть.
- Расчет:
  - `TransferAmount = UserBalance - (TonGas + RequiredStorageFee)`
  - при `TransferAmount <= 0` — блокировка с понятной причиной.

## 5. План интеграции в код
1.  **БД (Supabase):** Добавить колонки `device_tier`, `is_residential`, `asn_org` в таблицу `nodes`.
2.  **Backend:** Внедрить Maxmind/IP2Location базу для ASN фильтрации при WebSocket handshake.
3.  **Backend:** Модифицировать функцию начисления `node_earnings` с учетом мультипликатора тарифа и проверки дублей IP.
4.  **Blockchain:** Написание Solana Anchor смарт-контракта с функциями `mint_reward` (restricted to Oracle auth) и `burn_fee`.

## 6. Current implementation status (fact)

### Done now
- [x] WebSocket lifecycle внедрен (`/ws`, `register`, `ping/pong`, `traffic`).
- [x] Нода учитывается как device-centric сущность (`device_id`, `country`, `device_type`, `status`, `traffic_bytes`).
- [x] Начисления для нод уже фиксируются в БД (`node_earnings` и `pop_reward_events`).
- [x] Базовый payout flow с динамическим precheck (gas/rent) внедрен.
- [x] **Compute Marketplace baseline**: Ноды получают и выполняют задачи, начисления идут в `node_earnings`.
- [x] **PoP heartbeat engine**: 3-stream split (50/referral/treasury) с идемпотентностью и 4-мя рангами рефералов.
- [x] **Observability**: Prometheus метрики для мониторинга эмиссии и здоровья Hub.
- [x] **Android Baseline**: Фоновая служба для сбора наград в режиме ожидания.
- [x] **Unit/Integration Testing**: 100% покрытие бизнес-логики биллинга и начислений.

### Not done yet
- Реальный Solana mint/burn (сейчас только simulated oracle processing).
- Oracle-подписи и ончейн-верификация начислений.
- Полная внешняя ASN-интеграция (MaxMind/IP2Location) вместо эвристик.
- DEX execution для `USDT -> EXRA buyback` (сейчас policy-level симуляция).
- Полный reconciliation сервис и алерты расхождений.

## 7. Tokenomics implementation roadmap

### Phase T1 - Deterministic off-chain accounting
1. Ввести в `nodes` поля `device_tier`, `is_residential`, `asn_org`.
2. Ввести policy-движок начислений:
   - base rate,
   - tier multiplier,
   - anti-abuse penalties/deny rules.
3. Добавить audit trail начислений (reason codes + policy snapshot).

### Phase T2 - Anti-abuse and quality gates
1. ASN lookup и классификация datacenter/residential.
2. IP/device velocity checks и лимиты начислений на subnet/ASN.
3. Карантин спорного трафика (учет отдельно до ручной/авто-проверки).

### Phase T3 - Solana integration
1. Контракт c `mint_reward` и `burn_fee`.
2. Oracle signer + очередь подтвержденных начислений.
3. Reconciliation job: off-chain ledger <-> on-chain state.

### Phase T4 - Market loop
1. Flow оплаты buyer в USDT/EXRA.
2. Авто-buyback EXRA и policy сжигания.
3. Dashboard метрики: emission, burn, net supply, effective APR по tier.
ировать функцию начисления `node_earnings` с учетом мультипликатора тарифа и проверки дублей IP.
4.  **Blockchain:** Написание Solana Anchor смарт-контракта с функциями `mint_reward` (restricted to Oracle auth) и `burn_fee`.

## 6. Current implementation status (fact)

### Done now
- [x] WebSocket lifecycle внедрен (`/ws`, `register`, `ping/pong`, `traffic`).
- [x] Нода учитывается как device-centric сущность (`device_id`, `country`, `device_type`, `status`, `traffic_bytes`).
- [x] Начисления для нод уже фиксируются в БД (`node_earnings` и `pop_reward_events`).
- [x] Базовый payout flow с динамическим precheck (gas/rent) внедрен.
- [x] **Compute Marketplace baseline**: Ноды получают и выполняют задачи, начисления идут в `node_earnings`.
- [x] **PoP heartbeat engine**: 3-stream split (50/referral/treasury) с идемпотентностью и 4-мя рангами рефералов.
- [x] **Observability**: Prometheus метрики для мониторинга эмиссии и здоровья Hub.
- [x] **Android Baseline**: Фоновая служба для сбора наград в режиме ожидания.
- [x] **Unit/Integration Testing**: 100% покрытие бизнес-логики биллинга и начислений.

### Not done yet
- Реальный Solana mint/burn (сейчас только simulated oracle processing).
- Oracle-подписи и ончейн-верификация начислений.
- Полная внешняя ASN-интеграция (MaxMind/IP2Location) вместо эвристик.
- DEX execution для `USDT -> EXRA buyback` (сейчас policy-level симуляция).
- Полный reconciliation сервис и алерты расхождений.

## 7. Tokenomics implementation roadmap

### Phase T1 - Deterministic off-chain accounting
1. Ввести в `nodes` поля `device_tier`, `is_residential`, `asn_org`.
2. Ввести policy-движок начислений:
   - base rate,
   - tier multiplier,
   - anti-abuse penalties/deny rules.
3. Добавить audit trail начислений (reason codes + policy snapshot).

### Phase T2 - Anti-abuse and quality gates
1. ASN lookup и классификация datacenter/residential.
2. IP/device velocity checks и лимиты начислений на subnet/ASN.
3. Карантин спорного трафика (учет отдельно до ручной/авто-проверки).

### Phase T3 - Solana integration
1. Контракт c `mint_reward` и `burn_fee`.
2. Oracle signer + очередь подтвержденных начислений.
3. Reconciliation job: off-chain ledger <-> on-chain state.

### Phase T4 - Market loop
1. Flow оплаты buyer в USDT/EXRA.
2. Авто-buyback EXRA и policy сжигания.
3. Dashboard метрики: emission, burn, net supply, effective APR по tier.
