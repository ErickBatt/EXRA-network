#!/bin/bash
# Скрипт для исправления TMA на production сервере
# Проверяет и добавляет недостающие переменные окружения

set -e

SERVER_IP="103.6.168.174"
SERVER_USER="root"
SERVER_DIR="/root/exra/server"

echo "🔧 Исправление TMA на production сервере..."
echo "============================================================"

# Функция для выполнения команд на сервере
run_remote() {
    ssh -o ConnectTimeout=10 "${SERVER_USER}@${SERVER_IP}" "$@"
}

# 1. Проверка подключения
echo ""
echo "1️⃣ Проверка подключения к серверу..."
if ! run_remote "echo 'OK'" > /dev/null 2>&1; then
    echo "❌ Не удается подключиться к серверу ${SERVER_IP}"
    echo "   Проверьте SSH ключи и доступность сервера"
    exit 1
fi
echo "✅ Подключение установлено"

# 2. Проверка текущих переменных
echo ""
echo "2️⃣ Проверка текущих TMA переменных..."
run_remote "cd ${SERVER_DIR} && grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET|TMA_API_BASE|GO_ENV' .env || true"

# 3. Проверка наличия TELEGRAM_BOT_TOKEN
echo ""
echo "3️⃣ Проверка TELEGRAM_BOT_TOKEN..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TELEGRAM_BOT_TOKEN=.\+' .env"; then
    echo "✅ TELEGRAM_BOT_TOKEN уже установлен"
else
    echo "❌ TELEGRAM_BOT_TOKEN не установлен или пустой"
    echo "   Добавляем из локального .env..."
    
    # Читаем токен из локального .env
    LOCAL_TOKEN=$(grep '^TELEGRAM_BOT_TOKEN=' server/.env | cut -d'=' -f2-)
    if [ -n "$LOCAL_TOKEN" ]; then
        run_remote "cd ${SERVER_DIR} && sed -i '/^TELEGRAM_BOT_TOKEN=/d' .env && echo 'TELEGRAM_BOT_TOKEN=${LOCAL_TOKEN}' >> .env"
        echo "✅ TELEGRAM_BOT_TOKEN добавлен"
    else
        echo "⚠️  TELEGRAM_BOT_TOKEN не найден в локальном .env"
    fi
fi

# 4. Проверка TMA_SESSION_SECRET
echo ""
echo "4️⃣ Проверка TMA_SESSION_SECRET..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TMA_SESSION_SECRET=.\+' .env"; then
    echo "✅ TMA_SESSION_SECRET уже установлен"
else
    echo "❌ TMA_SESSION_SECRET не установлен или пустой"
    echo "   Генерируем новый секрет..."
    
    # Генерируем случайный секрет
    NEW_SECRET=$(python3 -c "import secrets; print(secrets.token_urlsafe(32))")
    run_remote "cd ${SERVER_DIR} && sed -i '/^TMA_SESSION_SECRET=/d' .env && echo 'TMA_SESSION_SECRET=${NEW_SECRET}' >> .env"
    echo "✅ TMA_SESSION_SECRET сгенерирован и добавлен"
fi

# 5. Проверка TMA_API_BASE
echo ""
echo "5️⃣ Проверка TMA_API_BASE..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TMA_API_BASE=' .env"; then
    echo "✅ TMA_API_BASE уже установлен"
else
    echo "⚠️  TMA_API_BASE не установлен, добавляем..."
    run_remote "cd ${SERVER_DIR} && echo 'TMA_API_BASE=https://app.exra.space' >> .env"
    echo "✅ TMA_API_BASE добавлен"
fi

# 6. Проверка GO_ENV
echo ""
echo "6️⃣ Проверка GO_ENV..."
if run_remote "cd ${SERVER_DIR} && grep -q '^GO_ENV=' .env"; then
    CURRENT_ENV=$(run_remote "cd ${SERVER_DIR} && grep '^GO_ENV=' .env | cut -d'=' -f2-")
    echo "ℹ️  GO_ENV=${CURRENT_ENV}"
else
    echo "⚠️  GO_ENV не установлен, добавляем production..."
    run_remote "cd ${SERVER_DIR} && echo 'GO_ENV=production' >> .env"
    echo "✅ GO_ENV=production добавлен"
fi

# 7. Проверка TMA_COOKIE_INSECURE (должен быть 0 или отсутствовать в production)
echo ""
echo "7️⃣ Проверка TMA_COOKIE_INSECURE..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TMA_COOKIE_INSECURE=1' .env"; then
    echo "⚠️  TMA_COOKIE_INSECURE=1 найден - это небезопасно для production!"
    echo "   Удаляем или устанавливаем в 0..."
    run_remote "cd ${SERVER_DIR} && sed -i '/^TMA_COOKIE_INSECURE=/d' .env"
    echo "✅ TMA_COOKIE_INSECURE удален (cookies будут Secure)"
else
    echo "✅ TMA_COOKIE_INSECURE не установлен (cookies будут Secure)"
fi

# 8. Показываем итоговую конфигурацию
echo ""
echo "8️⃣ Итоговая конфигурация TMA:"
run_remote "cd ${SERVER_DIR} && grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET|TMA_API_BASE|GO_ENV' .env | sed 's/\(TELEGRAM_BOT_TOKEN=\).*/\1***HIDDEN***/; s/\(TMA_SESSION_SECRET=\).*/\1***HIDDEN***/'"

# 9. Перезапуск сервера
echo ""
echo "9️⃣ Перезапуск сервера..."
echo "   Останавливаем текущий процесс..."
run_remote "pkill -f exra-server || true"
sleep 2

echo "   Запускаем сервер..."
run_remote "cd ${SERVER_DIR} && nohup ./exra-server-linux > server.log 2>&1 &"
sleep 3

# 10. Проверка запуска
echo ""
echo "🔟 Проверка запуска сервера..."
if run_remote "ps aux | grep -v grep | grep -q exra-server"; then
    echo "✅ Сервер успешно запущен"
    run_remote "ps aux | grep -v grep | grep exra-server"
else
    echo "❌ Сервер не запустился, проверяем логи..."
    run_remote "tail -50 ${SERVER_DIR}/server.log"
    exit 1
fi

# 11. Тест TMA endpoint
echo ""
echo "1️⃣1️⃣ Тест TMA auth endpoint..."
RESPONSE=$(run_remote "curl -s -X POST http://localhost:8080/api/tma/auth -H 'Content-Type: application/json' -d '{\"init_data\":\"test\"}' -w '\nHTTP_CODE:%{http_code}'")
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)

if [ "$HTTP_CODE" = "401" ]; then
    echo "✅ Endpoint работает корректно (401 для невалидных данных)"
elif [ "$HTTP_CODE" = "500" ]; then
    echo "❌ Endpoint возвращает 500 - проверьте логи:"
    run_remote "tail -30 ${SERVER_DIR}/server.log | grep -i error"
else
    echo "ℹ️  HTTP код: $HTTP_CODE"
    echo "$RESPONSE"
fi

echo ""
echo "============================================================"
echo "✅ Исправление завершено!"
echo ""
echo "📋 Следующие шаги:"
echo "   1. Проверьте TMA в браузере: https://app.exra.space"
echo "   2. Если проблема сохраняется, проверьте логи:"
echo "      ssh ${SERVER_USER}@${SERVER_IP} 'tail -100 ${SERVER_DIR}/server.log'"
echo "   3. Проверьте NGINX конфигурацию для /api/tma/* маршрутов"
