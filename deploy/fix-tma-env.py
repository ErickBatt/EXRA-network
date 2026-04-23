#!/usr/bin/env python3
"""
Исправление TMA переменных в /etc/exra.env на production сервере
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Текущий /etc/exra.env ===")
stdin, stdout, stderr = ssh.exec_command("cat /etc/exra.env")
current_env = stdout.read().decode()
print(current_env)

print("\n=== Проверка наличия TMA переменных ===")
has_tma_secret = "TMA_SESSION_SECRET" in current_env
has_bot_token = "TELEGRAM_BOT_TOKEN" in current_env
has_tma_base = "TMA_API_BASE" in current_env

print(f"TMA_SESSION_SECRET: {'✓ Есть' if has_tma_secret else '✗ Отсутствует'}")
print(f"TELEGRAM_BOT_TOKEN: {'✓ Есть' if has_bot_token else '✗ Отсутствует'}")
print(f"TMA_API_BASE: {'✓ Есть' if has_tma_base else '✗ Отсутствует'}")

if not has_tma_secret or not has_bot_token or not has_tma_base:
    print("\n=== Добавление недостающих TMA переменных ===")
    
    commands = []
    if not has_bot_token:
        commands.append('echo "TELEGRAM_BOT_TOKEN=8702349787:AAENQVRmAsCHUDeJyG8zqGsnZeKFA5fLoTE" >> /etc/exra.env')
    if not has_tma_secret:
        commands.append('echo "TMA_SESSION_SECRET=J5fOPYVo3QNzCjVXT6lRmgD4REi5mjNan4X8CC8ocmw" >> /etc/exra.env')
    if not has_tma_base:
        commands.append('echo "TMA_API_BASE=https://exra.space" >> /etc/exra.env')
    
    for cmd in commands:
        print(f"Выполняю: {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        stdout.read()
    
    print("\n=== Перезапуск сервиса ===")
    stdin, stdout, stderr = ssh.exec_command("systemctl restart exra.service")
    stdout.read()
    
    print("✓ Сервис перезапущен")
    
    import time
    time.sleep(3)
    
    print("\n=== Проверка статуса после перезапуска ===")
    stdin, stdout, stderr = ssh.exec_command("systemctl status exra.service --no-pager -n 10")
    print(stdout.read().decode())
    
    print("\n=== Проверка health endpoint ===")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8080/health")
    print(stdout.read().decode())
else:
    print("\n✓ Все TMA переменные присутствуют")

ssh.close()
