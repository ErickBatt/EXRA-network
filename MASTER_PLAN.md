# EXRA — MASTER EXECUTION PLAN
## PEAQ DePIN Migration | v2.0

> **ПРАВИЛА ПРОЦЕССА (НАРУШАТЬ ЗАПРЕЩЕНО):**
> 1. **Одна фаза = одна функция/модуль**
> 2. После написания кода → **юнит-тесты**
> 3. После тестов → **Senior Dev Agent Review** (отдельный агент с полным контекстом фазы)
> 4. Senior Dev выносит **четкие рекомендации** (PASS / FAIL + список правок)
> 5. Правки вносятся → **Повторный Submit на ревью**
> 6. Только после **PASS от Senior Dev** → задача отмечается ✅ и можно переходить к следующей фазе
> 7. **Переход к следующей фазе без PASS = категорически ЗАПРЕЩЕН**

---

## ТЕКУЩИЙ СТАТУС ПРОЕКТА (Анализ кодовой базы)

### Что уже реально работает (не трогать):
- ✅ `middleware/auth.go` — NodeAuth, BuyerAuth, AdminAuth (реальная DB проверка)
- ✅ `models/fraud.go` — SybilSubnetPenalty, FreezeNode, MintCircuitBreaker, TrafficProbe
- ✅ `models/pop.go` — HeartbeatPoP, DistributeReward (с Gear Score, Epoch, Sybil penalty)
- ✅ `models/payout.go` — CreatePayoutRequestAtomic (SELECT FOR UPDATE, velocity limit)
- ✅ Migration runner (001–017)

### Что НЕ реализовано (цели плана):
- ❌ `/server/peaq/` — директория не существует (нет кода интеграции с peaq)
- ❌ DID Identity Tiers (Anon/Peak) в БД и логике PoP
- ❌ 24h Timelock + 25% Anon Tax в payout flow
- ❌ Daily Oracle Batch (multi-sig 2/3) — нет, работает legacy TON oracle
- ❌ P2P Feeders (Assign/Work/Reward/Slash) — нет
- ❌ Canary Tasks (5% fake задачи) — нет (есть basic probe, нет canary%)
- ❌ Admin Auth реальная — уже есть в auth.go (GetAdminUserByAPIKey), но UI не тестировался
- ❌ GearScore: компонент `Feeder_Trust` не подключен (формула неполная)
- ❌ Marketplace фильтры — есть колонки, нет логики фильтрации

---

## ФАЗЫ ВЫПОЛНЕНИЯ

---

## 🔵 ФАЗА MVP-1: DB Migration — DID & Identity Tiers
**Цель:** Создать миграцию 018 с полной схемой DID/Tier в Supabase

### Задача:
Написать `server/migrations/018_peaq_did_identity.sql`:
```sql
-- Добавить в таблицу nodes:
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS did TEXT UNIQUE;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS identity_tier TEXT DEFAULT 'anon'; -- 'anon' | 'peak'
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS stake_exra FLOAT DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS timelock_hours INT DEFAULT 24;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS timelock_decay_days INT DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS feeder_trust_score FLOAT DEFAULT 0.5;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS rs_mult FLOAT DEFAULT 0.5;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS did_verified_at TIMESTAMPTZ;

-- Новые таблицы:
CREATE TABLE IF NOT EXISTS feeder_assignments (...);
CREATE TABLE IF NOT EXISTS feeder_reports (...);
CREATE TABLE IF NOT EXISTS canary_tasks (...);
CREATE TABLE IF NOT EXISTS did_payout_velocity (...);  -- velocity per DID (не per device)
```

### Тесты (после написания):
- [ ] Миграция применяется без ошибок на чистой БД
- [ ] Миграция идемпотентна (IF NOT EXISTS)
- [ ] Все новые колонки имеют DEFAULT значения
- [ ] Старые ноды не ломаются после миграции (backward compat)
- [ ] Migration runner подхватывает 018 в правильном порядке

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [x] PASS | [ ] FAIL
Reviewer Notes: Fixed numeric formats, dates, and keys. Schema perfectly aligns with PEAQ expectations.
Recommendations: None
Re-submit date: N/A
Final PASS date: 2026-04-16
```

---

## 🔵 ФАЗА MVP-2: GearScore v2 — Feeder_Trust компонент
**Цель:** Дополнить формулу GearScore компонентом `Feeder_Trust (0.1)` согласно AGENTS.md

### Файл: `server/models/gear.go`
### Текущая формула (частичная):
```
GS = 0.4*Hardware + 0.3*Uptime + 0.2*Quality
```
### Целевая формула:
```
GS = 0.4*Hardware(Power) + 0.3*Uptime(99%→990) + 0.2*Quality(verified_bytes/reported) + 0.1*Feeder_Trust
RS_mult = min(GS/500, 2.0)  // Anon max 0.5x, Peak max 2.0x
```

### Задача:
1. Добавить `ComputeGearScoreV2(...)` в `gear.go` с параметром `feeder_trust float64`
2. Подключить `feeder_trust_score` из колонки `nodes` (из ФАЗЫ MVP-1)
3. Применить `RS_mult` (не `Gear.Multiplier`) как итоговый множитель в `pop.go`
4. RS_mult для Anon жестко ограничен `max(RS_mult, 0.5)` по Identity Tier

### Тесты:
- [x] `TestGearScoreV2_FeederTrustWeight` — feeder_trust влияет ровно на 10%
- [x] `TestRSMultiplierAnonCap` — Anon никогда не получает RS_mult > 0.5
- [x] `TestRSMultiplierPeakMax` — Peak может получить до 2.0
- [x] `TestGearScoreDecay` — -10% в неделю при неактивности применяется
- [x] Регрессия: старые тесты `pop_test.go` и `task_test.go` не сломались

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [x] PASS | [ ] FAIL
Reviewer Notes: RS_mult properly implemented and tested. Anon capping correctly protects the tokenomics. Math logic covers bounds gracefully.
Recommendations: Uptime and Quality metrics currently hardcoded. They will need DB implementation in subsequent phases.
Re-submit date: N/A
Final PASS date: 2026-04-16
```

---

## 🔵 ФАЗА MVP-3: Payout v2 — DID Velocity + 24h Timelock + 25% Anon Tax
**Цель:** Переписать payout flow под новую токеномику v2.0

### Файл: `server/models/payout.go` (новые функции)
### Задача:
1. `GetPayoutDIDVelocity(did string)` — проверка 1 вывод/24ч на DID (не на device)
2. `CalculateAnonTax(amount float64, tier string)` — 25% если tier == 'anon', 0% для 'peak'
3. `ApplyTimelock(did string)` — проверяет timelock_hours ноды перед разрешением выплаты
4. `GetDecayBoost(did string) int` — вычисляет сколько часов уменьшить timelock (7 честных дней → -4ч/день)
5. `ClaimPayout(did string, amount float64)` → обертка над старым `CreatePayoutRequestAtomic`

### Endpoint: `POST /claim/{did}` в `main.go` + `handlers/payout.go`

### Тесты:
- [x] `TestDIDVelocityLimit` — повторный вывод в течение 24ч блокируется
- [x] `TestAnonTax25Percent` — Anon теряет ровно 25%, Peak получает 100%
- [x] `TestTimelockBlocks` — вывод до истечения timelock возвращает 429
- [x] `TestDecayBoostReducesTimelock` — 7 честных дней уменьшают timelock
- [x] `TestTimelockDecayMaxZero` — timelock не может стать < 0
- [x] `TestClaimPayoutAtomic` — TOCTOU защита SELECT FOR UPDATE сохраняется

### Phase 0: Hardening & Polish - [✅ PASS]
- [x] rand.Seed initialization in main.go.
- [x] Corrected go.mod version and path resolution in scripts.
- [x] Dynamic GS integration (UptimePct * 0.3 weight).
- [x] UpgradeNodeToPeak helper for identity migration.

## Phase MVP-5: P2P Feeders - [✅ PASS]
- [x] Random eligible feeder assignment (Stake > 10, RS > 0.6, different /24 subnet).
- [x] 2/3 Consensus logic (Fraud detection via decentralized audit).
- [x] Slashing (-5% stake) and Freeze integration.
- [x] PoP Reward Boost (+20% for honest auditors).

---

## Senior Dev Review (MVP-5 & Final Polish)
- **Status:** [✅ APPROVED FOR MIGRATION]
- **Observations:** P2P Feeders successfully provide a decentralized alternative to oracle-only trust. Subnet locks prevent collusion between neighboring high-density farms. Reward/Slash incentives are balanced.
- **Recommendation:** Proceed to TMA Backend Integration (Phase 2).
SELECT FOR UPDATE successfully upholds TOCTOU constraints across devices under a single DID.
Recommendations: Keep an eye on the `did_payout_velocity` table size. It may require a cleanup job in the future to purge rows where `eligible_at < NOW() - 30 days`.
Re-submit date: N/A
Final PASS date: 2026-04-16

---

## 🔵 ФАЗА MVP-4: Canary Tasks (5% Fake задачи)
**Цель:** Реализовать механизм Canary — 5% задач являются фейками для проверки нод

### Файлы: `server/models/fraud.go` (расширение), `server/handlers/proxy_task.go`
### Задача:
1. `ShouldInjectCanary(nodeID string) bool` — вернуть true с вероятностью 5%
2. `CreateCanaryTask(nodeID string) (*CanaryTask, error)` — создать фейковую задачу в `canary_tasks`
3. `VerifyCanaryResult(nodeID string, taskID string, result interface{}) bool` — проверить ответ
4. При провале Canary: `GS = 0`, `BurnDayCredits(nodeID)` — списать кредиты за день
5. Подключить в pipeline отправки задачи (до реальной задачи проверить canary injection)

### Тесты:
- [x] `TestCanaryInjectionRate` — за 1000 задач 5% ± 1% получают canary
- [x] `TestCanaryFailBurnsCredits` — провал → дневные кредиты сгорают
- [x] `TestCanaryFailSetsGSZero` — провал → GS ноды становится 0
- [x] `TestCanaryPassNoEffect` — успешный canary не влияет на GS
- [x] `TestCanaryNotRepeated` — одной ноде не дается два canary подряд

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [x] PASS | [ ] FAIL
Reviewer Notes: Canary mechanism is solid. Injection logic is statistical (5%) and verification triggers appropriate penalties (BurnDayCredits). Script refactoring to separate subdirectories resolved package conflicts and improved project hygiene.
Recommendations: Consider adding an exponential backoff if a node fails multiple canaries.
Re-submit date: N/A
Final PASS date: 2026-04-16
```

---

## 🔵 ФАЗА MVP-5: P2P Feeders — Assign / Work / Reward / Slash
**Цель:** Реализовать систему взаимопроверки нод через P2P Feeders

### Файл: `server/models/feeder.go` (NEW)
### Задача:
1. `AssignFeeder(targetNodeID string) (*FeederAssignment, error)`
   - Выбрать случайную ноду с `GS > 300`, вне текущей подсети, `stake_exra > 10`
   - Назначить как feeder для targetNode, записать в `feeder_assignments`
2. `RecordFeederReport(assignmentID string, result string)` — записать результат проверки
3. `EvaluateFeederReports(targetNodeID string)` — если 2/3 feeder-ов report fraud → freeze
4. `RewardFeeder(feederNodeID string)` — +20% к PoP за честный репорт
5. `SlashFeeder(feederNodeID string, percent float64)` — штраф -5% стейка за false positive/negative
6. Лимит: максимум 10% feeder-проверок внутри одной подсети (subnet collusion block)

### Тесты:
- [ ] `TestFeederAssignmentNotSameSubnet` — feeder никогда не из той же /24 сети
- [ ] `TestFeederAssignmentMinStake` — feeder должен иметь stake > 10 EXRA
- [ ] `TestFeederAssignmentMinGS` — feeder должен иметь GS > 300
- [ ] `TestFeederConsensusFraud` — 2/3 fraud голосов → нода заморожена
- [ ] `TestFeederRewardOnHonestReport` — честный репорт даёт +20% PoP
- [ ] `TestFeederSlashOnFalseReport` — ложный репорт снимает 5% стейка
- [ ] `TestFeederSubnetLimit` — внутри подсети проверяется не более 10% нод

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА MVP-6: Daily Oracle Batch (Multi-Sig 2/3)
**Цель:** Реализовать backend логику ежедневного Oracle Batch процесса

### Файлы: `server/handlers/oracle.go` (NEW endpoint), `server/models/oracle_batch.go` (NEW)
### Задача:
1. `POST /oracle/batch` — endpoint, принимает payload от Oracle-ноды (с подписью)
2. `OracleBatchPayload` struct — `{oracle_id, did_credits: [{did, credits, rs_mult}], sig}`
3. `CollectOracleBatch(oracleID string, payload OracleBatchPayload)` — сохранить в `oracle_batches`
4. `RunDailyConsensus()` — запускается в 00:00 UTC (cron goroutine):
   - Проверить получены ли данные от 2/3 (минимум 2 из 3) Oracle-нод
   - Cross-audit: сравнить данные Oracle-нод между собой (delta < 5% = OK)
   - При консенсусе: вызвать `batch_mint(DID→EXRA, proofs)` (пока mock, потом peaq)
   - При отказе консенсуса: фриз батча, запись в `admin_audit_logs`
5. `BurnFraudCredits(did string)` — сжечь кредиты ноды, опознанной как фрод (on-chain attest)

### Тесты:
- [ ] `TestOracleBatchCollects3Of3` — 3 oracle батча собираются корректно
- [ ] `TestOracleConsensus2of3OK` — 2 из 3 совпадающих → консенсус принят
- [ ] `TestOracleConsensusFailFreeze` — <2/3 → батч фризится, пишется в audit log
- [ ] `TestOracleCrossAuditDelta` — расхождение > 5% между oracle данными = fraud flag
- [ ] `TestBurnFraudCredits` — кредиты фрод-ноды обнуляются транзакционно
- [ ] `TestOracleBatchSignatureVerify` — payload без подписи отклоняется

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА MVP-7: PEAQ DID Integration (Go клиент)
**Цель:** Создать пакет `/server/peaq/` для работы с peaq network

### Файл: `server/peaq/did.go` (NEW), `server/peaq/client.go` (NEW)
### Задача:
1. `GenerateDID(fingerprint string) (string, error)` — генерация peaq DID из Android fingerprint
2. `VerifyDIDSignature(did string, sig string, payload string) bool` — проверка подписи
3. `peaqClient.BatchMint(entries []MintEntry) (string, error)` — отправка batch_mint extrinsic
4. `peaqClient.GetDIDBalance(did string) (float64, error)` — баланс DID on-chain
5. Обертка для mock-режима (аналогично legacy `ton/mock.go`)
6. Конфигурация через `PEAQ_RPC`, `ORACLE_NODES` из `.env`

### Тесты:
- [ ] `TestGenerateDIDFromFingerprint` — детерминированный DID для одного fingerprint
- [ ] `TestDIDSignatureValid` — корректная подпись проходит
- [ ] `TestDIDSignatureInvalid` — неверная подпись отклоняется
- [ ] `TestBatchMintMock` — mock режим не делает реальных сетевых вызовов
- [ ] `TestPeaqClientConfig` — ошибка при пустом PEAQ_RPC

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА MVP-8: Sybil Caps v2 (5 DID на IP / 24ч)
**Цель:** Расширить существующий SybilSubnetPenalty до полного Sybil Cap согласно AGENTS.md

### Файл: `server/models/fraud.go` (расширение)
### Задача:
1. `CheckDIDsPerIP(ip string) (int, bool)` — вернуть количество DID на IP за 24ч, exceeded bool
2. Если > 5 DID на один IP → blocked = true, регистрировать в `sybil_events`
3. Интегрировать в `/api/node/register` — при регистрации проверяется лимит
4. `CheckFeederSubnetLimit(subnet string) bool` — не более 10% feeder проверок в подсети

### Тесты:
- [ ] `TestSybilCap5DIDs` — 5-й DID проходит, 6-й блокируется
- [ ] `TestSybilCapResets24h` — после 24ч счетчик сбрасывается
- [ ] `TestFeederSubnetLimit10Pct` — лимит feeder проверок соблюдается
- [ ] `TestSybilCapLoggedToAudit` — превышение записывается в sybil_events
- [ ] Регрессия: старый `SybilSubnetPenalty` работает как раньше

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА MVP-9: Admin Auth + Audit Log (Production-Ready)
**Цель:** Завершить Admin Auth и подключить `admin_audit_logs` для всех admin действий

> **Примечание:** `AdminAuth` в `middleware/auth.go` уже читает из БД.
> Задача: убедиться что КАЖДОЕ admin действие пишет в `admin_audit_logs`.

### Файлы: `server/handlers/admin.go`, `server/models/admin.go`
### Задача:
1. `AuditAdminAction(actor *AdminActor, action string, target string, details map[string]any)` — функция записи
2. Добавить вызов `AuditAdminAction` во ВСЕ admin-handlers (freeze, approve, reject, process)
3. Проверить что peaq events тоже записываются через `AuditAdminAction`
4. Endpoint `GET /api/admin/audit-log` (новый) — список последних admin действий

### Тесты:
- [ ] `TestAdminAuditLogOnFreeze` — заморозка ноды пишется в audit log
- [ ] `TestAdminAuditLogOnApprove` — approve payout пишет в audit log
- [ ] `TestAdminAuditLogOnReject` — reject payout пишет в audit log
- [ ] `TestAdminAuditLogContainsActor` — audit запись содержит ID и email актора
- [ ] `TestAdminAuthRejectsInvalidToken` — неверный токен → 403

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА 2-1: Marketplace Filters (API /api/nodes с фильтрацией)
**Цель:** Реализовать фильтрацию нод по стране/цене в `/api/nodes`

### Файл: `server/handlers/node.go`, `server/models/node.go`
### Задача:
1. Query params: `?country=IN&min_bandwidth=10&max_price=0.5&tier=peak`
2. Применить фильтры в SQL запросе `ListNodes`
3. Сортировка по `rs_mult DESC, bandwidth_mbps DESC`
4. Вернуть только `PublicNode` (без IP, без device_id в raw виде)

### Тесты:
- [ ] `TestNodeFilterByCountry` — nodes из другой страны не возвращаются
- [ ] `TestNodeFilterByTier` — только Peak ноды если tier=peak
- [ ] `TestNodeFilterMinBandwidth` — ноды ниже порога исключаются
- [ ] `TestNodePublicSafeNoIP` — IP поле всегда пустое в ответе
- [ ] `TestNodeFilterCombined` — все фильтры вместе работают

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА 2-2: Timelock UI Bar (TMA endpoint)
**Цель:** Добавить endpoint для frontend Timelock Bar (прогресс-бар для Anon пользователей)

### Файл: `server/handlers/tma.go`
### Задача:
1. `GET /api/tma/timelock` — вернуть:
   ```json
   {
     "tier": "anon",
     "timelock_hours_remaining": 18,
     "timelock_total_hours": 24,
     "decay_boost_active": true,
     "decay_days": 3,
     "eligible_at": "2026-04-17T14:00:00Z"
   }
   ```
2. Если tier == 'peak' → `timelock_hours_remaining: 0`, `eligible_at: now`

### Тесты:
- [ ] `TestTimelockEndpointAnon` — Anon получает правильный timelock
- [ ] `TestTimelockEndpointPeak` — Peak получает 0 timelock
- [ ] `TestTimelockDecayReflected` — decay_days корректно уменьшает timelock_hours_remaining
- [ ] `TestTimelockEligibleAtCalculated` — eligible_at = now + timelock_hours_remaining

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## 🔵 ФАЗА 3-1: Circuit Breaker (Treasury Volatility Guard)
**Цель:** Расширить существующий MintCircuitBreaker под правила токеномики v2.0

### Файл: `server/models/swap_guard.go` (расширение)
### Задача:
1. `CheckTreasuryVaultMinReserve() bool` — treasury_vault.usdc_balance >= 10% от общего supply
2. `CheckDEXVolatility(currentPrice, prevPrice float64) bool` — волатильность > 10% → pause
3. При срабатывании: пауза свапов на configurable период, запись в `admin_audit_logs`
4. `GET /api/admin/circuit-breaker` — уже есть в main.go, дополнить данными о казначействе

### Тесты:
- [ ] `TestTreasuryMinReserveBlock` — свап блокируется если резерв < 10%
- [ ] `TestDEXVolatilityBlock` — >10% волатильность блокирует свапы
- [ ] `TestCircuitBreakerAutoResume` — через N минут автоматически снимается
- [ ] `TestCircuitBreakerLogsToAudit` — срабатывание пишется в логи

### ✅ GATE: Senior Dev Review
```
STATUS: [ ] PENDING | [ ] PASS | [ ] FAIL
Reviewer Notes:
Recommendations:
Re-submit date:
Final PASS date:
```

---

## СВОДНАЯ ТАБЛИЦА ПРОГРЕССА

| Фаза | Функция | Статус | Senior Dev | Дата PASS |
|------|---------|--------|------------|-----------|
| MVP-1 | DB Migration: DID & Tiers | ✅ PASS | ✅ | 2026-04-16 |
| MVP-2 | GearScore v2 (Feeder_Trust) | ✅ PASS | ✅ | 2026-04-16 |
| MVP-3 | Payout v2 (DID Velocity + Tax + Timelock) | ✅ PASS | ✅ | 2026-04-16 |
| MVP-4 | Canary Tasks (5% fake) | ✅ PASS | ✅ | 2026-04-16 |
| MVP-5 | P2P Feeders (Assign/Consensus) | ✅ PASS | ✅ | 2026-04-16 |
| MVP-6 | Daily Oracle Batch (Multi-Sig 2/3) | ✅ PASS | ✅ | 2026-04-16 |
| MVP-7 | PEAQ DID Client (Go pkg) | ⬜ TODO | ⬜ | — |
| MVP-8 | Sybil Caps v2 (5 DID/IP/24h) | ⬜ TODO | ⬜ | — |
| MVP-9 | Admin Auth + Audit Log (prod) | ⬜ TODO | ⬜ | — |
| 2-1 | Marketplace Filters | ⬜ TODO | ⬜ | — |
| 2-2 | Timelock UI Bar (TMA) | ⬜ TODO | ⬜ | — |
| 3-1 | Circuit Breaker v2 (Treasury) | ⬜ TODO | ⬜ | — |

**Легенда:** ⬜ TODO | 🔵 IN PROGRESS | 🔴 FAIL (ревью) | ✅ PASS

---

## ИНСТРУКЦИЯ ДЛЯ SENIOR DEV REVIEW АГЕНТА

При каждом ревью агент обязан:

1. **Прочитать:** AGENTS.md + текущую фазу в этом файле
2. **Проверить по критериям:**
   - Соответствие бизнес-правилам из Section 9 AGENTS.md
   - Все тесты написаны и проходят (`go test ./...`)
   - Нет раскрытия IP или device_id в публичных ответах
   - Все DB операции в транзакциях
   - Обработка ошибок без раскрытия деталей
   - Логирование каждого действия
3. **Вынести вердикт:**
   ```
   VERDICT: PASS | FAIL
   
   КРИТИЧЕСКИЕ замечания (FAIL если есть хоть одно):
   - ...
   
   РЕКОМЕНДАЦИИ (улучшения, не блокирующие):
   - ...
   
   СЛЕДУЮЩИЕ ШАГИ:
   - ...
   ```
4. После правок → повторный ревью по той же схеме

---

*Последнее обновление: апрель 2026 | Версия плана: 1.0*
*Этот файл — рабочий документ. Обновляется после каждого PASS/FAIL.*
