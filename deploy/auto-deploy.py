#!/usr/bin/env python3
"""
EXRA — Auto Deploy Script (Python version for Windows)
Запускать: python deploy/auto-deploy.py

Требования: pip install paramiko
"""
import os
import sys
import subprocess
import glob
from pathlib import Path
import paramiko
from scp import SCPClient

# =============================================================================
# КОНФИГУРАЦИЯ
# =============================================================================
SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"
SERVER_DEPLOY_DIR = "/tmp"
# =============================================================================

REPO_ROOT = Path(__file__).parent.parent
GREEN = '\033[0;32m'
YELLOW = '\033[1;33m'
RED = '\033[0;31m'
NC = '\033[0m'

def ok(msg):
    print(f"{GREEN}✓ {msg}{NC}")

def warn(msg):
    print(f"{YELLOW}! {msg}{NC}")

def fail(msg):
    print(f"{RED}✗ {msg}{NC}")
    sys.exit(1)

def step(msg):
    print(f"\n{YELLOW}=== {msg} ==={NC}")

def run_command(cmd, cwd=None):
    """Запускает команду и возвращает результат"""
    result = subprocess.run(cmd, shell=True, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        print(result.stderr)
        return False
    return True

# ============================================================
# 1. BUILD
# ============================================================
step("1/3 Сборка проекта")
os.chdir(REPO_ROOT)

# Запускаем build.sh через bash (Git Bash должен быть установлен)
if not run_command("bash deploy/build.sh"):
    fail("Сборка провалилась")

# Находим последний архив
archives = glob.glob(str(REPO_ROOT / "deploy" / "dist" / "*.tar.gz"))
if not archives:
    fail("Архив не найден в deploy/dist/")

latest_archive = max(archives, key=os.path.getctime)
archive_name = os.path.basename(latest_archive)
ok(f"Архив готов: {archive_name}")

# ============================================================
# 2. UPLOAD
# ============================================================
step(f"2/3 Загрузка на сервер {SERVER_USER}@{SERVER_HOST}")

try:
    # Подключаемся по SSH
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD)
    
    # Загружаем файл через SCP
    with SCPClient(ssh.get_transport()) as scp:
        remote_path = f"{SERVER_DEPLOY_DIR}/{archive_name}"
        scp.put(latest_archive, remote_path)
    
    ok(f"Архив загружен → {remote_path}")
    
except Exception as e:
    fail(f"Загрузка провалилась: {e}")

# ============================================================
# 3. DEPLOY
# ============================================================
step("3/3 Деплой на сервере")

package_dir = archive_name.replace('.tar.gz', '')

deploy_commands = f"""set -e
cd {SERVER_DEPLOY_DIR}
echo "Распаковка архива..."
tar -xzf {archive_name}
echo "Запуск server-deploy.sh..."
cd {package_dir}
sudo bash server-deploy.sh
echo "Очистка временных файлов..."
cd ..
rm -rf {package_dir} {archive_name}
"""

try:
    stdin, stdout, stderr = ssh.exec_command(deploy_commands)
    exit_status = stdout.channel.recv_exit_status()
    
    # Выводим логи
    output = stdout.read().decode()
    errors = stderr.read().decode()
    
    if output:
        print(output)
    if errors:
        print(errors)
    
    if exit_status != 0:
        fail(f"Деплой провалился с кодом {exit_status}")
    
    ssh.close()
    ok("Деплой завершён!")
    
except Exception as e:
    fail(f"Деплой провалился: {e}")

print("\n=========================================")
print(" Проверь сервисы:")
print("")
print(f"   ssh {SERVER_USER}@{SERVER_HOST}")
print("   pm2 status")
print("   systemctl status exra.service")
print("   curl http://localhost:8081/health")
print("=========================================")

# Пауза перед закрытием окна
print("\n✓ Деплой завершён успешно!")
print("Нажми Enter для закрытия окна...")
input()


