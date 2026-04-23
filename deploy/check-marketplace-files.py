#!/usr/bin/env python3
"""
Проверка наличия файлов marketplace на сервере
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Проверка файлов marketplace ===\n")

# Проверяем исходники
stdin, stdout, stderr = ssh.exec_command("ls -la /var/www/landing/app/marketplace/ 2>/dev/null")
marketplace_src = stdout.read().decode()
if marketplace_src.strip():
    print("✓ Исходники app/marketplace существуют:")
    print(marketplace_src)
else:
    print("❌ Исходники app/marketplace НЕ найдены")

# Проверяем собранные файлы в .next
print("\n=== Проверка .next/server/app/marketplace ===")
stdin, stdout, stderr = ssh.exec_command("ls -la /var/www/landing/.next/server/app/marketplace/ 2>/dev/null")
marketplace_built = stdout.read().decode()
if marketplace_built.strip():
    print("✓ Собранные файлы существуют:")
    print(marketplace_built)
else:
    print("❌ Собранные файлы НЕ найдены - нужна пересборка")

# Проверяем package.json для понимания как собирать
print("\n=== package.json (build script) ===")
stdin, stdout, stderr = ssh.exec_command("cat /var/www/landing/package.json | grep -A 3 'scripts'")
print(stdout.read().decode())

# Проверяем где находится next
print("\n=== Поиск next ===")
stdin, stdout, stderr = ssh.exec_command("which next || find /var/www/landing/node_modules/.bin -name 'next' 2>/dev/null")
next_path = stdout.read().decode().strip()
if next_path:
    print(f"✓ Next.js найден: {next_path}")
else:
    print("❌ Next.js не найден")

ssh.close()
