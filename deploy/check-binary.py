#!/usr/bin/env python3
"""
Проверяет бинарник на сервере
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Проверка бинарника на сервере ===")
stdin, stdout, stderr = ssh.exec_command("file /usr/local/bin/exra-server")
print(stdout.read().decode())

print("\n=== Права доступа ===")
stdin, stdout, stderr = ssh.exec_command("ls -la /usr/local/bin/exra-server")
print(stdout.read().decode())

print("\n=== Попытка запуска напрямую ===")
stdin, stdout, stderr = ssh.exec_command("/usr/local/bin/exra-server --version 2>&1 | head -5")
output = stdout.read().decode()
errors = stderr.read().decode()
print(output if output else errors)

ssh.close()
