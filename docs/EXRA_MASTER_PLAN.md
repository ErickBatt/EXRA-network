# EXRA — Master Plan
> Единый документ по разработке. Обновлять при каждом изменении архитектуры.
> Последнее обновление: апрель 2026

---

## Статус проекта

| Компонент | Готовность | Статус |
|---|---|---|
| Go сервер (core) | 95% | ✅ база готова |
| Android APK | 65% | 🔄 в работе |
| Dashboard (Next.js) | 80% | ✅ работает |
| TMA (Telegram Mini App) | 100% | ✅ запущена |
| Desktop агент | 60% | 🔄 в работе |
| TON смарт-контракт | 50% | 🔄 деплой testnet |
| Dynamic Pricing | 40% | 🔄 в разработке |
| TON интеграция в Go | 100% | ✅ завершена |

---

## Блокчейн: TON (не Solana)

**Решение принято:** переходим на TON.

**Причины:**
- TMA уже готова, аудитория в Telegram
- Catchain 2.0 (апрель 2026) — sub-second транзакции, блоки каждые 400ms
- Следующий шаг TON — снижение комиссий в 6x
- 1 миллиард пользователей Telegram = наша аудитория
- TON Foundation Champion Grants — реальнее чем Solana Foundation для нас

---

## ЭТАП 1 — TON смарт-контракт (Jetton)

### Что делаем
Берём готовый официальный контракт, не пишем с нуля.

**Репозиторий:** `https://github.com/ton-blockchain/jetton-contract`

Это официальный reference implementation Jetton (TEP-74) от ton-blockchain.
NOTCoin уже задеплоил library cell на 100 лет — нам не надо деплоить библиотеку.

### Что добавляем сверху
Одно изменение — ограничить mint только нашим Oracle адресом:

```func
;; В jetton-minter.fc добавить проверку:
throw_unless(error::unauthorized, equal_slices(sender_address, oracle_address));
```

### Параметры токена
- **Название:** Exra
- **Тикер:** $EXRA
- **Decimals:** 9
- **Max supply:** 1,000,000,000 EXRA (hard cap)
- **Admin:** наш Oracle (Go сервер)
- **Premine:** 0

Экономическая спецификация и неизменяемые инварианты зафиксированы в `docs/PROTOCOL_ECONOMY_SPEC.md`.

### Шаги деплоя
```bash
# 1. Клонировать
git clone https://github.com/ton-blockchain/jetton-contract
cd jetton-contract

# 2. Установить зависимости
npm install

# 3. Добавить oracle_address в minter
# Редактировать contracts/jetton-minter.fc

# 4. Задеплоить на testnet
npx blueprint run --custom https://testnet.toncenter.com/api/v2/ \
  --custom-version v2 --custom-type testnet --custom-key <API_KEY>

# 5. Проверить wallet lib
npx blueprint run checkWalletLib

# 6. Задеплоить на mainnet
npx blueprint run
```

### Промт агенту
```
Прочитай AGENTS.md.

Задача: подготовить TON Jetton контракт для Exra.

1. Клонируй https://github.com/ton-blockchain/jetton-contract
2. В contracts/jetton-minter.fc добавь проверку oracle_address:
   - mint операция разрешена только с адреса ORACLE_ADDRESS
   - добавь хранение oracle_address в storage контракта
   - добавь op::change_oracle для смены oracle адреса (только admin)
3. Параметры токена: name="Exra", symbol="EXRA", decimals=9
4. Напиши деплой скрипт scripts/deployExra.ts с параметрами
5. Задеплой на testnet, покажи адрес контракта
```

---

## ЭТАП 2 — TON интеграция в Go сервер

### Что меняем
Только папка `server/solana/` → `server/ton/`

**Файлы которые трогаем:**
```
server/solana/client.go  →  server/ton/client.go
server/solana/mint.go    →  server/ton/mint.go
server/config/config.go  →  обновить переменные
```

**Удаляем:**
```
solana/Anchor.toml
solana/Cargo.toml
solana/programs/
```

### Новые переменные окружения
```env
# Было (Solana)
SOLANA_RPC_URL=https://api.devnet.solana.com
KILO_MINT=xxx
SOLANA_USD_PRICE=150

# Стало (TON)
TON_RPC_URL=https://toncenter.com/api/v2/
TON_API_KEY=your_toncenter_api_key
JETTON_MASTER_ADDRESS=EQxxxx...
ORACLE_PRIVATE_KEY=your_oracle_keypair
TON_WALLET_ADDRESS=your_oracle_wallet
```

### client.go — TON HTTP API v2
```go
// GET баланс кошелька
GET /getAddressBalance?address={address}

// GET информация о Jetton кошельке
GET /runGetMethod?address={jetton_master}&method=get_jetton_data

// Минт через internal message
POST /sendBoc — отправка подписанной транзакции
```

### mint.go — TON Jetton Transfer
```go
// Последовательность минта:
// 1. Получить Jetton wallet адрес ноды
// 2. Создать internal message op::mint
// 3. Подписать Oracle keypair
// 4. Отправить через TON API
// 5. Записать tx hash в oracle_mint_queue
```

### Промт агенту
```
Прочитай AGENTS.md и ARCHITECTURE.md.

Замени Solana интеграцию на TON в Go сервере:

1. Переименуй server/solana/ → server/ton/
2. server/ton/client.go:
   - Используй TON HTTP API v2 (toncenter.com/api/v2)
   - Функции: GetBalance(address), GetJettonWallet(master, owner), SendBoc(boc)
   - Читай TON_RPC_URL и TON_API_KEY из config
3. server/ton/mint.go:
   - Функция MintJettons(deviceID, amount, recipientWallet string)
   - Создаёт и подписывает TON internal message op::mint
   - Записывает результат в oracle_mint_queue
4. server/config/config.go:
   - SOLANA_RPC_URL → TON_RPC_URL
   - KILO_MINT → JETTON_MASTER_ADDRESS
   - Добавить TON_API_KEY, ORACLE_PRIVATE_KEY, TON_WALLET_ADDRESS
   - Убрать SOLANA_USD_PRICE
5. Удали папку solana/ (Anchor проект)
6. go build ./... должен проходить без ошибок

Обнови AGENTS.md — блокчейн теперь TON.
```

---

## ЭТАП 3 — Dynamic Pricing (рыночное ценообразование)

### Концепция
Воркер сам ставит цену за GB. Рынок определяет справедливую цену через конкуренцию.

**Режим ожидания (Heartbeat):**
- Нода онлайн → каждые 5 минут PoP награда
- Пассивный доход даже без активных сессий
- Создаёт плотность сети

**Режим маркетплейса (Active Selling):**
- Воркер ставит цену: "1 EXRA за 1 GB"
- Нода появляется в маркетплейсе с этой ценой
- Конкуренция → рынок сам регулирует цены по регионам

**Момент продажи:**
- Покупатель выбирает ноду → фиксируется locked_price_per_gb
- Reward сплит считается от locked_price, не от глобальной ставки
- Интерфейс "взрывается" анимацией когда пошёл трафик

**Auto-Price режим:**
- Система сама ставит среднюю цену по региону
- Удобно для новичков

### Изменения в БД
```sql
-- Миграция 010_dynamic_pricing.sql
ALTER TABLE nodes ADD COLUMN price_per_gb NUMERIC(10,4) DEFAULT 1.50;
ALTER TABLE nodes ADD COLUMN auto_price BOOLEAN DEFAULT true;
ALTER TABLE sessions ADD COLUMN locked_price_per_gb NUMERIC(10,4);

-- View для средней цены по стране
CREATE VIEW market_avg_price AS
SELECT country, AVG(price_per_gb) as avg_price, COUNT(*) as node_count
FROM nodes WHERE status = 'online'
GROUP BY country;
```

### Изменения в API
```
GET /api/nodes               — добавить price_per_gb в ответ
GET /api/nodes?sort=price    — сортировка cheapest first
GET /api/nodes/market-price?country=IN — средняя цена по стране
POST /api/session/start      — фиксировать locked_price_per_gb
```

### WS протокол
```json
// Нода → Сервер при регистрации
{
  "type": "register",
  "device_id": "xxx",
  "country": "IN",
  "device_type": "phone",
  "price_per_gb": 1.20,
  "auto_price": false
}
```

### Billing логика
```go
// Было:
cost := float64(bytesUsed) / 1e9 * config.RatePerGB

// Стало:
cost := float64(bytesUsed) / 1e9 * session.LockedPricePerGB
```

### Промт агенту
```
Прочитай AGENTS.md и ARCHITECTURE.md.

Внедри Dynamic Pricing для нод на маркетплейсе:

1. МИГРАЦИЯ server/migrations/010_dynamic_pricing.sql:
   ALTER TABLE nodes ADD COLUMN price_per_gb NUMERIC(10,4) DEFAULT 1.50;
   ALTER TABLE nodes ADD COLUMN auto_price BOOLEAN DEFAULT true;
   ALTER TABLE sessions ADD COLUMN locked_price_per_gb NUMERIC(10,4);
   CREATE VIEW market_avg_price AS SELECT country, AVG(price_per_gb) as avg_price
   FROM nodes WHERE status='online' GROUP BY country;

2. МОДЕЛЬ server/models/node.go:
   - Добавить PricePerGB float64 и AutoPrice bool в struct Node
   - Функция GetMarketAvgPrice(country string) float64
   - Обновить UpsertWSNode — принимать price_per_gb из WS register

3. API server/handlers/node.go:
   - GET /api/nodes — добавить price_per_gb в ответ
   - Query param ?sort=price_asc — сортировка по цене
   - GET /api/nodes/market-price?country=IN — средняя цена по стране

4. BILLING server/models/session.go:
   - При старте сессии записывать locked_price_per_gb = node.price_per_gb
   - FinalizeSession использовать locked_price_per_gb вместо глобального RATE_PER_GB
   - Если auto_price=true — использовать GetMarketAvgPrice(node.Country)

5. WS ПРОТОКОЛ server/hub/client.go:
   - В register сообщении принимать price_per_gb (float64)
   - Если не передано — auto_price=true

6. ANDROID android/.../ExraWsClient.kt:
   - В register добавить price_per_gb из SharedPreferences
   - Ключ "user_price_per_gb", default 0.0 (= auto)

go build ./... должен проходить без ошибок.
Обнови AGENTS.md — добавь секцию Dynamic Pricing.
```

---

## ЭТАП 4 — TON Oracle архитектура

### Концепция
Go сервер = единственный доверенный Oracle. Только он может минтить EXRA.

```
Нода шарит трафик
      ↓
Go сервер фиксирует байты
      ↓
PoP heartbeat срабатывает
      ↓
Oracle подписывает mint транзакцию
      ↓
TON смарт-контракт минтит EXRA на кошелёк ноды
      ↓
10% от buyer сессии → burn
```

### Oracle безопасность
- Oracle keypair хранить в переменных окружения (не в коде)
- В продакшне — HSM или Vault
- Ротация ключей через `op::change_oracle` в контракте

### Reconciliation
```sql
-- Таблица для сверки off-chain vs on-chain
oracle_mint_queue (
  id, device_id, amount_exra,
  status: pending|submitted|confirmed|failed,
  ton_tx_hash, created_at, confirmed_at
)
```

Reconciliation job каждые 10 минут:
- Берёт pending записи
- Проверяет статус tx на TON
- Обновляет статус в БД
- Алерт если tx failed

---

## ЭТАП 5 — TON Foundation Grant

### Почему мы подходим
- TMA уже готова (Telegram Mini App)
- Аудитория Индия/ЮВА — прямо в Telegram
- DePIN вертикаль — один из 5 приоритетов TON Champion Grants
- Fair launch, 0 premine — сильный нарратив
- Sub-second транзакции (Catchain 2.0) — критично для микроплатежей

### Стратегия (Champion Grants = без открытых заявок)
Нужно засветиться в экосистеме:

1. **Зарегистрироваться** на TON Builders Portal — ton.org/builders
2. **Зайти в TON Hub** для ЮВА (South & Southeast Asia)
3. **Участвовать в хакатоне** — там замечают и подключают к грантам
4. **TON VC Club** в Telegram — прямой выход на инвесторов
5. **TONcoin.Fund** — венчурный фонд, инвестирует в обмен на токены

### Параллельно
- **STON.fi grants** — до $10K для DeFi/DePIN, rolling basis
- **swap.coffee grants** — $2,500–$10,000

### Питч для TON (одна строка)
"Exra turns any Telegram user's phone into a node that earns $EXRA — the first DePIN bandwidth marketplace built natively for TON Mini Apps."

---

## Структура проекта (актуальная)

```
/exra
  /server          ← Go backend (ядро)
    /handlers      ← API endpoints
    /models        ← бизнес логика
    /hub           ← WebSocket manager
    /middleware    ← auth, rate limit, logging
    /migrations    ← 001-009 SQL (010 в планах)
    /ton           ← (заменить /solana) TON интеграция
    /metrics       ← Prometheus
  /android         ← Kotlin foreground service
  /desktop         ← Go desktop агент
  /dashboard       ← Next.js маркетплейс + TMA
    /app/tma       ← Telegram Mini App
    /app/marketplace
    /app/auth
  /docs            ← документация
  /solana          ← УДАЛИТЬ после этапа 2
```

---

## Порядок работ

| # | Этап | Задача | Срок | Статус |
|---|---|---|---|---|
| 1 | Смарт-контракт | Деплой Jetton на TON testnet | 1-2 дня | ✅ |
| 2 | Go сервер | Заменить solana/ → ton/ | 1 день | ✅ |
| 3 | Dynamic Pricing | Миграция + API + billing | 2-3 дня | 🔄 |
| 4 | Oracle | Подписание TON транзакций | 2 дня | 🔄 |
| 5 | Android | traffic events + цена | 1-2 нед | 🔄 |
| 6 | Маркетплейс | Dynamic prices в UI | 1 день | ❌ |
| 7 | Grant | TON Builders Portal + Hub | ongoing | 🔄 |
| 8 | Домен | Купить exra.network (฿261) | при деньгах | ❌ |

---

## Технический стек (финальный)

| Компонент | Технология |
|---|---|
| Блокчейн | TON (не Solana) |
| Токен | Jetton (TEP-74) |
| Смарт-контракт | FunC (готовый от ton-blockchain) |
| Go сервер | TON HTTP API v2 (toncenter.com) |
| Кошелёк юзера | TON Space / Tonkeeper |
| DEX | STON.fi / DeDust |
| TMA | Next.js + @twa-dev/sdk |
| Auth (маркетплейс) | Supabase Auth + Magic Link |
| База данных | Supabase (PostgreSQL) |
| Метрики | Prometheus + Grafana |
| Android | Kotlin + Foreground Service |

---

## Переменные окружения (итоговый .env)

```env
# Server
PORT=8080
PROXY_SECRET=xxx
NODE_SECRET=xxx
RATE_PER_GB=1.50

# Database
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_KEY=xxx

# TON (заменяет Solana)
TON_RPC_URL=https://toncenter.com/api/v2/
TON_API_KEY=xxx
JETTON_MASTER_ADDRESS=EQxxx
ORACLE_PRIVATE_KEY=xxx
TON_WALLET_ADDRESS=EQxxx

# Supabase Auth
NEXT_PUBLIC_SUPABASE_URL=xxx
NEXT_PUBLIC_SUPABASE_ANON_KEY=xxx
```

---

*Документ создан в апреле 2026. Обновлять при каждом завершённом этапе.*
