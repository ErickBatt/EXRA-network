# TMA Issue Diagnosis & Fix

**Дата:** 21 апреля 2026  
**Проблема:** TMA (Telegram Mini App) возвращает 500 Internal Server Error при попытке авторизации  
**URL:** https://app.exra.space/next-tma/auth  
**Статус:** 🔴 КРИТИЧЕСКИЙ - блокирует использование TMA

---

## 🔍 Диагностика

### Симптомы
1. ❌ В браузере: "Failed to load account"
2. ❌ В консоли: `POST https://app.exra.space/next-tma/auth 500 (Internal Server Error)`
3. ❌ Ошибки в консоли Chrome:
   - `AbortError: Failed to execute 'keys' on 'Cache'`
   - `Duplicate key: TranslationToneNeutral`

### Анализ кода

#### ✅ Код сервера корректен
- `server/handlers/tma.go` - реализация правильная
- `server/middleware/tma_auth.go` - middleware работает корректно
- `server/main.go:152` - роутинг настроен:
  ```go
  r.HandleFunc("/api/tma/auth", tmaAuthLimit(handlers.TmaAuth)).Methods("POST")
  ```

#### ❌ Вероятная причина: Отсутствие переменных окружения

Согласно `server/middleware/tma_auth.go:72-75`:
```go
botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
if botToken == "" {
    return nil, errors.New("TELEGRAM_BOT_TOKEN not configured")
}
```

**Если `TELEGRAM_BOT_TOKEN` не установлен → сервер возвращает 500 ошибку.**

---

## 🔧 Решение

### Необходимые переменные окружения

В файле `/root/exra/server/.env` на production сервере должны быть:

```bash
# TMA (Telegram Mini App) - ОБЯЗАТЕЛЬНО
TELEGRAM_BOT_TOKEN=8702349787:AAENQVRmAsCHUDeJyG8zqGsnZeKFA5fLoTE
TMA_SESSION_SECRET=J5fOPYVo3QNzCjVXT6lRmgD4REi5mjNan4X8CC8ocmw
TMA_API_BASE=https://app.exra.space
GO_ENV=production
```

### Шаги исправления

#### Вариант 1: Автоматический (если SSH доступен)

```bash
# Из корня проекта
bash deploy/fix-tma-production.sh
```

или

```bash
python deploy/fix-tma-quick.py
```

#### Вариант 2: Ручной (через SSH)

```bash
# 1. Подключиться к серверу
ssh root@103.6.168.174

# 2. Перейти в директорию сервера
cd /root/exra/server

# 3. Проверить текущие переменные
grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET' .env

# 4. Добавить недостающие переменные
# Если TELEGRAM_BOT_TOKEN отсутствует:
echo 'TELEGRAM_BOT_TOKEN=8702349787:AAENQVRmAsCHUDeJyG8zqGsnZeKFA5fLoTE' >> .env

# Если TMA_SESSION_SECRET отсутствует:
echo 'TMA_SESSION_SECRET=J5fOPYVo3QNzCjVXT6lRmgD4REi5mjNan4X8CC8ocmw' >> .env

# Если TMA_API_BASE отсутствует:
echo 'TMA_API_BASE=https://app.exra.space' >> .env

# Если GO_ENV отсутствует:
echo 'GO_ENV=production' >> .env

# 5. Перезапустить сервер
pkill -f exra-server
nohup ./exra-server-linux > server.log 2>&1 &

# 6. Проверить запуск
ps aux | grep exra-server

# 7. Проверить логи
tail -f server.log

# 8. Тест endpoint
curl -X POST http://localhost:8080/api/tma/auth \
  -H 'Content-Type: application/json' \
  -d '{"init_data":"test"}'
# Ожидаемый результат: 401 (невалидные данные) или 400 (malformed)
# НЕ должно быть 500!
```

#### Вариант 3: Через веб-панель хостинга

Если SSH недоступен:
1. Зайти в панель управления DigitalOcean/хостинга
2. Открыть консоль сервера
3. Выполнить команды из Варианта 2

---

## ✅ Проверка исправления

### 1. Проверка переменных окружения
```bash
ssh root@103.6.168.174 'cd /root/exra/server && grep -E "TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET" .env'
```

Должно вывести (токены скрыты):
```
TELEGRAM_BOT_TOKEN=8702349787:***
TMA_SESSION_SECRET=J5fOPYVo3QNzCjVXT6lRmgD4REi5mjNan4X8CC8ocmw
```

### 2. Проверка процесса сервера
```bash
ssh root@103.6.168.174 'ps aux | grep exra-server'
```

Должен показать запущенный процесс.

### 3. Проверка endpoint
```bash
curl -X POST https://app.exra.space/api/tma/auth \
  -H 'Content-Type: application/json' \
  -d '{"init_data":"test"}'
```

Ожидаемый ответ (401 или 400, НЕ 500):
```json
{"error":"invalid telegram initData"}
```

### 4. Проверка в браузере
1. Открыть https://app.exra.space
2. Telegram Mini App должен загрузиться без ошибки "Failed to load account"
3. В консоли не должно быть 500 ошибок

---

## 📋 Дополнительные проверки

### Проверка логов сервера
```bash
ssh root@103.6.168.174 'tail -100 /root/exra/server/server.log | grep -i "tma\|telegram\|error"'
```

### Проверка NGINX конфигурации
```bash
ssh root@103.6.168.174 'nginx -t'
```

Убедиться, что `/api/tma/*` проксируется на backend:
```nginx
location /api/ {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection 'upgrade';
    proxy_set_header Host $host;
    proxy_cache_bypass $http_upgrade;
}
```

---

## 🚨 Известные проблемы

### SSH Connection Timeout
**Проблема:** `ssh: connect to host 103.6.168.174 port 22: Connection timed out`

**Возможные причины:**
1. Firewall блокирует SSH (порт 22)
2. Сервер недоступен
3. SSH ключи не настроены
4. VPN/прокси блокирует подключение

**Решение:**
1. Проверить доступность сервера: `ping 103.6.168.174`
2. Проверить SSH ключи: `ssh -v root@103.6.168.174`
3. Использовать веб-консоль хостинга
4. Проверить firewall правила

### Cookie SameSite Issues (iOS)
Согласно AGENTS.md §15 TMA Security P1:
- Текущая `SameSite=Strict` может не работать в Telegram iOS WebView
- **Fix:** Перевести на `SameSite=Lax` + явная проверка `Origin`/`Referer`

---

## 📚 Связанные документы

- `AGENTS.md` §15 - TMA Security (P1 риски)
- `server/handlers/tma.go` - TMA endpoints
- `server/middleware/tma_auth.go` - TMA authentication
- `docs/TMA_MARKETPLACE_HARDENING_PLAN.md` - Security hardening

---

## 🎯 Следующие шаги после исправления

1. ✅ Проверить TMA в браузере
2. ✅ Проверить авторизацию через Telegram
3. ✅ Проверить linking устройств
4. ⚠️ Закрыть P1 риски из AGENTS.md §15:
   - JWT revocation
   - SameSite=Lax для iOS
   - Fingerprint binding
   - Ownership + DID lookup optimization

---

**Статус:** 🟡 Ожидает исправления на production сервере  
**Приоритет:** P0 (блокирует MVP launch)  
**Ответственный:** DevOps / Backend team
