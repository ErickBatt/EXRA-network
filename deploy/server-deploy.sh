#!/usr/bin/env bash
# =============================================================================
# EXRA — Server Deploy Script
# Запускать на сервере от root:  sudo bash server-deploy.sh
#
# Ожидаемая структура рядом со скриптом:
#   ./exra-server-linux    — Go binary
#   ./dashboard/           — Next.js standalone build
#       server.js
#       node_modules/
#       .next/static/
#       public/
# =============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# ---- пути на сервере (менять только если переезд) ----
BINARY_DST="/usr/local/bin/exra-server"
DASHBOARD_DST="/var/www/dashboard"
SYSTEMD_SERVICE="exra.service"
PM2_NAME="exra-dashboard"

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
ok "Go binary: $(du -sh "${SCRIPT_DIR}/exra-server-linux" | cut -f1)"
ok "Dashboard: OK"

# ============================================================
# 1. DEPLOY GO BACKEND
# ============================================================
step "1/3 Обновление Go backend"

systemctl stop "${SYSTEMD_SERVICE}" 2>/dev/null && ok "Сервис остановлен" || warn "Сервис уже был остановлен"
sleep 1

cp "${SCRIPT_DIR}/exra-server-linux" "${BINARY_DST}"
chmod +x "${BINARY_DST}"
ok "Binary скопирован → ${BINARY_DST}"

systemctl start "${SYSTEMD_SERVICE}"
sleep 2

if systemctl is-active --quiet "${SYSTEMD_SERVICE}"; then
  ok "Сервис запущен: ${SYSTEMD_SERVICE}"
else
  fail "Сервис не поднялся! Проверь: journalctl -u ${SYSTEMD_SERVICE} -n 30"
fi

# ============================================================
# 2. DEPLOY NEXT.JS DASHBOARD (STANDALONE)
# ============================================================
step "2/3 Обновление Next.js dashboard"

# Сохраняем .env.local — НЕ перезаписываем (там секреты)
ENV_BACKUP=""
if [ -f "${DASHBOARD_DST}/.env.local" ]; then
  ENV_BACKUP=$(mktemp)
  cp "${DASHBOARD_DST}/.env.local" "${ENV_BACKUP}"
  ok ".env.local сохранён"
fi

# Останавливаем PM2
pm2 stop "${PM2_NAME}" 2>/dev/null && ok "PM2 остановлен" || warn "PM2 процесс не был запущен"

# Копируем standalone build
mkdir -p "${DASHBOARD_DST}"
# rsync сохраняет .env.local если он уже там — но мы уже сохранили, копируем всё
cp -r "${SCRIPT_DIR}/dashboard/." "${DASHBOARD_DST}/"
ok "Dashboard файлы скопированы → ${DASHBOARD_DST}"

# Восстанавливаем .env.local
if [ -n "${ENV_BACKUP}" ]; then
  cp "${ENV_BACKUP}" "${DASHBOARD_DST}/.env.local"
  rm "${ENV_BACKUP}"
  ok ".env.local восстановлен"
fi

# Запускаем PM2 (standalone: node server.js напрямую, не npm start)
if pm2 show "${PM2_NAME}" >/dev/null 2>&1; then
  # Процесс уже зарегистрирован — рестартуем с новым кодом
  pm2 restart "${PM2_NAME}"
  ok "PM2 перезапущен"
else
  # Первый раз — регистрируем
  PORT=3000 pm2 start "${DASHBOARD_DST}/server.js" \
    --name "${PM2_NAME}" \
    --env production
  pm2 save
  ok "PM2 запущен и сохранён"
fi

sleep 3

# ============================================================
# 3. HEALTH CHECKS
# ============================================================
step "3/3 Проверка здоровья сервисов"

# API
if curl -sf --max-time 5 http://localhost:8080/health >/dev/null 2>&1; then
  ok "API /health: OK (порт 8080)"
else
  warn "API не отвечает на /health — проверь: journalctl -u ${SYSTEMD_SERVICE} -n 20"
fi

# Dashboard
if curl -sf --max-time 10 http://localhost:3000 >/dev/null 2>&1; then
  ok "Dashboard: OK (порт 3000)"
else
  warn "Dashboard не отвечает — проверь: pm2 logs ${PM2_NAME} --lines 30"
fi

echo ""
echo "========================================="
ok "Деплой завершён!"
echo ""
echo " Полезные команды:"
echo "   pm2 logs ${PM2_NAME}          — логи dashboard"
echo "   journalctl -u ${SYSTEMD_SERVICE} -f   — логи API"
echo "   pm2 status                    — статус PM2"
echo "   systemctl status ${SYSTEMD_SERVICE}   — статус API"
echo "========================================="
