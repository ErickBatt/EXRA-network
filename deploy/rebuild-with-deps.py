#!/usr/bin/env python3
"""
Установка зависимостей и пересборка landing
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("="*70)
print("УСТАНОВКА ЗАВИСИМОСТЕЙ И ПЕРЕСБОРКА")
print("="*70)

print("\n1. Установка зависимостей...")
stdin, stdout, stderr = ssh.exec_command("cd /var/www/landing && npm install")
while not stdout.channel.exit_status_ready():
    if stdout.channel.recv_ready():
        print(stdout.channel.recv(1024).decode(), end='')
print(stdout.read().decode())

print("\n2. Пересборка с использованием node_modules/.bin/next...")
commands = """
cd /var/www/landing
export PATH=$PATH:/var/www/landing/node_modules/.bin
echo "=== Сборка Next.js ==="
./node_modules/.bin/next build
"""

stdin, stdout, stderr = ssh.exec_command(commands)
while not stdout.channel.exit_status_ready():
    if stdout.channel.recv_ready():
        print(stdout.channel.recv(1024).decode(), end='')
print(stdout.read().decode())

error = stderr.read().decode()
if error and "warning" not in error.lower():
    print("Stderr:", error)

print("\n3. Перезапуск PM2...")
stdin, stdout, stderr = ssh.exec_command("pm2 restart exra-landing")
print(stdout.read().decode())

import time
time.sleep(3)

print("\n4. Проверка...")
stdin, stdout, stderr = ssh.exec_command("ls -la /var/www/landing/.next/server/app/marketplace/ 2>/dev/null")
marketplace_built = stdout.read().decode()
if marketplace_built.strip():
    print("✅ Marketplace собран:")
    print(marketplace_built)
else:
    print("❌ Marketplace НЕ собран")

print("\n5. Тест доступности...")
tests = [
    ("Landing", "curl -s -o /dev/null -w '%{http_code}' https://exra.space/"),
    ("Marketplace", "curl -s -o /dev/null -w '%{http_code}' https://exra.space/marketplace"),
]

all_ok = True
for name, cmd in tests:
    stdin, stdout, stderr = ssh.exec_command(cmd)
    status = stdout.read().decode().strip()
    if status == "200":
        print(f"✅ {name}: HTTP {status}")
    else:
        print(f"❌ {name}: HTTP {status}")
        all_ok = False

if all_ok:
    print("\n" + "="*70)
    print("✅ ВСЁ РАБОТАЕТ!")
    print("="*70)
    print("\n🔗 Проверь в браузере:")
    print("   https://exra.space/marketplace")
else:
    print("\n⚠️  Проверяем логи...")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 10 --nostream")
    print(stdout.read().decode())

ssh.close()
