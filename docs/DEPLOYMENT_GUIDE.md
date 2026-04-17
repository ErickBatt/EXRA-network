# Exra Server Deployment Guide

This guide provides step-by-step instructions for deploying the Exra Hub & API server to a production environment.

## 1. Prerequisites

- **OS**: Ubuntu 22.04 LTS or similar Linux distribution.
- **Go**: v1.21+
- **Database**: Supabase (PostgreSQL) account.
- **Cache**: Redis instance (local or managed).
- **Blockchain**: Peaq L1 Node (local or RPC).
- **Geolocation**: `GeoLite2-City.mmdb` file in the `/server` directory.

## 2. Environment Setup

1. Copy the template: `cp .env.example .env`
2. Update the following critical variables:
    - `SUPABASE_URL`
    - `PEAQ_RPC` (L1 Connection)
    - `PEAQ_ORACLE_SEED` (Substrate compatible seed)
    - `NODE_SECRET` (For node handshakes)
    - `PROXY_SECRET` (For admin-to-server auth)

## 3. Database Migrations

Apply migrations in order (001 to 022).
You can use the Supabase SQL Editor or a CLI tool:
```bash
# Example using psql
for f in migrations/*.sql; do psql $DATABASE_URL -f $f; done
```

## 4. Building the Server

```bash
cd server
go mod download
go build -o exra-server main.go
```

## 5. Systemd Service Setup

Create `/etc/systemd/system/exra.service`:
```ini
[Unit]
Description=Exra Network Hub
After=network.target

[Service]
User=www-data
Group=www-data
WorkingDirectory=/var/www/exra/server
ExecStart=/var/www/exra/server/exra-server
Restart=always
EnvironmentFile=/var/www/exra/server/.env

[Install]
WantedBy=multi-user.target
```

## 6. Security Checklist

- [x] **Admin Auth**: Derived from individual API keys in `admin_users` table.
- [x] **Rate Limiting**: Enabled for HTTP (IP-based) and WebSocket (Message-based).
- [x] **Headers**: Ensure `X-Forwarded-For` is correctly passed by Nginx.
- [ ] **Firewall**: Open port 8080 (API/WS) and 80/443 (Nginx).

## 7. Monitoring

- Logs: `journalctl -u exra -f`
- Metrics: `http://localhost:8080/metrics` (Prometheus format)
- Health: `http://localhost:8080/health`
