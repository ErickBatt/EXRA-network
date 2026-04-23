#!/usr/bin/env python3
"""
Сборка landing локально и деплой на сервер
"""
import paramiko
import subprocess
import tarfile
import os

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

print("="*70)
print("СБОРКА И ДЕПЛОЙ LANDING")
print("="*70)

# 1. Локальная сборка
print("\n1. Сборка Next.js локально...")
print("   Переход в landing/...")
os.chdir("landing")

print("   Установка зависимостей (если нужно)...")
subprocess.run(["npm", "install"], check=False)

print("   Сборка проекта...")
result = subprocess.run(["npm", "run", "build"], capture_output=True, text=True)
if result.returncode != 0:
    print("❌ Ошибка сборки:")
    print(result.stderr)
    exit(1)

print("✓ Сборка завершена")
os.chdir("..")

# 2. Создаем архив с собранным проектом
print("\n2. Создание архива...")
archive_path = "deploy/dist/landing-built.tar.gz"
os.makedirs("deploy/dist", exist_ok=True)

with tarfile.open(archive_path, "w:gz") as tar:
    tar.add("landing/.next", arcname=".next")
    tar.add("landing/app", arcname="app")
    tar.add("landing/components", arcname="components")
    tar.add("landing/public", arcname="public")

print(f"✓ Архив создан: {archive_path}")

# 3. Подключаемся к серверу
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

# 4. Загружаем архив
print("\n3. Загрузка на сервер...")
sftp = ssh.open_sftp()
remote_archive = "/tmp/landing-built.tar.gz"
sftp.put(archive_path, remote_archive)
sftp.close()
print(f"✓ Загружено: {remote_archive}")

# 5. Распаковываем и перезапускаем
print("\n4. Распаковка и перезапуск...")
commands = f"""
cd /var/www/landing
echo "Создание бэкапа..."
tar -czf /tmp/landing-backup-$(date +%s).tar.gz .next app components 2>/dev/null || true
echo "Распаковка новых файлов..."
tar -xzf {remote_archive} -C /var/www/landing/
echo "Установка прав..."
chown -R root:root /var/www/landing
echo "Очистка..."
rm {remote_archive}
echo "Перезапуск PM2..."
pm2 restart exra-landing
"""

stdin, stdout, stderr = ssh.exec_command(commands)
output = stdout.read().decode()
print(output)

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
    print("   https://exra.space - должна быть ссылка 'Marketplace' в навигации")
    print("   https://exra.space/marketplace - должен редиректить на app.exra.space")
else:
    print("\n⚠️  Есть проблемы, проверяем логи...")
    stdin, stdout, stderr = ssh.exec_command("pm2 logs exra-landing --lines 10 --nostream")
    print(stdout.read().decode())

ssh.close()
