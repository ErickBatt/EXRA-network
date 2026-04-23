#!/usr/bin/env python3
"""
Быстрый деплой landing (Next.js) на production сервер
"""
import paramiko
import os

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("="*70)
print("ДЕПЛОЙ LANDING (MARKETPLACE FIX)")
print("="*70)

# Путь к landing на сервере (нужно найти)
print("\n1. Поиск директории landing на сервере...")
stdin, stdout, stderr = ssh.exec_command("find /root -name 'landing' -type d 2>/dev/null | head -5")
landing_paths = stdout.read().decode().strip().split('\n')
print(f"Найдено путей: {landing_paths}")

# Проверяем PM2
print("\n2. Проверка PM2 процесса exra-landing...")
stdin, stdout, stderr = ssh.exec_command("pm2 describe exra-landing | grep 'script path'")
pm2_path = stdout.read().decode().strip()
print(f"PM2 path: {pm2_path}")

# Извлекаем рабочую директорию из script path
stdin, stdout, stderr = ssh.exec_command("pm2 jlist | grep -A 20 'exra-landing' | grep 'pm_cwd' | cut -d'\"' -f4")
cwd = stdout.read().decode().strip()
print(f"Working directory: {cwd}")

if not cwd or cwd == "N/A" or not cwd.startswith("/"):
    print("⚠️  Не удалось найти рабочую директорию, пробуем стандартные пути...")
    possible_paths = ["/var/www/landing", "/root/exra/landing", "/root/landing", "/opt/exra/landing"]
    for path in possible_paths:
        stdin, stdout, stderr = ssh.exec_command(f"test -d {path} && echo 'exists'")
        if stdout.read().decode().strip() == "exists":
            cwd = path
            print(f"✓ Найдено: {cwd}")
            break

if not cwd:
    print("❌ Не удалось найти директорию landing!")
    ssh.close()
    exit(1)

print(f"\n3. Загрузка файлов в {cwd}...")

# Создаем SFTP соединение
sftp = ssh.open_sftp()

# Загружаем новую страницу marketplace
local_marketplace = "landing/app/marketplace/page.tsx"
remote_marketplace = f"{cwd}/app/marketplace/page.tsx"

print(f"   Создание директории {cwd}/app/marketplace...")
stdin, stdout, stderr = ssh.exec_command(f"mkdir -p {cwd}/app/marketplace")
stdout.read()

print(f"   Загрузка {local_marketplace}...")
sftp.put(local_marketplace, remote_marketplace)
print(f"   ✓ {remote_marketplace}")

# Загружаем обновленный navbar
local_navbar = "landing/components/navbar.tsx"
remote_navbar = f"{cwd}/components/navbar.tsx"
print(f"   Загрузка {local_navbar}...")
sftp.put(local_navbar, remote_navbar)
print(f"   ✓ {remote_navbar}")

sftp.close()

print("\n4. Перезапуск PM2 процесса...")
stdin, stdout, stderr = ssh.exec_command("pm2 restart exra-landing")
print(stdout.read().decode())

import time
time.sleep(3)

print("\n5. Проверка статуса...")
stdin, stdout, stderr = ssh.exec_command("pm2 describe exra-landing | grep -E 'status|uptime|restarts'")
print(stdout.read().decode())

print("\n6. Проверка доступности...")
stdin, stdout, stderr = ssh.exec_command("curl -s -o /dev/null -w '%{http_code}' https://exra.space/marketplace")
status = stdout.read().decode().strip()
print(f"https://exra.space/marketplace → HTTP {status}")

if status == "200":
    print("\n✅ ДЕПЛОЙ УСПЕШЕН!")
    print("\n🔗 Проверь в браузере:")
    print("   https://exra.space/marketplace")
    print("   (должен редиректить на https://app.exra.space)")
else:
    print(f"\n⚠️  Неожиданный статус: {status}")
    print("\nПроверка логов PM2:")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 10 --nostream")
    print(stdout.read().decode())

ssh.close()
