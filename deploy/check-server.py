#!/usr/bin/env python3
"""
Проверка состояния сервера и логов
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Логи exra.service (последние 50 строк) ===")
stdin, stdout, stderr = ssh.exec_command("journalctl -u exra.service -n 50 --no-pager")
print(stdout.read().decode())

print("\n=== PM2 статус ===")
stdin, stdout, stderr = ssh.exec_command("pm2 status")
print(stdout.read().decode())

print("\n=== Проверка /etc/exra.env ===")
stdin, stdout, stderr = ssh.exec_command("ls -la /etc/exra.env")
print(stdout.read().decode())

ssh.close()
