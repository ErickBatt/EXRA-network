# EXRA Reality Audit — 2026-04-23

Этот документ фиксирует фактическое состояние проекта по коду, тестам и git-веткам.
Whitepaper описывает целевую продуктовую картину. Этот файл описывает, что реально есть в репозитории на 23 апреля 2026.

## 1. Источники

- Whitepaper: `EXRA White Paper_ Sovereign DePIN Infrastructure.pdf`
- Инженерный ledger: `AGENTS.md`
- Проверка факта реализации: код, тесты, git-ветки, текущее рабочее дерево

## 2. Короткий вывод

- Whitepaper нельзя читать как инженерную истину для `main`: фразы вроде `Technical readiness: 100%` и `Marketplace audit v2.4.1 passed` описывают целевое состояние, а не текущий код.
- Старый `AGENTS.md` тоже врал, но в другую сторону: значимая часть audit-fix'ов уже реально есть в `main`, хотя документ продолжал держать их открытыми.
- Сверка веток не выявила “скрытую супер-ветку”, которая доводит проект до 100%. Большинство ценных hardening-коммитов либо уже absorbed в `main`, либо дублируются текущим рабочим деревом.
- На 2026-04-23 проект выглядит как:
  - примерно `75-80%` по объёму реализованного функционала
  - примерно `55-60%` по production-readiness
  - примерно `60-65%` по production-readiness, если закрыть TMA P1, buyer-side traffic cross-check и live-chain peaq compatibility

## 3. Что проверено

### Git-ветки

Сравнение велось относительно `main`:

| Ветка | main_only | branch_only | Вывод |
|---|---:|---:|---|
| `claude/brave-lewin-2497be` | 20 | 0 | Уже поглощена `main` |
| `claude/bold-carson-9596fe` | 22 | 1 | Отдельного feature-gap не видно |
| `claude/happy-einstein-60e24d` | 22 | 7 | В основном hardening/tests, содержательно уже размазаны по `main` и рабочему дереву |
| `claude/agitated-burnell-563cba` | 19 | 4 | Прод/деплой/SSR фиксы, без “100% готовности” |
| `claude/modest-goldstine-1188c0` | 19 | 3 | В основном UI/landing |
| `claude/zen-snyder-98d395` | 22 | 1 | Аудит-репорт и тесты |

### Cherry-pick проверка

В отдельной integration-ветке были перепроверены high-priority коммиты:

- `78600ca`
- `687ecc0`
- `913bc84`
- `9328b36`
- `097503b`
- `9f4fa3e`
- `aa53eb4`
- `8c8b17e`
- `0a525f5`

Практический результат: содержательные изменения этих коммитов в основном уже присутствуют в `main` или в текущем рабочем дереве. Чистые cherry-pick'и либо оказывались пустыми, либо конфликтовали только на уже обновлённых местах.

## 4. Тестовая картина

После обновления stale-тестов под текущие API и текущую реализацию:

- `server/handlers/tma_auth_test.go`
- `server/models/payout_test.go`
- `server/handlers/matcher_concurrency_test.go`
- `server/hub/client_trust_test.go`
- `server/models/canary_test.go`

получен следующий результат:

```text
cd server
go test -count=1 ./...

ok   exra/gateway
ok   exra/handlers
ok   exra/hub
ok   exra/middleware
ok   exra/models
ok   exra/tests
```

Это важно: ранее “краснота” была не столько доказательством незакрытых багов, сколько смесью из реальных исторических находок и устаревших proof-of-bug тестов, которые уже спорили с текущим кодом.

## 5. Что реально уже сделано

Подтверждено кодом в `main`:

- `A1` `gateway/sessions.go::Stitch` закрыт через `sync.Once` + `done`
- `A3` `gateway/bridge.go` ставит `SetReadDeadline(90s)`
- `B2` matcher уже не на старой forensic-формуле
- `B3` hold баланса стоит до выдачи Gateway JWT
- `C1` cleanup для hub идёт через TTL-per-entry, а не bulk reset
- `C2` Redis subscription loops умеют reconnect
- `D1` старый hardcoded gateway fallback ушёл из matcher flow
- `D3` WebSocket origin guard завязан на `WS_ALLOWED_ORIGINS`
- `E1` `feeder_report` требует DID подпись
- `E2` universal canary hash убран: `CreateCanaryTask` генерирует per-task hash
- `F1` oracle proposal signature проверяется до consensus
- `G3` gateway byte accounting и billing settlement реализованы

## 6. Что остаётся открытым

### Confirmed open

1. `TMA_SESSION_SECRET` fallback dev-secret
   - Код: `server/middleware/tma_auth.go`
   - Если env пуст, сервер падает не везде, а местами уходит на dev secret

2. `SameSite=Strict` для TMA cookie
   - Код: `server/middleware/tma_auth.go`
   - Риск: Telegram iOS WebView

3. Нет TMA session revocation
   - Код: `server/middleware/tma_auth.go`
   - JWT живёт 24h без blacklist/session revoke

4. Fingerprint binding в TMA approval flow не доведён до жёсткого enforcement

5. DID uniqueness для stake flow не выглядит жёстко зафиксированной схемой

6. `E3` не закрыт полностью
   - `hub/client.go` теперь clamp'ит worker-reported bytes
   - но buyer-side counter cross-check в `models/session.go::FinalizeSession` отсутствует

7. `E4` IPv6 farm handling
   - `server/models/fraud.go::toSubnet24` работает только по IPv4

8. Go ↔ peaq runtime bridge выглядит несовместимым с pallet API
   - pallet ждёт `batch_mint(H256, Vec<Claim>, Vec<(u8,[u8;64])>)`
   - клиент шлёт `[]byte + []RewardEntry + []OracleSignature`
   - клиент вызывает `PalletExra.update_reputations`, а в pallet есть `update_stats`
   - mock E2E не доказывают реальную chain compatibility

### Partially closed

1. `B1` matcher double-booking
   - В production path есть `AtomicClaimNode(...)`
   - Нужен честный Redis-backed integration test

2. Canary anti-fraud
   - Universal literal уже убран
   - Но следующий шаг всё ещё нужен: привязать canary к реальному proxy challenge / feeder-side verification, а не только к случайному серверному hash

## 7. Компонентные цифры

| Компонент | Реализация | Production-ready | Комментарий |
|---|---:|---:|---|
| Go server core + gateway | 88% | 68% | Маршруты, auth, payout, compute, gateway и большая часть audit-fix'ов уже есть; открыты TMA P1, E3 final hardening и peaq bridge |
| Marketplace / Dashboard | 75% | 60% | Buyer/TMA flows есть, но prod-hardening и часть deploy wiring ещё не финализированы |
| TMA backend | 75% | 55% | Cookie/session/ownership уже есть, P1 по secret, revocation, fingerprint, DID uniqueness остаются |
| Android node | 70% | 55% | DID, WS, heartbeat, tunnel, compute result есть; prod-proof и полный anti-fraud path не закрыты |
| peaq pallet (Rust) | 85% | 70% | Содержательный pallet и tests есть |
| Go ↔ peaq bridge | 45% | 25% | Есть интерфейс и mock-тесты, но сигнатуры вызовов выглядят несовместимыми с pallet |
| Desktop agent | 30% | 15% | Есть skeleton клиента, heartbeat, tunnel/compute sim |

## 8. Практический вывод по веткам

- Переносить в `main` “всё подряд из feature-веток” не нужно: это не даёт нового скачка готовности.
- Реальная польза от branch-audit в другом:
  - убрать ложные документы и ложные красные тесты
  - зафиксировать честный инженерный статус
  - добить реальные незакрытые места

## 9. Следующие шаги

1. Закрыть TMA P1 из секции 15 `AGENTS.md`
2. Добавить buyer-side traffic cross-check в `models/session.go::FinalizeSession`
3. Сверить `server/peaq/peaq_client.go` против актуального pallet metadata и dispatch signatures
4. Добавить Redis-backed integration test на `AtomicClaimNode(...)`
5. Только после этого снова поднимать продуктовые формулировки про “audit passed” и “100% readiness”
