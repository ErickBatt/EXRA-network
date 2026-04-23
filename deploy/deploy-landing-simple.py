#!/usr/bin/env python3
"""
Простой деплой landing - загрузка файлов и пересборка на сервере
"""
import paramiko
import tarfile
import os

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

print("="*70)
print("ДЕПЛОЙ LANDING (MARKETPLACE)")
print("="*70)

# 1. Создаем архив с исходниками
print("\n1. Создание архива...")
archive_path = "deploy/dist/landing-src.tar.gz"
os.makedirs("deploy/dist", exist_ok=True)

with tarfile.open(archive_path, "w:gz") as tar:
    tar.add("landing/app", arcname="app")
    tar.add("landing/components", arcname="components")

print(f"✓ Архив создан: {archive_path}")

# 2. Подключаемся к серверу
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

# 3. Загружаем архив
print("\n2. Загрузка на сервер...")
sftp = ssh.open_sftp()
remote_archive = "/tmp/landing-src.tar.gz"
sftp.put(archive_path, remote_archive)
sftp.close()
print(f"✓ Загружено: {remote_archive}")

# 4. Распаковываем и пересобираем
print("\n3. Распаковка и пересборка на сервере...")
commands = f"""
cd /var/www/landing
echo "=== Создание бэкапа ==="
tar -czf /tmp/landing-backup-$(date +%s).tar.gz app components .next 2>/dev/null || true
echo "=== Распаковка новых файлов ==="
tar -xzf {remote_archive} -C /var/www/landing/
echo "=== Проверка node_modules ==="
if [ ! -d "node_modules" ]; then
  echo "node_modules не найдены, устанавливаем..."
  npm install
fi
echo "=== Пересборка Next.js ==="
npx next build
echo "=== Очистка ==="
rm {remote_archive}
"""

stdin, stdout, stderr = ssh.exec_command(commands)
# Читаем вывод в реальном времени
import select
while not stdout.channel.exit_status_ready():
    if stdout.channel.recv_ready():
        print(stdout.channel.recv(1024).decode(), end='')

# Читаем остаток
print(stdout.read().decode())
error = stderr.read().decode()
if error and "warning" not in error.lower():
    print("Stderr:", error)

# 5. Перезапуск PM2
print("\n4. Перезапуск PM2...")
stdin, stdout, stderr = ssh.exec_command("pm2 restart exra-landing")
print(stdout.read().decode())

import time
time.sleep(3)

# 6. Проверка
print("\n5. Проверка статуса...")
stdin, stdout, stderr = ssh.exec_command("pm2 describe exra-landing | grep -E 'status|uptime|restarts'")
print(stdout.read().decode())

print("\n6. Тест доступности...")
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
    print("✅ ДЕПЛОЙ УСПЕШЕН!")
    print("="*70)
    print("\n🔗 Проверь в браузере:")
    print("   1. https://exra.space")
    print("      → В навигации должна быть ссылка 'Marketplace'")
    print("   2. https://exra.space/marketplace")
    print("      → Должен показать страницу с редиректом на app.exra.space")
else:
    print("\n⚠️  Есть проблемы, проверяем логи...")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 15 --nostream")
    print(stdout.read().decode())

ssh.close()
