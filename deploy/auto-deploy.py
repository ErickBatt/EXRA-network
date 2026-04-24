#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
EXRA — Auto Deploy Script
Запускать из корня репо: python deploy/auto-deploy.py
"""
import os
import sys
import subprocess
import glob
from pathlib import Path

# Fix Windows cp1251 terminal — force UTF-8 output
if hasattr(sys.stdout, 'reconfigure'):
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')

# =============================================================================
# КОНФИГУРАЦИЯ
# =============================================================================
SERVER_HOST = "103.6.168.174"
SERVER_USER = "root"
SERVER_PASSWORD = "Kukusa13Kro333"
SERVER_DEPLOY_DIR = "/tmp"
# =============================================================================

REPO_ROOT = Path(__file__).parent.parent
GREEN  = '\033[0;32m'
YELLOW = '\033[1;33m'
RED    = '\033[0;31m'
NC     = '\033[0m'

def ok(msg):   print(f"{GREEN}[OK] {msg}{NC}")
def warn(msg): print(f"{YELLOW}[!]  {msg}{NC}")
def step(msg): print(f"\n{YELLOW}=== {msg} ==={NC}")

def fail(msg):
    print(f"\n{RED}[ОШИБКА] {msg}{NC}")
    print("\nНажми Enter для закрытия...")
    input()
    sys.exit(1)

def check_deps():
    missing = []
    try:
        import paramiko
    except ImportError:
        missing.append("paramiko")
    try:
        from scp import SCPClient
    except ImportError:
        missing.append("scp")
    if missing:
        print(f"{RED}Не установлены зависимости: {', '.join(missing)}{NC}")
        print(f"Выполни: pip install {' '.join(missing)}")
        print("\nНажми Enter для закрытия...")
        input()
        sys.exit(1)

def run_command(cmd, cwd=None, stream=True):
    """Запускает команду, выводит output в реальном времени"""
    proc = subprocess.Popen(
        cmd, shell=True, cwd=cwd,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        text=True, encoding='utf-8', errors='replace'
    )
    output = []
    for line in proc.stdout:
        print(line, end='')
        output.append(line)
    proc.wait()
    return proc.returncode == 0, ''.join(output)

# ============================================================
# MAIN
# ============================================================
check_deps()

import paramiko
from scp import SCPClient

os.chdir(REPO_ROOT)

# ---- 1. BUILD ----
step("1/3 Сборка проекта")
ok_build, out = run_command("bash deploy/build.sh")
if not ok_build:
    fail("Сборка провалилась — см. вывод выше")

archives = sorted(glob.glob(str(REPO_ROOT / "deploy" / "dist" / "*.tar.gz")), key=os.path.getctime)
if not archives:
    fail("Архив не найден в deploy/dist/")

latest_archive = archives[-1]
archive_name   = os.path.basename(latest_archive)
ok(f"Архив: {archive_name}")

# ---- 2. UPLOAD ----
step(f"2/3 Загрузка на сервер {SERVER_USER}@{SERVER_HOST}")

try:
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER_HOST, username=SERVER_USER, password=SERVER_PASSWORD, timeout=30)

    with SCPClient(ssh.get_transport(), progress=lambda f,s,p: print(f"\r  Загружено: {p}/{s} байт", end='')) as scp:
        scp.put(latest_archive, f"{SERVER_DEPLOY_DIR}/{archive_name}")
    print()
    ok(f"Загружен → {SERVER_DEPLOY_DIR}/{archive_name}")

except Exception as e:
    fail(f"Загрузка провалилась: {e}")

# ---- 3. DEPLOY ----
step("3/3 Деплой на сервере")

package_dir = archive_name.replace('.tar.gz', '')
deploy_cmd = f"""set -e
cd {SERVER_DEPLOY_DIR}
tar -xzf {archive_name}
cd {package_dir}
bash server-deploy.sh
cd ..
rm -rf {package_dir} {archive_name}
"""

try:
    stdin, stdout, stderr = ssh.exec_command(deploy_cmd, get_pty=True)
    for line in stdout:
        print(line, end='')
    exit_status = stdout.channel.recv_exit_status()
    ssh.close()

    if exit_status != 0:
        fail(f"server-deploy.sh завершился с кодом {exit_status}")

except Exception as e:
    fail(f"Деплой провалился: {e}")

print(f"""
=========================================
 Готово! Проверь:

   ssh {SERVER_USER}@{SERVER_HOST}
   pm2 status
   systemctl status exra.service
   curl http://localhost:8081/health
=========================================
""")
print("Нажми Enter для закрытия...")
input()
