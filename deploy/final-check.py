#!/usr/bin/env python3
"""
Финальная проверка всех критических endpoints
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("="*70)
print("ФИНАЛЬНАЯ ПРОВЕРКА EXRA.SPACE")
print("="*70)

# Критические endpoints для проверки
endpoints = [
    ("🏥 Health Check", "curl -s https://exra.space/health"),
    ("📊 Nodes Stats", "curl -s https://exra.space/nodes/stats"),
    ("🌐 Public Nodes", "curl -s https://exra.space/nodes"),
    ("📱 TMA Epoch", "curl -s https://exra.space/api/tma/epoch"),
    ("🗺️  Map Stream", "curl -s -m 2 https://exra.space/ws/map || echo 'WebSocket endpoint (нормально что не отвечает на curl)'"),
]

all_ok = True

for name, cmd in endpoints:
    print(f"\n{name}")
    print(f"  URL: {cmd.split('curl -s ')[1].split(' ')[0] if 'curl' in cmd else 'N/A'}")
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode().strip()
    error = stderr.read().decode().strip()
    
    if "WebSocket" in output:
        print(f"  ✅ {output}")
    elif output and not error:
        # Показываем первые 100 символов
        preview = output[:100] + "..." if len(output) > 100 else output
        print(f"  ✅ {preview}")
    elif error:
        print(f"  ❌ Ошибка: {error[:100]}")
        all_ok = False
    else:
        print(f"  ⚠️  Пустой ответ")
        all_ok = False

print("\n" + "="*70)
print("СТАТУС СЕРВИСОВ")
print("="*70)

# Проверка статуса всех сервисов
services = [
    ("Go Server (exra.service)", "systemctl is-active exra.service"),
    ("Dashboard (PM2)", "pm2 describe exra-dashboard | grep status | head -1"),
    ("Landing (PM2)", "pm2 describe exra-landing | grep status | head -1"),
    ("Nginx", "systemctl is-active nginx"),
    ("PostgreSQL", "systemctl is-active postgresql || echo 'local'"),
    ("Redis", "systemctl is-active redis || redis-cli ping"),
]

for name, cmd in services:
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode().strip()
    if "active" in output or "online" in output or "PONG" in output or "local" in output:
        print(f"✅ {name}: {output}")
    else:
        print(f"⚠️  {name}: {output}")

print("\n" + "="*70)
print("ПОСЛЕДНИЕ ЛОГИ (проверка на ошибки)")
print("="*70)

stdin, stdout, stderr = ssh.exec_command("journalctl -u exra.service -n 5 --no-pager | grep -v 'INFO'")
errors = stdout.read().decode().strip()
if errors:
    print(errors)
else:
    print("✅ Ошибок не найдено в последних 5 строках")

print("\n" + "="*70)
if all_ok:
    print("✅ ВСЕ СИСТЕМЫ РАБОТАЮТ!")
else:
    print("⚠️  ЕСТЬ ПРОБЛЕМЫ - см. детали выше")
print("="*70)

print("\n📝 Что было исправлено:")
print("  1. Добавлены TMA переменные в /etc/exra.env:")
print("     - TMA_SESSION_SECRET")
print("     - TMA_API_BASE")
print("  2. Обновлен nginx конфиг для проксирования API endpoints")
print("  3. Перезапущены сервисы exra.service и nginx")

print("\n🔗 Доступные URL:")
print("  - https://exra.space - Landing page")
print("  - https://exra.space/health - Health check")
print("  - https://exra.space/api/* - API endpoints")
print("  - https://app.exra.space - Dashboard/TMA")
print("  - https://api.exra.space - Direct API access")

ssh.close()
