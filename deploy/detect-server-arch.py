#!/usr/bin/env python3
"""
Определяет архитектуру сервера
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Архитектура сервера ===")
stdin, stdout, stderr = ssh.exec_command("uname -m")
arch = stdout.read().decode().strip()
print(f"Архитектура: {arch}")

stdin, stdout, stderr = ssh.exec_command("cat /proc/cpuinfo | grep 'model name' | head -1")
cpu = stdout.read().decode().strip()
print(f"CPU: {cpu}")

stdin, stdout, stderr = ssh.exec_command("lscpu | grep Architecture")
lscpu = stdout.read().decode().strip()
print(f"lscpu: {lscpu}")

ssh.close()

print("\n=== Рекомендация ===")
if 'aarch64' in arch or 'arm' in arch.lower():
    print("Сервер использует ARM64 архитектуру")
    print("Нужно собирать с: GOARCH=arm64")
elif 'x86_64' in arch or 'amd64' in arch:
    print("Сервер использует AMD64/x86_64 архитектуру")
    print("Нужно собирать с: GOARCH=amd64")
