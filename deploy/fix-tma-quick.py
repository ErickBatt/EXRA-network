#!/usr/bin/env python3
"""
Быстрое исправление TMA на production.
Проверяет переменные окружения и перезапускает сервер.
"""

import subprocess
import sys
import time

SERVER_IP = "103.6.168.174"
SERVER_USER = "root"
SERVER_DIR = "/root/exra/server"

# Токен из локального .env
TELEGRAM_BOT_TOKEN = "8702349787:AAENQVRmAsCHUDeJyG8zqGsnZeKFA5fLoTE"
TMA_SESSION_SECRET = "J5fOPYVo3QNzCjVXT6lRmgD4REi5mjNan4X8CC8ocmw"

def run_ssh(cmd):
    """Выполнить команду на удаленном сервере"""
    full_cmd = ["ssh", "-o", "ConnectTimeout=10", f"{SERVER_USER}@{SERVER_IP}", cmd]
    try:
        result = subprocess.run(full_cmd, capture_output=True, text=True, timeout=30)
        return result.stdout, result.stderr, result.returncode
    except subprocess.TimeoutExpired:
        return "", "SSH timeout", 1
    except Exception as e:
        return "", str(e), 1

def main():
    print("🔧 Быстрое исправление TMA на production...")
    print("=" * 60)
    
    # 1. Проверка подключения
    print("\n1️⃣ Проверка подключения...")
    stdout, stderr, code = run_ssh("echo OK")
    if code != 0:
        print(f"❌ Не удается подключиться: {stderr}")
        print("   Возможные причины:")
        print("   - Сервер недоступен")
        print("   - SSH ключи не настроены")
        print("   - Firewall блокирует подключение")
        return 1
    print("✅ Подключение установлено")
    
    # 2. Проверка текущих переменных
    print("\n2️⃣ Проверка текущих TMA переменных...")
    stdout, stderr, code = run_ssh(f"cd {SERVER_DIR} && grep -E 'TELEGRAM_BOT_TOKEN|TMA_SESSION_SECRET' .env || echo 'NOT_FOUND'")
    
    has_bot_token = False
    has_session_secret = False
    
    if stdout:
        for line in stdout.split('\n'):
            if 'TELEGRAM_BOT_TOKEN=' in line and not line.strip().endswith('='):
                has_bot_token = True
                print(f"   ✅ TELEGRAM_BOT_TOKEN найден")
            elif 'TMA_SESSION_SECRET=' in line and not line.strip().endswith('='):
                has_session_secret = True
                print(f"   ✅ TMA_SESSION_SECRET найден")
    
    # 3. Добавление недостающих переменных
    needs_restart = False
    
    if not has_bot_token:
        print("\n3️⃣ Добавление TELEGRAM_BOT_TOKEN...")
        cmd = f"cd {SERVER_DIR} && sed -i '/^TELEGRAM_BOT_TOKEN=/d' .env && echo 'TELEGRAM_BOT_TOKEN={TELEGRAM_BOT_TOKEN}' >> .env"
        stdout, stderr, code = run_ssh(cmd)
        if code == 0:
            print("   ✅ TELEGRAM_BOT_TOKEN добавлен")
            needs_restart = True
        else:
            print(f"   ❌ Ошибка: {stderr}")
    
    if not has_session_secret:
        print("\n4️⃣ Добавление TMA_SESSION_SECRET...")
        cmd = f"cd {SERVER_DIR} && sed -i '/^TMA_SESSION_SECRET=/d' .env && echo 'TMA_SESSION_SECRET={TMA_SESSION_SECRET}' >> .env"
        stdout, stderr, code = run_ssh(cmd)
        if code == 0:
            print("   ✅ TMA_SESSION_SECRET добавлен")
            needs_restart = True
        else:
            print(f"   ❌ Ошибка: {stderr}")
    
    # 5. Добавление TMA_API_BASE если отсутствует
    print("\n5️⃣ Проверка TMA_API_BASE...")
    stdout, stderr, code = run_ssh(f"cd {SERVER_DIR} && grep -q '^TMA_API_BASE=' .env && echo 'EXISTS' || echo 'NOT_FOUND'")
    if 'NOT_FOUND' in stdout:
        cmd = f"cd {SERVER_DIR} && echo 'TMA_API_BASE=https://app.exra.space' >> .env"
        run_ssh(cmd)
        print("   ✅ TMA_API_BASE добавлен")
        needs_restart = True
    else:
        print("   ✅ TMA_API_BASE уже установлен")
    
    # 6. Проверка GO_ENV
    print("\n6️⃣ Проверка GO_ENV...")
    stdout, stderr, code = run_ssh(f"cd {SERVER_DIR} && grep -q '^GO_ENV=' .env && echo 'EXISTS' || echo 'NOT_FOUND'")
    if 'NOT_FOUND' in stdout:
        cmd = f"cd {SERVER_DIR} && echo 'GO_ENV=production' >> .env"
        run_ssh(cmd)
        print("   ✅ GO_ENV=production добавлен")
        needs_restart = True
    
    # 7. Перезапуск сервера если нужно
    if needs_restart or True:  # Всегда перезапускаем для применения изменений
        print("\n7️⃣ Перезапуск сервера...")
        
        # Останавливаем
        print("   Останавливаем текущий процесс...")
        run_ssh("pkill -f exra-server || true")
        time.sleep(2)
        
        # Запускаем
        print("   Запускаем сервер...")
        run_ssh(f"cd {SERVER_DIR} && nohup ./exra-server-linux > server.log 2>&1 &")
        time.sleep(3)
        
        # Проверяем
        stdout, stderr, code = run_ssh("ps aux | grep -v grep | grep exra-server")
        if stdout.strip():
            print("   ✅ Сервер успешно запущен")
        else:
            print("   ❌ Сервер не запустился!")
            print("\n   Последние строки лога:")
            stdout, stderr, code = run_ssh(f"tail -30 {SERVER_DIR}/server.log")
            print(stdout)
            return 1
    
    # 8. Тест endpoint
    print("\n8️⃣ Тест TMA auth endpoint...")
    stdout, stderr, code = run_ssh("""curl -s -X POST http://localhost:8080/api/tma/auth -H 'Content-Type: application/json' -d '{"init_data":"test"}' -w '\\nHTTP_CODE:%{http_code}'""")
    
    if 'HTTP_CODE:401' in stdout:
        print("   ✅ Endpoint работает (401 - ожидаемо для невалидных данных)")
    elif 'HTTP_CODE:500' in stdout:
        print("   ❌ Endpoint возвращает 500!")
        print("\n   Последние ошибки:")
        stdout, stderr, code = run_ssh(f"tail -50 {SERVER_DIR}/server.log | grep -i error")
        print(stdout)
    else:
        print(f"   ℹ️  Ответ: {stdout}")
    
    print("\n" + "=" * 60)
    print("✅ Исправление завершено!")
    print("\n📋 Проверьте TMA в браузере: https://app.exra.space")
    print("📋 Логи сервера: ssh root@103.6.168.174 'tail -f /root/exra/server/server.log'")
    
    return 0

if __name__ == "__main__":
    sys.exit(main())
