#!/usr/bin/env python3
"""
Проверка всех nginx конфигов exra
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

configs = ['exra', 'exra-api', 'exra-app']

for config_name in configs:
    print(f"\n{'='*60}")
    print(f"=== /etc/nginx/sites-available/{config_name} ===")
    print('='*60)
    stdin, stdout, stderr = ssh.exec_command(f"cat /etc/nginx/sites-available/{config_name}")
    print(stdout.read().decode())

# Проверяем какие домены слушает nginx
print(f"\n{'='*60}")
print("=== Все server_name в конфигах ===")
print('='*60)
stdin, stdout, stderr = ssh.exec_command("grep -r 'server_name' /etc/nginx/sites-enabled/")
print(stdout.read().decode())

# Проверяем доступность через curl с разными доменами
print(f"\n{'='*60}")
print("=== Тест доступности ===")
print('='*60)

tests = [
    "curl -s -o /dev/null -w 'exra.space/health: %{http_code}\\n' https://exra.space/health",
    "curl -s -o /dev/null -w 'www.exra.space/health: %{http_code}\\n' https://www.exra.space/health",
    "curl -s -o /dev/null -w 'app.exra.space/health: %{http_code}\\n' https://app.exra.space/health",
]

for test in tests:
    stdin, stdout, stderr = ssh.exec_command(test)
    print(stdout.read().decode().strip())

ssh.close()
