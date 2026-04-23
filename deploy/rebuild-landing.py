#!/usr/bin/env python3
"""
Пересборка и деплой landing с новыми изменениями
"""
import paramiko
import tarfile
import os
import tempfile

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

print("="*70)
print("ПЕРЕСБОРКА И ДЕПЛОЙ LANDING")
print("="*70)

# 1. Создаем архив с исходниками landing
print("\n1. Создание архива landing...")
archive_path = "deploy/dist/landing-update.tar.gz"
os.makedirs("deploy/dist", exist_ok=True)

with tarfile.open(archive_path, "w:gz") as tar:
    tar.add("landing/app", arcname="app")
    tar.add("landing/components", arcname="components")
    tar.add("landing/lib", arcname="lib")
    tar.add("landing/package.json", arcname="package.json")
    tar.add("landing/next.config.mjs", arcname="next.config.mjs")
    tar.add("landing/tsconfig.json", arcname="tsconfig.json")
    tar.add("landing/tailwind.config.ts", arcname="tailwind.config.ts")
    tar.add("landing/postcss.config.mjs", arcname="postcss.config.mjs")

print(f"✓ Архив создан: {archive_path}")

# 2. Подключаемся к серверу
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

# 3. Загружаем архив
print("\n2. Загрузка на сервер...")
sftp = ssh.open_sftp()
remote_archive = "/tmp/landing-update.tar.gz"
sftp.put(archive_path, remote_archive)
sftp.close()
print(f"✓ Загружено: {remote_archive}")

# 4. Распаковываем и пересобираем
print("\n3. Распаковка и пересборка...")
commands = f"""
cd /var/www/landing
echo "Создание бэкапа..."
tar -czf /tmp/landing-backup-$(date +%s).tar.gz app components 2>/dev/null || true
echo "Распаковка новых файлов..."
tar -xzf {remote_archive} -C /var/www/landing/
echo "Пересборка Next.js..."
npm run build
echo "Очистка..."
rm {remote_archive}
"""

stdin, stdout, stderr = ssh.exec_command(commands)
output = stdout.read().decode()
error = stderr.read().decode()
print(output)
if error:
    print("Stderr:", error)

# 5. Перезапуск PM2
print("\n4. Перезапуск PM2...")
stdin, stdout, stderr = ssh.exec_command("pm2 restart exra-landing")
print(stdout.read().decode())

import time
time.sleep(3)

# 6. Проверка
print("\n5. Проверка...")
stdin, stdout, stderr = ssh.exec_command("pm2 describe exra-landing | grep -E 'status|uptime'")
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
    print("\n✅ ДЕПЛОЙ УСПЕШЕН!")
    print("\n🔗 Проверь в браузере:")
    print("   https://exra.space/marketplace")
    print("   (должен показать страницу с редиректом на app.exra.space)")
else:
    print("\n⚠️  Есть проблемы, проверяем логи...")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 20 --nostream")
    print(stdout.read().decode())

ssh.close()
