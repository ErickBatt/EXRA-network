#!/usr/bin/env python3
"""
Компиляция Go сервера для Linux
"""

import subprocess
import os
import sys

def main():
    print("🔨 Компиляция Go сервера для Linux...")
    print("=" * 60)
    
    # Переходим в директорию server
    os.chdir("server")
    
    # Устанавливаем переменные окружения для кросс-компиляции
    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = "amd64"
    env["CGO_ENABLED"] = "0"
    
    print("\n1️⃣ Компиляция...")
    print("   GOOS=linux GOARCH=amd64 CGO_ENABLED=0")
    
    # Компилируем
    result = subprocess.run(
        ["go", "build", "-o", "exra-server-linux", "."],
        env=env,
        capture_output=True,
        text=True
    )
    
    if result.returncode != 0:
        print(f"\n❌ Ошибка компиляции:")
        print(result.stderr)
        return 1
    
    # Проверяем результат
    if os.path.exists("exra-server-linux"):
        size_mb = os.path.getsize("exra-server-linux") / (1024 * 1024)
        print(f"\n✅ Сервер скомпилирован: exra-server-linux ({size_mb:.2f} MB)")
        
        print("\n" + "=" * 60)
        print("📋 Следующие шаги:")
        print("\n1. Загрузите файл на сервер:")
        print("   scp server/exra-server-linux root@103.6.168.174:/root/exra/server/")
        print("   Пароль: Kukusa13Kro333")
        print("\n2. На сервере выполните:")
        print("   cd /root/exra/server")
        print("   chmod +x exra-server-linux")
        print("   pkill -f exra-server || true")
        print("   nohup ./exra-server-linux > server.log 2>&1 &")
        print("\n3. Проверьте TMA: https://app.exra.space")
        print("=" * 60)
        
        return 0
    else:
        print("\n❌ Файл exra-server-linux не создан!")
        return 1

if __name__ == "__main__":
    sys.exit(main())
