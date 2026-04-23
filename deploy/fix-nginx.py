#!/usr/bin/env python3
"""
Проверка и исправление конфигурации nginx
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Проверка конфигурации nginx для exra.space ===\n")

# Ищем конфиг файл
stdin, stdout, stderr = ssh.exec_command("ls -la /etc/nginx/sites-enabled/ | grep exra")
print("Файлы в sites-enabled:")
print(stdout.read().decode())

stdin, stdout, stderr = ssh.exec_command("ls -la /etc/nginx/sites-available/ | grep exra")
print("\nФайлы в sites-available:")
print(stdout.read().decode())

# Проверяем основной конфиг
stdin, stdout, stderr = ssh.exec_command("cat /etc/nginx/sites-enabled/exra.space 2>/dev/null || cat /etc/nginx/sites-available/exra.space 2>/dev/null || echo 'Конфиг не найден'")
config = stdout.read().decode()
print("\n=== Текущий конфиг exra.space ===")
print(config)

# Проверяем логи nginx
print("\n=== Последние ошибки nginx ===")
stdin, stdout, stderr = ssh.exec_command("tail -20 /var/log/nginx/error.log")
print(stdout.read().decode())

# Проверяем что слушает порт 80/443
print("\n=== Порты 80 и 443 ===")
stdin, stdout, stderr = ssh.exec_command("netstat -tlnp | grep ':80\\|:443'")
print(stdout.read().decode())

# Тест конфигурации nginx
print("\n=== Тест конфигурации nginx ===")
stdin, stdout, stderr = ssh.exec_command("nginx -t")
print(stdout.read().decode())
print(stderr.read().decode())

ssh.close()
