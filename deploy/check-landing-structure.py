#!/usr/bin/env python3
"""
Проверка структуры landing на сервере
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Структура /var/www/landing ===\n")
stdin, stdout, stderr = ssh.exec_command("ls -la /var/www/landing/")
print(stdout.read().decode())

print("\n=== Есть ли .next? ===")
stdin, stdout, stderr = ssh.exec_command("test -d /var/www/landing/.next && echo 'YES' || echo 'NO'")
print(stdout.read().decode())

print("\n=== Поиск app и components ===")
stdin, stdout, stderr = ssh.exec_command("find /var/www/landing -maxdepth 2 -type d -name 'app' -o -name 'components' 2>/dev/null")
print(stdout.read().decode())

print("\n=== Содержимое .next (если есть) ===")
stdin, stdout, stderr = ssh.exec_command("ls -la /var/www/landing/.next/ 2>/dev/null | head -20")
print(stdout.read().decode())

ssh.close()
