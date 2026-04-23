#!/usr/bin/env python3
"""
Обновление nginx конфига для проксирования всех API endpoints
"""
import paramiko

SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"

# Новый конфиг с правильным проксированием
NEW_CONFIG = """server {
    listen 443 ssl;
    server_name exra.space www.exra.space;
    ssl_certificate /etc/letsencrypt/live/exra.space/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/exra.space/privkey.pem;

    # Редиректы TMA на app.exra.space
    location ^~ /tma {
        return 301 https://app.exra.space$request_uri;
    }
    location ^~ /next-tma {
        return 301 https://app.exra.space$request_uri;
    }

    # API endpoints (включая /health, /nodes, /ws и т.д.)
    location ~ ^/(api|health|nodes|ws|proxy|oracle|claim) {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;
    }

    # Landing page (Next.js на порту 3001)
    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name exra.space www.exra.space;
    return 301 https://$host$request_uri;
}
"""

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)

print("=== Создание бэкапа текущего конфига ===")
stdin, stdout, stderr = ssh.exec_command("cp /etc/nginx/sites-available/exra /etc/nginx/sites-available/exra.bak.$(date +%s)")
print(stdout.read().decode())

print("\n=== Запись нового конфига ===")
# Экранируем конфиг для bash
escaped_config = NEW_CONFIG.replace('"', '\\"').replace('$', '\\$')
stdin, stdout, stderr = ssh.exec_command(f'cat > /etc/nginx/sites-available/exra << \'EOF\'\n{NEW_CONFIG}\nEOF')
print("✓ Конфиг записан")

print("\n=== Тест конфигурации nginx ===")
stdin, stdout, stderr = ssh.exec_command("nginx -t")
test_output = stdout.read().decode()
test_error = stderr.read().decode()
print(test_output)
print(test_error)

if "successful" in test_error or "successful" in test_output:
    print("\n=== Перезагрузка nginx ===")
    stdin, stdout, stderr = ssh.exec_command("systemctl reload nginx")
    print("✓ Nginx перезагружен")
    
    import time
    time.sleep(2)
    
    print("\n=== Проверка доступности ===")
    tests = [
        ("Health", "curl -s https://exra.space/health"),
        ("Nodes Stats", "curl -s https://exra.space/nodes/stats"),
        ("TMA Epoch", "curl -s https://exra.space/api/tma/epoch"),
    ]
    
    for name, cmd in tests:
        stdin, stdout, stderr = ssh.exec_command(cmd)
        output = stdout.read().decode()[:200]
        print(f"\n{name}:")
        print(f"  {output}...")
else:
    print("\n❌ Ошибка в конфигурации! Откат не выполнен.")
    print("Проверь конфиг вручную: ssh root@103.6.168.174")

ssh.close()
