#!/bin/bash
# Скрипт для проверки и запуска правильного бинарника сервера

SERVER_IP="103.6.168.174"
SERVER_USER="root"
SERVER_PASSWORD="Kukusa13Kro333"
SERVER_DIR="/root/exra/server"

echo "🔍 Поиск и запуск сервера на production..."
echo "============================================================"

# Функция для выполнения команд
run_remote() {
    sshpass -p "${SERVER_PASSWORD}" ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_IP}" "$@"
}

echo ""
echo "1️⃣ Проверка доступных бинарников..."
run_remote "ls -lh ${SERVER_DIR}/*.exe ${SERVER_DIR}/exra-server* ${SERVER_DIR}/server* 2>/dev/null || echo 'Бинарники не найдены'"

echo ""
echo "2️⃣ Проверка текущих процессов..."
run_remote "ps aux | grep -E 'exra|server' | grep -v grep || echo 'Нет запущенных процессов'"

echo ""
echo "3️⃣ Проверка последних логов..."
run_remote "tail -20 ${SERVER_DIR}/server.log 2>/dev/null || echo 'Лог не найден'"

echo ""
echo "4️⃣ Попытка запуска доступных бинарников..."

# Проверяем разные варианты имен
for binary in "exra-server-linux" "server" "exra.exe" "server.exe" "main" "exra"; do
    echo "   Проверяем ${binary}..."
    if run_remote "test -f ${SERVER_DIR}/${binary} && echo 'EXISTS'"; then
        echo "   ✅ Найден ${binary}, запускаем..."
        run_remote "cd ${SERVER_DIR} && chmod +x ${binary} && nohup ./${binary} > server.log 2>&1 &"
        sleep 3
        
        if run_remote "ps aux | grep -v grep | grep -q ${binary}"; then
            echo "   ✅ Сервер успешно запущен!"
            run_remote "ps aux | grep -v grep | grep ${binary}"
            
            # Тест endpoint
            echo ""
            echo "5️⃣ Тест TMA endpoint..."
            sleep 2
            RESPONSE=$(run_remote "curl -s -X POST http://localhost:8080/api/tma/auth -H 'Content-Type: application/json' -d '{\"init_data\":\"test\"}' -w '\nHTTP_CODE:%{http_code}'")
            echo "$RESPONSE"
            
            exit 0
        fi
    fi
done

echo ""
echo "❌ Не удалось найти рабочий бинарник сервера"
echo "   Возможно нужно скомпилировать и загрузить новый"
