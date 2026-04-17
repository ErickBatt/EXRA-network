# EXRA DePIN — Implementation Spec v1
> Единый документ приоритетов разработки на основе принятых решений.
> Дата: апрель 2026

---

## Проблема

EXRA имеет рабочий backend (прокси, PoP, сессии) но не может выплачивать реальные токены пользователям — TON интеграция в mock-режиме. Без реальных выплат невозможно привлечь первых воркеров и проверить продукт на живых пользователях.

**Кто страдает:** воркеры из Индии, ЮВА, Африки которые подключились но не могут вывести заработок.  
**Цена бездействия:** нулевое доверие к продукту, отток первых пользователей до монетизации.

---

## Цели

1. Первый реальный вывод EXRA на TON кошелёк в течение 2 недель
2. Себестоимость газа для нас = $0 (пользователь платит сам через вычет)
3. Воркер выводит без TON на балансе — нулевой барьер входа
4. Oracle кошелёк самопополняется автоматически — не требует ручного вмешательства
5. Supply-based halving + Gear Score уже в коде — нужна только TON интеграция

---

## Не входит в v1

- Пулы и рейтинги пулов → Phase 3
- Batch mint (одна транзакция на 100 человек) → Phase 2
- DEX интеграция реальных цен (STON.fi/DeDust) → Phase 2
- iOS агент → Phase 3
- TMA backend → Phase 2 (но критично)

---

## Фичи по приоритету

---

### P0 — TON Jetton контракт (блокирует всё)

**Проблема:** без контракта нет минтинга, нет выплат, продукт не работает.

**Решение:**
- Взять официальный `ton-blockchain/jetton-contract` (TEP-74)
- Добавить одну проверку: mint только от Oracle адреса
- Задеплоить через Blueprint (Node.js)

**Параметры контракта:**
```
name:           Exra
symbol:         EXRA  
decimals:       9
max_supply:     1,000,000,000
admin/minter:   UQC4u_AdYRtVgG8gi4AuNfwVOfIeTz6FNJDRQlswbvsNAtZo
deployer:       UQCk4JKBzZ4CKV9aAFJ8YCh0AyoKjW6cSrvHCsCMmbmDpS1A
network:        mainnet
```

**Acceptance criteria:**
- [ ] Контракт задеплоен на mainnet
- [ ] Только Oracle адрес может вызвать mint
- [ ] Попытка mint с другого адреса → revert
- [ ] JETTON_MASTER_ADDRESS прописан в .env сервера
- [ ] `MOCK_TON=false` — сервер работает с реальным TON

**Зависимости:** Blueprint, Node.js, ~0.3 TON на деплой с основного кошелька  
**Оценка:** 1-2 дня

---

### P0 — Реальный TON mint в Go сервере

**Проблема:** `ton/mint.go` использует `GenerateMockMintBOC()` — фейковые хэши.

**Решение:**
- Подключить `tonutils-go` библиотеку
- Реализовать подпись реального BOC через seed phrase Oracle кошелька
- Отправка через TON RPC (toncenter.com/api/v2)

**Что менять в коде:**
```
server/ton/client.go  → реальный SendBoc() через tonutils-go
server/ton/mint.go    → убрать GenerateMockMintBOC(), использовать реальный BOC
server/config/config.go → ORACLE_PRIVATE_KEY (seed phrase или hex key)
```

**Acceptance criteria:**
- [ ] `go test ./ton/...` проходит с реальным RPC (testnet сначала)
- [ ] Тестовый вывод: 1 EXRA приходит на кошелёк
- [ ] Статус транзакции проверяется через `GetTxStatus()`
- [ ] При ошибке — retry с экспоненциальным backoff (уже есть)

**Зависимости:** задеплоенный контракт (P0 выше), ORACLE_PRIVATE_KEY  
**Оценка:** 2-3 дня

---

### P0 — Withdrawal flow (3 варианта для пользователя)

**Проблема:** сейчас один вариант вывода, нет UI выбора.

**UX flow:**
```
Баланс: 100 EXRA ≈ $1.20

[Вывести] →

Выберите формат:
○ USDT   → получишь ~$1.03  (gas $0.05 + спред 10%)
○ EXRA   → получишь 98.5 EXRA на TON кошелёк  
○ TON    → получишь ~0.21 TON по курсу

Адрес кошелька: [________________]

ℹ️ TON на балансе не нужен — просто укажи адрес
```

**Бизнес-правила:**
- Минимальный вывод: $1 (чтобы gas не съедал > 5%)
- Gas = 0.05 TON вычитается из суммы пользователя
- Максимум 1 вывод за 24 часа (уже реализовано)
- TON на кошельке пользователя НЕ нужен

**API:**
```
POST /api/payout/request
{
  "device_id": "...",
  "recipient_wallet": "UQ...",
  "amount_usd": 1.50,
  "output_currency": "USDT" | "EXRA" | "TON"
}
```

**Acceptance criteria:**
- [ ] Пользователь выбирает валюту вывода
- [ ] Сумма пересчитывается с учётом gas и спреда
- [ ] Вывод EXRA → mint на TON кошелёк
- [ ] Вывод USDT → перевод USDT из Treasury vault
- [ ] Вывод TON → конвертация через treasury по курсу
- [ ] Кошелёк валидируется (формат UQ.../EQ...)
- [ ] Ошибка если сумма < $1

**Оценка:** 1-2 дня

---

### P0 — Gas Pool (авто-пополнение Oracle)

**Проблема:** Oracle кошелёк тратит TON на газ при каждом минте. Нужно авто-пополнение чтобы не следить вручную.

**Схема:**
```
Пользователь выводит $5
→ вычитаем $0.05 газ
→ записываем в gas_pool: +0.05 TON эквивалент
→ фоновый воркер каждые 10 мин:
    если баланс Oracle < 2 TON:
      собрать gas_pool → отправить на Oracle одной транзакцией
```

**Миграция БД:**
```sql
CREATE TABLE gas_pool (
  id          bigserial PRIMARY KEY,
  source_id   text,           -- payout_request_id
  amount_ton  float8,         -- сколько газа собрали
  sent        boolean DEFAULT false,
  sent_at     timestamptz,
  created_at  timestamptz DEFAULT NOW()
);
```

**Go воркер:**
```go
// server/ton/gas_pool.go
func RunGasPoolWorker(oracleAddress string) {
    ticker := time.NewTicker(10 * time.Minute)
    for range ticker.C {
        balance := getOracleBalance(oracleAddress)
        if balance < 2.0 { // TON
            topUpOracle()
        }
    }
}
```

**Acceptance criteria:**
- [ ] Каждый вывод пишет в gas_pool
- [ ] Воркер стартует вместе с сервером
- [ ] При balance < 2 TON → авто-пополнение
- [ ] Логируется каждое пополнение
- [ ] При ошибке пополнения → алерт в admin incidents

**Оценка:** 1 день

---

### P0 — Wallet initialization check

**Проблема:** если у пользователя новый кошелёк без Jetton wallet — первый mint упадёт.

**Решение:**
```
Перед mint → проверить существует ли Jetton wallet
Если нет → инициализировать (стоит ~0.1 TON доп. газ)
Вычесть стоимость инициализации из суммы пользователя
```

**Acceptance criteria:**
- [ ] `GET /api/payout/precheck` возвращает `wallet_initialized: true/false`
- [ ] Если `false` — в breakdown добавляется storage_fee
- [ ] Первый вывод успешно инициализирует кошелёк

**Оценка:** 0.5 дня

---

### P1 — FOMO счётчик: "дней до халвинга"

**Проблема:** текущий `/api/tokenomics/epoch` показывает остаток токенов но не время.

**Решение:**
```
speed = EXRA намайнено за последние 24 часа
days_remaining = epoch_remaining / speed

Если speed = 0 → "зависит от активности сети"
```

**Расширение API:**
```json
GET /api/tokenomics/epoch
{
  "epoch_remaining": 47832441,
  "multiplier": 2.0,
  "epoch_name": "Genesis",
  "estimated_days_remaining": 47,    // НОВОЕ
  "mining_speed_24h": 1017000,       // EXRA/день, НОВОЕ
  "epoch_progress_pct": 52.1
}
```

**Acceptance criteria:**
- [ ] `estimated_days_remaining` считается по реальной скорости из БД
- [ ] Обновляется каждый час (кэш)
- [ ] Если скорость = 0 → null в ответе

**Оценка:** 0.5 дня

---

### P1 — Push уведомление: эпоха заполнена на 90%

**Проблема:** воркеры не знают когда Genesis заканчивается — упускают момент.

**Решение:**
- Фоновый воркер проверяет `epoch_progress_pct` каждый час
- При пересечении 90% → отправить FCM push всем Android устройствам

**Acceptance criteria:**
- [ ] Push отправляется один раз при 90% (не повторяется)
- [ ] Текст: "⚡ Genesis эпоха заполнена на 90%! Осталось X EXRA с множителем x2"
- [ ] Флаг `epoch_90pct_notified` в БД чтобы не дублировать

**Оценка:** 1 день

---

### P2 — Пулы (Phase 3)

**Концепция:** пул = группа нод с общей репутацией. Oracle приоритизирует ноды из топ-пулов.

**Treasury тиры:**

| Тир | Условие | Treasury |
|-----|---------|----------|
| Solo | < 10 нод | 30% |
| Silver | 10–99 нод, аптайм > 90% | 20% |
| Gold | 100–499 нод, аптайм > 95% | 15% |
| Platinum | 500+ нод, аптайм > 98% | 10% |

**Минимальный treasury: всегда 10%** — меньше нельзя (swap ликвидность).

**Механика:**
- Нода вступает в пул → получает тир treasury пула
- Oracle при выборе ноды для оффера → приоритет нодам из Gold/Platinum пулов
- Рейтинг пулов публичный → вирусный рост

**Acceptance criteria (Phase 3):**
- [ ] `POST /api/pool/create` — создать пул
- [ ] `POST /api/pool/{id}/join` — вступить в пул
- [ ] Treasury % автоматически меняется по тиру пула
- [ ] Публичная таблица топ пулов

**Оценка:** 5-7 дней (Phase 3)

---

## Порядок реализации

```
Неделя 1:
  День 1-2: Деплой Jetton контракта (Blueprint)
  День 3-4: Реальный TON mint в Go (tonutils-go)
  День 5:   Gas Pool таблица + авто-пополнение

Неделя 2:
  День 1:   Wallet initialization check
  День 2:   Withdrawal flow (3 варианта + UI)
  День 3:   FOMO счётчик "дней до халвинга"
  День 4:   Тестирование end-to-end на mainnet
  День 5:   Push уведомление 90% эпохи

Неделя 3+:
  TMA backend
  Пулы (Phase 3)
  Batch mint
```

---

## Переменные окружения (добавить)

```env
# TON (заполнить после деплоя контракта)
MOCK_TON=false
JETTON_MASTER_ADDRESS=EQ...   # адрес задеплоенного контракта
TON_RPC_URL=https://toncenter.com/api/v2/
TON_API_KEY=...               # получить на toncenter.com
ORACLE_PRIVATE_KEY=...        # seed phrase или hex key Oracle кошелька
ORACLE_WALLET_ADDRESS=UQC4u_AdYRtVgG8gi4AuNfwVOfIeTz6FNJDRQlswbvsNAtZo
ORACLE_MIN_BALANCE_TON=2.0    # порог авто-пополнения
```

---

## Открытые вопросы

| Вопрос | Кто отвечает | Блокирует |
|--------|-------------|-----------|
| Курс EXRA/USDT для вывода — фиксированный или с DEX? | Продукт | Withdrawal flow |
| USDT вывод — откуда Treasury берёт ликвидность изначально? | Продукт | USDT вывод |
| Seed phrase Oracle кошелька — как хранить безопасно на сервере? | Инфра | TON mint |
| FCM push — нужна регистрация Firebase? | Разработка | Push уведомления |

---

## Метрики успеха

| Метрика | Цель | Срок |
|---------|------|------|
| Первый реальный вывод | 1 транзакция | Неделя 1 |
| Успешных выводов | > 90% без ошибок | Неделя 2 |
| Среднее время вывода | < 30 секунд | Неделя 2 |
| Oracle баланс < 1 TON | 0 случаев | Постоянно |
| Воркеров сделавших вывод | > 50 | Месяц 1 |
