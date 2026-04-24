#!/usr/bin/env bash
# =============================================================================
# EXRA — Server Deploy Script
# Запускать на сервере от root:  sudo bash server-deploy.sh
#
# Ожидаемая структура рядом со скриптом:
#   ./exra-server-linux    — Go binary (control plane + gateway)
#   ./dashboard/           — Next.js standalone build (port 3000)
#       server.js, node_modules/, .next/static/, public/
#   ./landing/             — Next.js standalone build (port 3001)
#       server.js, node_modules/, .next/static/, public/
#   ./env.sample           — шаблон /etc/exra.env (audit v2.4.1 vars)
# =============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# ---- пути на сервере (менять только если переезд) ----
BINARY_DST="/usr/local/bin/exra-server"
DASHBOARD_DST="/var/www/dashboard"
LANDING_DST="/var/www/landing"
SYSTEMD_SERVICE="exra.service"
PM2_DASHBOARD="exra-dashboard"
PM2_LANDING="exra-landing"
ENV_FILE="/etc/exra.env"

# ---- цвета ----
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✓ $*${NC}"; }
warn() { echo -e "${YELLOW}! $*${NC}"; }
fail() { echo -e "${RED}✗ $*${NC}"; exit 1; }
step() { echo -e "\n${YELLOW}=== $* ===${NC}"; }

# ---- проверка root ----
[ "$(id -u)" -eq 0 ] || fail "Запускать от root: sudo bash server-deploy.sh"

# ---- проверка файлов ----
step "Проверка пакета"
[ -f "${SCRIPT_DIR}/exra-server-linux" ] || fail "exra-server-linux не найден в ${SCRIPT_DIR}"
[ -f "${SCRIPT_DIR}/dashboard/server.js" ] || fail "dashboard/server.js не найден — неполный архив"
[ -f "${SCRIPT_DIR}/landing/server.js" ] || fail "landing/server.js не найден — неполный архив"
ok "Go binary: $(du -sh "${SCRIPT_DIR}/exra-server-linux" | cut -f1)"
ok "Dashboard:  $(du -sh "${SCRIPT_DIR}/dashboard" | cut -f1)"
ok "Landing:    $(du -sh "${SCRIPT_DIR}/landing" | cut -f1)"

# ---- проверка audit v2.4.1 env vars ----
step "Проверка /etc/exra.env (audit v2.4.1 required vars)"
if [ ! -f "${ENV_FILE}" ]; then
  warn "${ENV_FILE} не существует — копирую шаблон"
  if [ -f "${SCRIPT_DIR}/env.sample" ]; then
    cp "${SCRIPT_DIR}/env.sample" "${ENV_FILE}"
    chmod 600 "${ENV_FILE}"
    warn "ЗАПОЛНИ реальные значения в ${ENV_FILE} и запусти ещё раз!"
    warn "Особенно: WS_ALLOWED_ORIGINS, GATEWAY_JWT_PUBLIC_KEY,"
    warn "          GATEWAY_JWT_PRIVATE_KEY, PEAQ_ORACLE_SEED"
    exit 1
  else
    fail "env.sample отсутствует в пакете"
  fi
fi

# Проверяем что ключевые vars не содержат REPLACE_* плейсхолдеры.
# Для JWT ключей принимаем либо EdDSA пару (PRIV+PUB), либо транзитивный HMAC SECRET.
check_var() {
  local var="$1"
  local val
  val=$(grep -E "^${var}=" "${ENV_FILE}" | head -1 | cut -d= -f2- | tr -d '"' || true)
  [ -n "${val}" ] && [[ "${val}" != REPLACE_* ]]
}

MISSING_VARS=()
for var in WS_ALLOWED_ORIGINS PEAQ_ORACLE_SEED; do
  check_var "${var}" || MISSING_VARS+=("${var}")
done

# JWT: нужна либо EdDSA пара, либо транзитивный HMAC
if ! { check_var GATEWAY_JWT_ED25519_PRIV && check_var GATEWAY_JWT_ED25519_PUB; } \
   && ! check_var GATEWAY_JWT_SECRET; then
  MISSING_VARS+=("GATEWAY_JWT_ED25519_PRIV+PUB или GATEWAY_JWT_SECRET")
fi

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
  fail "В ${ENV_FILE} не заполнены audit v2.4.1 переменные: ${MISSING_VARS[*]}. Сервер не стартанёт в secure-режиме. Заполни и запусти ещё раз."
fi
ok "/etc/exra.env — audit v2.4.1 переменные выставлены"

# ============================================================
# 0. DB MIGRATIONS (идемпотентные — ALTER IF NOT EXISTS / CREATE IF NOT EXISTS)
# ============================================================
step "0/4 Применение DB миграций"

MIG_DIR_DST="/root/exra/server/migrations"
if [ -d "${SCRIPT_DIR}/migrations" ]; then
  mkdir -p "${MIG_DIR_DST}"
  cp "${SCRIPT_DIR}/migrations/"*.sql "${MIG_DIR_DST}/"
  # postgres user не может читать из /root/ (home 700) — используем /tmp/ как
  # временный стейджинг для psql -f
   MIG_STAGE=$(mktemp -d /tmp/exra-mig-XXXXXX)
   cp "${MIG_DIR_DST}"/*.sql "${MIG_STAGE}/"
   chmod -R 644 "${MIG_STAGE}"/*.sql
   chmod 755 "${MIG_STAGE}"
   ok "Миграции скопированы → ${MIG_DIR_DST} (stage: ${MIG_STAGE})"

  # Извлекаем DB имя из SUPABASE_URL в env (postgres://user:pass@host/DBNAME?...)
  DB_NAME=$(grep -E "^SUPABASE_URL=" "${ENV_FILE}" | head -1 | sed -E 's|.*/([^/?]+)(\?.*)?$|\1|')
  DB_NAME="${DB_NAME:-exra}"
  ok "Target DB: ${DB_NAME}"

  FAILED_MIGS=()
  for f in $(ls "${MIG_STAGE}"/*.sql | sort); do
    name=$(basename "$f")
    # UTF-8 check — пропускаем файлы с невалидным encoding (типа 009_buyer_email.sql)
    if ! iconv -f utf-8 -t utf-8 "$f" >/dev/null 2>&1; then
      warn "  SKIP ${name} (невалидный UTF-8)"
      continue
    fi
    if sudo -u postgres psql "${DB_NAME}" -v ON_ERROR_STOP=0 -q -f "$f" >/tmp/mig.log 2>&1; then
      ok "  ${name}"
    else
      if grep -qvE "already exists|does not exist, skipping|NOTICE" /tmp/mig.log; then
        tail -3 /tmp/mig.log | while IFS= read -r line; do
          warn "    ${line}"
        done
      fi
      FAILED_MIGS+=("${name}")
    fi
  done
  rm -f /tmp/mig.log
  rm -rf "${MIG_STAGE}"

  if [ ${#FAILED_MIGS[@]} -gt 0 ]; then
    warn "Миграции с предупреждениями: ${FAILED_MIGS[*]} (проверь вручную если важно)"
  fi
else
  warn "Папка migrations/ отсутствует в пакете — пропускаю"
fi

# ============================================================
# 1. DEPLOY GO BACKEND
# ============================================================
step "1/4 Обновление Go backend"

systemctl stop "${SYSTEMD_SERVICE}" 2>/dev/null && ok "Сервис остановлен" || warn "Сервис уже был остановлен"
sleep 1

# Синхронизируем исходники из архива, затем билдим на сервере
SOURCE_DIR="/root/exra/server"
if [ -d "${SCRIPT_DIR}/server-src" ]; then
  mkdir -p "${SOURCE_DIR}"
  rsync -a --delete \
    --exclude='*.exe' --exclude='exra-server-linux*' --exclude='*.log' \
    "${SCRIPT_DIR}/server-src/" "${SOURCE_DIR}/"
  ok "Исходники обновлены → ${SOURCE_DIR}"
else
  warn "server-src не найден в архиве — билдим из текущего кода на сервере"
fi

command -v go >/dev/null 2>&1 || fail "Go не установлен на сервере"
ok "Сборка ($(go version | awk '{print $3}'))..."
cd "${SOURCE_DIR}"
go build -o "${BINARY_DST}" .
ok "Binary собран → ${BINARY_DST}"
chmod +x "${BINARY_DST}"

# Убиваем зависший процесс на порту 8080 (бывает при нечистом стопе сервиса)
fuser -k 8080/tcp 2>/dev/null || true
sleep 1

systemctl start "${SYSTEMD_SERVICE}"
sleep 2

if systemctl is-active --quiet "${SYSTEMD_SERVICE}"; then
  ok "Сервис запущен: ${SYSTEMD_SERVICE}"
else
  echo ""
  echo "=== ОШИБКА: Сервис не поднялся! Логи: ==="
  journalctl -u "${SYSTEMD_SERVICE}" -n 50 --no-pager
  echo "=== Конец логов ==="
  fail "Сервис не поднялся! Проверь логи выше"
fi

# ============================================================
# 2. DEPLOY NEXT.JS DASHBOARD (STANDALONE, port 3000)
# ============================================================
step "2/4 Обновление Next.js dashboard"

# Сохраняем .env.local — НЕ перезаписываем (там секреты)
ENV_BACKUP=""
if [ -f "${DASHBOARD_DST}/.env.local" ]; then
  ENV_BACKUP=$(mktemp)
  cp "${DASHBOARD_DST}/.env.local" "${ENV_BACKUP}"
  ok "dashboard .env.local сохранён"
fi

pm2 stop "${PM2_DASHBOARD}" 2>/dev/null && ok "PM2 dashboard остановлен" || warn "PM2 dashboard не был запущен"

mkdir -p "${DASHBOARD_DST}"
cp -r "${SCRIPT_DIR}/dashboard/." "${DASHBOARD_DST}/"
ok "Dashboard файлы скопированы → ${DASHBOARD_DST}"

if [ -n "${ENV_BACKUP}" ]; then
  cp "${ENV_BACKUP}" "${DASHBOARD_DST}/.env.local"
  rm "${ENV_BACKUP}"
  ok "dashboard .env.local восстановлен"
fi

if pm2 show "${PM2_DASHBOARD}" >/dev/null 2>&1; then
  pm2 restart "${PM2_DASHBOARD}"
  ok "PM2 dashboard перезапущен"
else
  PORT=3000 pm2 start "${DASHBOARD_DST}/server.js" \
    --name "${PM2_DASHBOARD}" \
    --env production
  pm2 save
  ok "PM2 dashboard запущен и сохранён"
fi

# ============================================================
# 3. DEPLOY NEXT.JS LANDING (STANDALONE, port 3001)
# ============================================================
step "3/4 Обновление Next.js landing"

ENV_BACKUP_L=""
if [ -f "${LANDING_DST}/.env.local" ]; then
  ENV_BACKUP_L=$(mktemp)
  cp "${LANDING_DST}/.env.local" "${ENV_BACKUP_L}"
  ok "landing .env.local сохранён"
fi

pm2 stop "${PM2_LANDING}" 2>/dev/null && ok "PM2 landing остановлен" || warn "PM2 landing не был запущен"

mkdir -p "${LANDING_DST}"
cp -r "${SCRIPT_DIR}/landing/." "${LANDING_DST}/"
ok "Landing файлы скопированы → ${LANDING_DST}"

if [ -n "${ENV_BACKUP_L}" ]; then
  cp "${ENV_BACKUP_L}" "${LANDING_DST}/.env.local"
  rm "${ENV_BACKUP_L}"
  ok "landing .env.local восстановлен"
fi

if pm2 show "${PM2_LANDING}" >/dev/null 2>&1; then
  pm2 restart "${PM2_LANDING}"
  ok "PM2 landing перезапущен"
else
  PORT=3001 pm2 start "${LANDING_DST}/server.js" \
    --name "${PM2_LANDING}" \
    --env production
  pm2 save
  ok "PM2 landing запущен и сохранён"
fi

sleep 3

# ============================================================
# 4. HEALTH CHECKS
# ============================================================
step "4/4 Проверка здоровья сервисов"

# Control plane API
if curl -sf --max-time 5 http://localhost:8081/health >/dev/null 2>&1; then
  ok "API /health: OK (порт 8081)"
elif curl -sf --max-time 5 http://localhost:8080/health >/dev/null 2>&1; then
  ok "API /health: OK (порт 8080 — legacy)"
else
  warn "API не отвечает — проверь: journalctl -u ${SYSTEMD_SERVICE} -n 20"
fi

# Dashboard
if curl -sf --max-time 10 http://localhost:3000 >/dev/null 2>&1; then
  ok "Dashboard: OK (порт 3000)"
else
  warn "Dashboard не отвечает — проверь: pm2 logs ${PM2_DASHBOARD} --lines 30"
fi

# Landing
if curl -sf --max-time 10 http://localhost:3001 >/dev/null 2>&1; then
  ok "Landing: OK (порт 3001)"
else
  warn "Landing не отвечает — проверь: pm2 logs ${PM2_LANDING} --lines 30"
fi

echo ""
echo "========================================="
ok "Деплой завершён!"
echo ""
echo " Полезные команды:"
echo "   pm2 logs ${PM2_DASHBOARD}         — логи dashboard"
echo "   pm2 logs ${PM2_LANDING}           — логи landing"
echo "   journalctl -u ${SYSTEMD_SERVICE} -f   — логи API"
echo "   pm2 status                        — статус всех PM2 процессов"
echo "   systemctl status ${SYSTEMD_SERVICE}   — статус API"
echo ""
echo " Nginx (если ещё не настроен):"
echo "   exra.space           → proxy_pass http://localhost:3001   (landing)"
echo "   app.exra.space       → proxy_pass http://localhost:3000   (dashboard)"
echo "   dashboard.exra.space → proxy_pass http://localhost:3000   (dashboard)"
echo "   api.exra.space       → proxy_pass http://localhost:8081   (control plane)"
echo "========================================="
