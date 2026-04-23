#!/usr/bin/env python3
"""
Установка недостающих React зависимостей
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("="*70)
print("УСТАНОВКА REACT ЗАВИСИМОСТЕЙ")
print("="*70)

print("\n1. Проверка package.json...")
stdin, stdout, stderr = ssh.exec_command("cat /var/www/landing/package.json")
print(stdout.read().decode())

print("\n2. Установка react и react-dom...")
commands = """
cd /var/www/landing
npm install react react-dom
"""
stdin, stdout, stderr = ssh.exec_command(commands)
while not stdout.channel.exit_status_ready():
    if stdout.channel.recv_ready():
        print(stdout.channel.recv(1024).decode(), end='')
print(stdout.read().decode())

print("\n3. Пересборка...")
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
else:
    print("\n✅ Сборка успешна!")

print("\n4. Перезапуск PM2...")
stdin, stdout, stderr = ssh.exec_command("pm2 restart exra-landing")
print(stdout.read().decode())

import time
time.sleep(5)

print("\n5. Тест...")
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
else:
    print("\n⚠️  Логи...")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 10 --nostream")
    print(stdout.read().decode())

ssh.close()
