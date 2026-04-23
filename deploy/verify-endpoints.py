#!/usr/bin/env python3
"""
Проверка работоспособности критических endpoints после исправления
"""
import paramiko
import json

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

endpoints = [
    ("Health Check", "curl -s http://localhost:8080/health"),
    ("Nodes Stats", "curl -s http://localhost:8080/nodes/stats"),
    ("Public Nodes", "curl -s http://localhost:8080/nodes | head -c 200"),
    ("TMA Epoch", "curl -s http://localhost:8080/api/tma/epoch"),
]

print("=== Проверка endpoints ===\n")
for name, cmd in endpoints:
    print(f"📍 {name}")
    print(f"   Команда: {cmd}")
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode().strip()
    error = stderr.read().decode().strip()
    
    if error:
        print(f"   ❌ Ошибка: {error}")
    elif output:
        print(f"   ✅ Ответ: {output[:150]}...")
    else:
        print(f"   ⚠️  Пустой ответ")
    print()

print("\n=== Проверка логов на ошибки TMA ===")
stdin, stdout, stderr = ssh.exec_command("journalctl -u exra.service -n 20 --no-pager | grep -i 'tma\\|error\\|500'")
logs = stdout.read().decode()
if logs.strip():
    print(logs)
else:
    print("✅ Ошибок TMA не найдено в последних 20 строках логов")

print("\n=== Проверка внешнего доступа (через nginx) ===")
stdin, stdout, stderr = ssh.exec_command("curl -s -o /dev/null -w '%{http_code}' https://exra.space/health")
status_code = stdout.read().decode().strip()
print(f"https://exra.space/health → HTTP {status_code}")

if status_code == "200":
    print("✅ Сайт доступен извне!")
else:
    print(f"⚠️  Неожиданный код ответа: {status_code}")
    print("\n=== Проверка nginx ===")
    stdin, stdout, stderr = ssh.exec_command("systemctl status nginx --no-pager -n 5")
    print(stdout.read().decode())

ssh.close()
