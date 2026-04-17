#!/bin/bash
# EXRA Server — One-shot deployment script
# Run as root on Ubuntu/Debian VPS
# Usage: bash deploy.sh

set -e
echo "=== EXRA Deploy ==="

# ── 1. System packages ──────────────────────────────────────────────────────
apt-get update -qq
apt-get install -y -qq curl wget git postgresql postgresql-contrib redis-server ufw fail2ban

# ── 2. Go 1.22 ──────────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "[Go] Installing..."
  wget -q https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -O /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  export PATH=$PATH:/usr/local/go/bin
  echo "[Go] $(go version)"
else
  export PATH=$PATH:/usr/local/go/bin
  echo "[Go] already installed: $(go version)"
fi

# ── 3. PostgreSQL ────────────────────────────────────────────────────────────
echo "[DB] Setting up PostgreSQL..."
systemctl enable postgresql --now
PG_PASS=$(openssl rand -base64 24 | tr -d '/+=')
sudo -u postgres psql -c "CREATE USER exra WITH PASSWORD '$PG_PASS';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE exra OWNER exra;" 2>/dev/null || true
DB_URL="postgres://exra:${PG_PASS}@localhost:5432/exra?sslmode=disable"
echo "[DB] ready → $DB_URL"

# ── 4. Redis ─────────────────────────────────────────────────────────────────
systemctl enable redis-server --now
echo "[Redis] ready"

# ── 5. Clone / update repo ───────────────────────────────────────────────────
REPO_DIR=/opt/exra
if [ -d "$REPO_DIR/.git" ]; then
  echo "[Git] pulling latest..."
  cd $REPO_DIR && git pull
else
  echo "[Git] IMPORTANT: paste your server code here manually or set up git remote"
  mkdir -p $REPO_DIR/server
fi

# ── 6. Build ─────────────────────────────────────────────────────────────────
cd $REPO_DIR/server
echo "[Build] building exra server..."
go build -o /usr/local/bin/exra-server . 2>&1
echo "[Build] done → /usr/local/bin/exra-server"

# ── 7. .env file ─────────────────────────────────────────────────────────────
NODE_SECRET=$(openssl rand -hex 32)
PROXY_SECRET=$(openssl rand -hex 32)

cat > /etc/exra.env <<EOF
PORT=8080
SUPABASE_URL=${DB_URL}
REDIS_URL=redis://localhost:6379
NODE_SECRET=${NODE_SECRET}
PROXY_SECRET=${PROXY_SECRET}

# PEAQ / DePIN — L1 Network Connectivity
PEAQ_RPC=${PEAQ_RPC:-"ws://127.0.0.1:9944"}
ORACLE_NODES=3
FEEDER_STAKE_MIN=10
TIMELOCK_ANON=24h

# Tokenomics (Policy finalized=true in production)
EXRA_MAX_SUPPLY=1000000000
POP_EMISSION_PER_HEARTBEAT=0.000050
RATE_PER_GB=0.30
EXRA_POLICY_FINALIZED=true

# PEAQ Identity (DID)
# ORACLE_PRIVATE_KEY should be set via environment variable for security!
# ORACLE_DID=did:peaq:0x...
EOF

chmod 600 /etc/exra.env
echo "[Env] written to /etc/exra.env"
echo "[Env] NODE_SECRET=${NODE_SECRET}"
echo "[Env] PROXY_SECRET=${PROXY_SECRET}"

# ── 8. GeoIP database ────────────────────────────────────────────────────────
if [ ! -f /usr/local/bin/GeoLite2-City.mmdb ]; then
  echo "[Geo] GeoLite2-City.mmdb not found — geo features will be disabled"
  echo "      Download from https://dev.maxmind.com/geoip/geolite2-free-geolocation-data"
fi

# ── 9. systemd service ───────────────────────────────────────────────────────
cat > /etc/systemd/system/exra.service <<'UNIT'
[Unit]
Description=EXRA DePIN Server
After=network.target postgresql.service redis-server.service
Wants=postgresql.service redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=/usr/local/bin
EnvironmentFile=/etc/exra.env
ExecStart=/usr/local/bin/exra-server
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable exra
systemctl restart exra
sleep 2

# ── 10. Firewall ─────────────────────────────────────────────────────────────
ufw --force enable
ufw allow 22/tcp
ufw allow 8080/tcp
ufw allow 443/tcp
ufw allow 80/tcp

# ── 11. Health check ─────────────────────────────────────────────────────────
sleep 3
echo ""
echo "=== Status ==="
systemctl status exra --no-pager -l | tail -20
echo ""
echo "=== Health check ==="
curl -s http://localhost:8080/health || echo "Server not responding yet — check: journalctl -u exra -f"
echo ""
echo "=== DONE ==="
SERVER_IP=$(curl -s ifconfig.me || echo "SERVER_IP")
echo "Server: http://${SERVER_IP}:8080"
echo "Health: http://${SERVER_IP}:8080/health"
echo "Logs:   journalctl -u exra -f"
echo ""
echo "SAVE THESE:"
echo "NODE_SECRET=${NODE_SECRET}"
echo "PROXY_SECRET=${PROXY_SECRET}"
