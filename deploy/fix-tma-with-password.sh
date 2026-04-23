#!/bin/bash
# Скрипт для исправления TMA на production с использованием sshpass
# Использует пароль для подключения

set -e

SERVER_IP="103.6.168.174"
SERVER_USER="root"
SERVER_DIR="/root/exra/server"
SERVER_PASSWORD="Kukusa13Kro333"

echo "🔧 Исправление TMA на production сервере..."
echo "============================================================"

# Функция для выполнения команд на сервере с паролем
run_remote() {
    sshpass -p "${SERVER_PASSWORD}" ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_IP}" "$@"
}

# 1. Проверка подключения
echo ""
echo "1️⃣ Проверка подключения к серверу..."
if ! run_remote "echo 'OK'" > /dev/null 2>&1; then
    echo "❌ Не удается подключиться к серверу ${SERVER_IP}"
    echo "   Установите sshpass: apt-get install sshpass (Linux) или brew install sshpass (Mac)"
    exit 1
fi
echo "✅ Подключение установлено"

# 2. Проверка директории и создание .env если нужно
echo ""
echo "2️⃣ Проверка директории сервера..."
run_remote "cd ${SERVER_DIR} && pwd"

echo ""
echo "3️⃣ Проверка файла .env..."
if ! run_remote "test -f ${SERVER_DIR}/.env && echo 'EXISTS'"; then
    echo "⚠️  Файл .env не существует, создаём..."
    run_remote "touch ${SERVER_DIR}/.env"
    echo "✅ Файл .env создан"
fi

# 4. Проверка текущих переменных
echo ""
echo "4️⃣ Проверка текущих TMA переменных..."
run_remote "cd ${SERVER_DIR} && grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET|TMA_API_BASE|GO_ENV' .env || echo 'Переменные не найдены'"

# 5. Добавление TELEGRAM_BOT_TOKEN
echo ""
echo "5️⃣ Добавление TELEGRAM_BOT_TOKEN..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TELEGRAM_BOT_TOKEN=' .env"; then
    echo "   ℹ️  TELEGRAM_BOT_TOKEN уже существует, обновляем..."
    run_remote "cd ${SERVER_DIR} && sed -i '/^TELEGRAM_BOT_TOKEN=/d' .env"
fi
run_remote "cd ${SERVER_DIR} && echo 'TELEGRAM_BOT_TOKEN=8702349787:AAENQVRmAsCHUDeJyG8zqGsnZeKFA5fLoTE' >> .env"
echo "✅ TELEGRAM_BOT_TOKEN добавлен"

# 6. Добавление TMA_SESSION_SECRET
echo ""
echo "6️⃣ Добавление TMA_SESSION_SECRET..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TMA_SESSION_SECRET=' .env"; then
    echo "   ℹ️  TMA_SESSION_SECRET уже существует, обновляем..."
    run_remote "cd ${SERVER_DIR} && sed -i '/^TMA_SESSION_SECRET=/d' .env"
fi
run_remote "cd ${SERVER_DIR} && echo 'TMA_SESSION_SECRET=J5fOPYVo3QNzCjVXT6lRmgD4REi5mjNan4X8CC8ocmw' >> .env"
echo "✅ TMA_SESSION_SECRET добавлен"

# 7. Добавление TMA_API_BASE
echo ""
echo "7️⃣ Добавление TMA_API_BASE..."
if run_remote "cd ${SERVER_DIR} && grep -q '^TMA_API_BASE=' .env"; then
    echo "   ℹ️  TMA_API_BASE уже существует, обновляем..."
    run_remote "cd ${SERVER_DIR} && sed -i '/^TMA_API_BASE=/d' .env"
fi
run_remote "cd ${SERVER_DIR} && echo 'TMA_API_BASE=https://app.exra.space' >> .env"
echo "✅ TMA_API_BASE добавлен"

# 8. Добавление GO_ENV
echo ""
echo "8️⃣ Добавление GO_ENV..."
if run_remote "cd ${SERVER_DIR} && grep -q '^GO_ENV=' .env"; then
    echo "   ℹ️  GO_ENV уже существует"
else
    run_remote "cd ${SERVER_DIR} && echo 'GO_ENV=production' >> .env"
    echo "✅ GO_ENV=production добавлен"
fi

# 9. Показываем итоговую конфигурацию
echo ""
echo "9️⃣ Итоговая конфигурация TMA:"
run_remote "cd ${SERVER_DIR} && grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET|TMA_API_BASE|GO_ENV' .env | sed 's/\(TELEGRAM_BOT_TOKEN=\).*/\1***HIDDEN***/; s/\(TMA_SESSION_SECRET=\).*/\1***HIDDEN***/'"

# 10. Проверка наличия бинарника
echo ""
echo "🔟 Проверка бинарника сервера..."
if run_remote "test -f ${SERVER_DIR}/exra-server-linux && echo 'EXISTS'"; then
    echo "✅ exra-server-linux найден"
else
    echo "❌ exra-server-linux не найден!"
    echo "   Проверьте путь к бинарнику"
    exit 1
fi

# 11. Перезапуск сервера
echo ""
echo "1️⃣1️⃣ Перезапуск сервера..."
echo "   Останавливаем текущий процесс..."
run_remote "pkill -f exra-server || true"
sleep 2

echo "   Запускаем сервер..."
run_remote "cd ${SERVER_DIR} && nohup ./exra-server-linux > server.log 2>&1 &"
sleep 3

# 12. Проверка запуска
echo ""
echo "1️⃣2️⃣ Проверка запуска сервера..."
if run_remote "ps aux | grep -v grep | grep -q exra-server"; then
    echo "✅ Сервер успешно запущен"
    run_remote "ps aux | grep -v grep | grep exra-server"
else
    echo "❌ Сервер не запустился, проверяем логи..."
    run_remote "tail -50 ${SERVER_DIR}/server.log"
    exit 1
fi

# 13. Тест TMA endpoint
echo ""
echo "1️⃣3️⃣ Тест TMA auth endpoint..."
sleep 2
RESPONSE=$(run_remote "curl -s -X POST http://localhost:8080/api/tma/auth -H 'Content-Type: application/json' -d '{\"init_data\":\"test\"}' -w '\nHTTP_CODE:%{http_code}'")
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)

if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "400" ]; then
    echo "✅ Endpoint работает корректно (${HTTP_CODE} для невалидных данных)"
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
echo "      sshpass -p '${SERVER_PASSWORD}' ssh ${SERVER_USER}@${SERVER_IP} 'tail -100 ${SERVER_DIR}/server.log'"
echo "   3. После проверки ОБЯЗАТЕЛЬНО смените пароль сервера!"
