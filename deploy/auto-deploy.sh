#!/usr/bin/env bash
# =============================================================================
# EXRA — Auto Deploy Script (Build + Upload + Deploy)
# Запускать из корня репозитория: bash deploy/auto-deploy.sh
#
# Что делает:
#   1. Собирает проект (build.sh)
#   2. Загружает архив на сервер через scp
#   3. Распаковывает и запускает server-deploy.sh на сервере
#
# Требования:
#   - sshpass (для SSH с паролем): apt install sshpass / brew install sshpass
#   - Или настроить SSH ключ для автоматического входа
# =============================================================================
set -e

# =============================================================================
# КОНФИГУРАЦИЯ — ЗАПОЛНИ ОДИН РАЗ
# =============================================================================
SERVER_HOST=103.6.168.174           # IP или домен сервера
SERVER_USER="root"                      # пользователь SSH (обычно root)
SERVER_PASSWORD="Kukusa13Kro333"                      # пароль SSH (оставь пустым если используешь ключ)
SERVER_DEPLOY_DIR="/tmp"                # куда загружать архив на сервере
# =============================================================================

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✓ $*${NC}"; }
warn() { echo -e "${YELLOW}! $*${NC}"; }
fail() { echo -e "${RED}✗ $*${NC}"; exit 1; }
step() { echo -e "\n${YELLOW}=== $* ===${NC}"; }

# Проверка конфигурации
if [ "${SERVER_HOST}" = "YOUR_SERVER_IP" ]; then
  fail "Заполни SERVER_HOST в deploy/auto-deploy.sh (строка 20)"
fi

# ============================================================
# 1. BUILD
# ============================================================
step "1/3 Сборка проекта"
cd "${REPO_ROOT}"
bash deploy/build.sh || fail "Сборка провалилась"

# Находим последний созданный архив
LATEST_ARCHIVE=$(ls -t "${REPO_ROOT}/deploy/dist/"*.tar.gz 2>/dev/null | head -1)
if [ -z "${LATEST_ARCHIVE}" ]; then
  fail "Архив не найден в deploy/dist/"
fi
ARCHIVE_NAME=$(basename "${LATEST_ARCHIVE}")
ok "Архив готов: ${ARCHIVE_NAME}"

# ============================================================
# 2. UPLOAD
# ============================================================
step "2/3 Загрузка на сервер ${SERVER_USER}@${SERVER_HOST}"

# Выбираем метод подключения
if [ -n "${SERVER_PASSWORD}" ]; then
  # SSH с паролем через sshpass
  if ! command -v sshpass >/dev/null 2>&1; then
    fail "sshpass не установлен. Установи: apt install sshpass (Linux) или brew install sshpass (Mac)"
  fi
  SCP_CMD="sshpass -p '${SERVER_PASSWORD}' scp -o StrictHostKeyChecking=no"
  SSH_CMD="sshpass -p '${SERVER_PASSWORD}' ssh -o StrictHostKeyChecking=no"
else
  # SSH с ключом (без пароля)
  SCP_CMD="scp -o StrictHostKeyChecking=no"
  SSH_CMD="ssh -o StrictHostKeyChecking=no"
fi

# Загружаем архив
eval "${SCP_CMD} '${LATEST_ARCHIVE}' ${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY_DIR}/${ARCHIVE_NAME}" || fail "Загрузка провалилась"
ok "Архив загружен → ${SERVER_DEPLOY_DIR}/${ARCHIVE_NAME}"

# ============================================================
# 3. DEPLOY
# ============================================================
step "3/3 Деплой на сервере"

PACKAGE_DIR="${ARCHIVE_NAME%.tar.gz}"

eval "${SSH_CMD} ${SERVER_USER}@${SERVER_HOST}" << EOF
set -e
cd ${SERVER_DEPLOY_DIR}
echo "Распаковка архива..."
tar -xzf ${ARCHIVE_NAME}
echo "Запуск server-deploy.sh..."
cd ${PACKAGE_DIR}
sudo bash server-deploy.sh
echo "Очистка временных файлов..."
cd ..
rm -rf ${PACKAGE_DIR} ${ARCHIVE_NAME}
EOF

ok "Деплой завершён!"

echo ""
echo "========================================="
echo " Проверь сервисы:"
echo ""
echo "   ssh ${SERVER_USER}@${SERVER_HOST}"
echo "   pm2 status"
echo "   systemctl status exra.service"
echo "   curl http://localhost:8081/health"
echo "========================================="
