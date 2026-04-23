#!/usr/bin/env python3
"""
Диагностика проблемы TMA на production сервере.
Проверяет переменные окружения, логи и доступность endpoint'ов.
"""

import subprocess
import sys

SERVER_IP = "128.199.78.213"
SERVER_USER = "root"

def run_ssh(cmd):
    """Выполнить команду на удаленном сервере"""
    full_cmd = ["ssh", f"{SERVER_USER}@{SERVER_IP}", cmd]
    try:
        result = subprocess.run(full_cmd, capture_output=True, text=True, timeout=30)
        return result.stdout, result.stderr, result.returncode
    except Exception as e:
        return "", str(e), 1

def main():
    print("🔍 Диагностика TMA на production сервере...")
    print("=" * 60)
    
    # 1. Проверка переменных окружения
    print("\n1️⃣ Проверка переменных окружения TMA:")
    stdout, stderr, code = run_ssh("cd /root/exra/server && grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET|TMA_API_BASE' .env || echo 'Файл .env не найден'")
    if code == 0:
        lines = stdout.strip().split('\n')
        for line in lines:
            if 'TELEGRAM_BOT_TOKEN' in line:
                if line.strip().endswith('=') or '=' not in line:
                    print(f"   ❌ TELEGRAM_BOT_TOKEN не установлен!")
                else:
                    token = line.split('=', 1)[1]
                    print(f"   ✅ TELEGRAM_BOT_TOKEN установлен (длина: {len(token)})")
            elif 'TMA_SESSION_SECRET' in line:
                if line.strip().endswith('=') or '=' not in line:
                    print(f"   ❌ TMA_SESSION_SECRET не установлен!")
                else:
                    secret = line.split('=', 1)[1]
                    print(f"   ✅ TMA_SESSION_SECRET установлен (длина: {len(secret)})")
            elif 'TMA_API_BASE' in line:
                print(f"   ℹ️  {line}")
    else:
        print(f"   ❌ Ошибка: {stderr}")
    
    # 2. Проверка логов сервера
    print("\n2️⃣ Последние ошибки в логах сервера:")
    stdout, stderr, code = run_ssh("tail -100 /root/exra/server/server.log | grep -i 'tma\\|telegram\\|error\\|fatal' | tail -20")
    if stdout.strip():
        for line in stdout.strip().split('\n'):
            print(f"   📋 {line}")
    else:
        print("   ℹ️  Нет ошибок TMA в последних 100 строках")
    
    # 3. Проверка процесса сервера
    print("\n3️⃣ Статус процесса сервера:")
    stdout, stderr, code = run_ssh("ps aux | grep exra-server | grep -v grep")
    if stdout.strip():
        print(f"   ✅ Сервер запущен")
        print(f"   {stdout.strip()}")
    else:
        print(f"   ❌ Сервер не запущен!")
    
    # 4. Тест endpoint'а
    print("\n4️⃣ Тест TMA auth endpoint:")
    stdout, stderr, code = run_ssh("""curl -s -X POST http://localhost:8080/api/tma/auth \
        -H 'Content-Type: application/json' \
        -d '{"init_data":"test"}' \
        -w '\\nHTTP_CODE:%{http_code}'""")
    
    if stdout:
        lines = stdout.strip().split('\n')
        for line in lines:
            if 'HTTP_CODE:' in line:
                http_code = line.split(':')[1]
                if http_code == '401':
                    print(f"   ✅ Endpoint отвечает (401 - ожидаемо для невалидных данных)")
                elif http_code == '500':
                    print(f"   ❌ Endpoint возвращает 500 - внутренняя ошибка!")
                else:
                    print(f"   ℹ️  HTTP код: {http_code}")
            else:
                print(f"   📋 {line}")
    
    # 5. Проверка таблиц БД
    print("\n5️⃣ Проверка таблиц TMA в БД:")
    stdout, stderr, code = run_ssh("""cd /root/exra/server && grep SUPABASE_URL .env | head -1""")
    if stdout.strip():
        print(f"   ✅ SUPABASE_URL настроен")
    else:
        print(f"   ❌ SUPABASE_URL не найден")
    
    print("\n" + "=" * 60)
    print("✅ Диагностика завершена")
    print("\n💡 Рекомендации:")
    print("   1. Если TELEGRAM_BOT_TOKEN не установлен - добавить в .env")
    print("   2. Если TMA_SESSION_SECRET не установлен - сгенерировать и добавить")
    print("   3. Если сервер возвращает 500 - проверить логи выше")
    print("   4. После изменений - перезапустить сервер")

if __name__ == "__main__":
    main()
