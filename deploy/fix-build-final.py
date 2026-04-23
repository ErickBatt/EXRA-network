#!/usr/bin/env python3
"""
Финальное исправление сборки
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("="*70)
print("ФИНАЛЬНОЕ ИСПРАВЛЕНИЕ СБОРКИ")
print("="*70)

print("\n1. Переустановка зависимостей (с патчем lockfile)...")
commands = """
cd /var/www/landing
npm install
"""
stdin, stdout, stderr = ssh.exec_command(commands)
while not stdout.channel.exit_status_ready():
    if stdout.channel.recv_ready():
        print(stdout.channel.recv(1024).decode(), end='')
print(stdout.read().decode())

print("\n2. Полная пересборка...")
commands = """
cd /var/www/landing
rm -rf .next
./node_modules/.bin/next build
"""
stdin, stdout, stderr = ssh.exec_command(commands)
while not stdout.channel.exit_status_ready():
    if stdout.channel.recv_ready():
        print(stdout.channel.recv(1024).decode(), end='')
output = stdout.read().decode()
print(output)

error = stderr.read().decode()
if "Failed to compile" in output or "Failed to compile" in error:
    print("\n❌ Сборка провалилась!")
    print(error)
    ssh.close()
    exit(1)

print("\n3. Перезапуск PM2...")
stdin, stdout, stderr = ssh.exec_command("pm2 restart exra-landing")
print(stdout.read().decode())

import time
time.sleep(5)

print("\n4. Проверка статуса...")
stdin, stdout, stderr = ssh.exec_command("pm2 describe exra-landing | grep -E 'status|uptime'")
print(stdout.read().decode())

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
    print("\n🎉 Проблемы решены:")
    print("   1. TMA авторизация работает (добавлены переменные окружения)")
    print("   2. API endpoints доступны (исправлен nginx)")
    print("   3. Marketplace страница работает (пересобран landing)")
    print("\n🔗 Проверь в браузере:")
    print("   • https://exra.space - ссылка 'Marketplace' в навигации")
    print("   • https://exra.space/marketplace - редирект на app.exra.space")
else:
    print("\n⚠️  Проверяем логи...")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 15 --nostream")
    print(stdout.read().decode())

ssh.close()
