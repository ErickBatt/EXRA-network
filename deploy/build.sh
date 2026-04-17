#!/usr/bin/env bash
# =============================================================================
# EXRA — Local Build Script
# Запускать из корня репозитория: bash deploy/build.sh
#
# Что делает:
#   1. Собирает Go бинарник под Linux
#   2. Собирает Next.js dashboard (standalone режим)
#   3. Пакует всё в tar.gz архив
#
# Результат: deploy/dist/exra-deploy-TIMESTAMP.tar.gz
# Загружаем на сервер через WinSCP → распаковываем → запускаем server-deploy.sh
# =============================================================================
set -e

# =============================================================================
# PRODUCTION CONFIG — заполни один раз (публичные ключи, не секреты)
# Supabase anon key специально создан для браузера — его можно коммитить.
# =============================================================================
export NEXT_PUBLIC_SUPABASE_URL="https://ymytnxamelpdvrnczhee.supabase.co"
export NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY="sb_publishable_C_PqDyPasl0bnafsHx8fOg_UrV_ZYOd"
export NEXT_PUBLIC_API_BASE_URL=""   # пустая строка = relative URL через nginx
# =============================================================================

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PACKAGE_NAME="exra-deploy-${TIMESTAMP}"
DIST_DIR="${REPO_ROOT}/deploy/dist"
BUILD_DIR="${DIST_DIR}/${PACKAGE_NAME}"

# ---- цвета для читабельности ----
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✓ $*${NC}"; }
warn() { echo -e "${YELLOW}! $*${NC}"; }
fail() { echo -e "${RED}✗ $*${NC}"; exit 1; }
step() { echo -e "\n${YELLOW}=== $* ===${NC}"; }

# ---- проверка зависимостей ----
step "Проверка окружения"
command -v npm >/dev/null 2>&1 || fail "npm не установлен"
command -v tar >/dev/null 2>&1 || fail "tar не найден"
ok "Node: $(node --version)"
ok "Supabase URL: ${NEXT_PUBLIC_SUPABASE_URL}"

# ---- 1. Go binary ----
step "Сборка Go backend (linux/amd64)"
cd "${REPO_ROOT}/server"

GO_CMD="go"
if ! command -v $GO_CMD >/dev/null 2>&1; then
  if command -v go.exe >/dev/null 2>&1; then
    GO_CMD="go.exe"
  fi
fi

if command -v $GO_CMD >/dev/null 2>&1; then
  export GOOS=linux
  export GOARCH=amd64
  export CGO_ENABLED=0
  $GO_CMD build -ldflags="-s -w" -o exra-server-linux .
  ok "Binary пересобран: server/exra-server-linux"
elif [ -f "${REPO_ROOT}/server/exra-server-linux" ]; then
  warn "Go не найден — используем существующий бинарник (Go код не менялся? ОК)"
  ok "Binary: server/exra-server-linux"
else
  fail "Go не установлен и бинарник server/exra-server-linux не найден. Установи Go: https://go.dev/dl/"
fi

# ---- 2. Next.js dashboard ----
step "Сборка Next.js dashboard (standalone)"
cd "${REPO_ROOT}/dashboard"
npm run build
ok "Next.js build: OK"

# ---- 3. Packaging ----
step "Упаковка пакета: ${PACKAGE_NAME}"
mkdir -p "${BUILD_DIR}/dashboard"

# Go binary
cp "${REPO_ROOT}/server/exra-server-linux" "${BUILD_DIR}/"

# Next.js standalone (minimal runtime, не нужен npm install на сервере)
cp -r "${REPO_ROOT}/dashboard/.next/standalone/." "${BUILD_DIR}/dashboard/"
# Static assets и public нужно скопировать рядом со standalone
mkdir -p "${BUILD_DIR}/dashboard/.next"
cp -r "${REPO_ROOT}/dashboard/.next/static"  "${BUILD_DIR}/dashboard/.next/static"
[ -d "${REPO_ROOT}/dashboard/public" ] && cp -r "${REPO_ROOT}/dashboard/public" "${BUILD_DIR}/dashboard/public" || true

# Серверный deploy скрипт идёт внутри архива
cp "${REPO_ROOT}/deploy/server-deploy.sh" "${BUILD_DIR}/"
chmod +x "${BUILD_DIR}/server-deploy.sh"

# ---- 4. Archive ----
step "Создание архива"
mkdir -p "${DIST_DIR}"
cd "${DIST_DIR}"
tar -czf "${PACKAGE_NAME}.tar.gz" "${PACKAGE_NAME}"
rm -rf "${PACKAGE_NAME}"  # убираем распакованную папку, оставляем только архив

ARCHIVE="${DIST_DIR}/${PACKAGE_NAME}.tar.gz"
ok "Архив готов: ${ARCHIVE}"

echo ""
echo "========================================="
echo " Дальнейшие шаги:"
echo ""
echo " 1. Загрузить через WinSCP:"
echo "    ${ARCHIVE}"
echo "    → на сервер в /tmp/${PACKAGE_NAME}.tar.gz"
echo ""
echo " 2. На сервере:"
echo "    cd /tmp"
echo "    tar -xzf ${PACKAGE_NAME}.tar.gz"
echo "    sudo bash ${PACKAGE_NAME}/server-deploy.sh"
echo "========================================="
