#!/bin/bash
# Скрипт для компиляции и деплоя сервера на production

set -e

SERVER_IP="103.6.168.174"
SERVER_USER="root"
SERVER_PASSWORD="Kukusa13Kro333"
SERVER_DIR="/root/exra/server"

echo "🔨 Компиляция и деплой сервера на production..."
echo "============================================================"

# 1. Компиляция сервера для Linux
echo ""
echo "1️⃣ Компиляция Go сервера для Linux..."
cd server
GOOS=linux GOARCH=amd64 go build -o exra-server-linux .
if [ ! -f "exra-server-linux" ]; then
    echo "❌ Ошибка компиляции!"
    exit 1
fi
echo "✅ Сервер скомпилирован: $(ls -lh exra-server-linux)"

# 2. Загрузка на сервер
echo ""
echo "2️⃣ Загрузка на production сервер..."
sshpass -p "${SERVER_PASSWORD}" scp -o StrictHostKeyChecking=no \
    exra-server-linux "${SERVER_USER}@${SERVER_IP}:${SERVER_DIR}/"
echo "✅ Бинарник загружен"

# 3. Установка прав и запуск
echo ""
echo "3️⃣ Запуск сервера..."
sshpass -p "${SERVER_PASSWORD}" ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_IP}" << 'EOF'
cd /root/exra/server
chmod +x exra-server-linux
pkill -f exra-server || true
sleep 2
nohup ./exra-server-linux > server.log 2>&1 &
sleep 3
ps aux | grep -v grep | grep exra-server
EOF

# 4. Тест endpoint
echo ""
echo "4️⃣ Тест TMA endpoint..."
sleep 2
RESPONSE=$(sshpass -p "${SERVER_PASSWORD}" ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_IP}" \
    "curl -s -X POST http://localhost:8080/api/tma/auth -H 'Content-Type: application/json' -d '{\"init_data\":\"test\"}' -w '\nHTTP_CODE:%{http_code}'")

echo "$RESPONSE"

if echo "$RESPONSE" | grep -q "HTTP_CODE:401"; then
    echo ""
    echo "✅ Сервер работает корректно!"
    echo "✅ TMA endpoint возвращает 401 (ожидаемо)"
    echo ""
    echo "🎉 Проверьте TMA в браузере: https://app.exra.space"
else
    echo ""
    echo "⚠️  Проверьте логи сервера"
fi

cd ..
