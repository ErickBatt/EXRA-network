# EXRA Network Deployment Runbook (v2.2)

This document provides step-by-step instructions for deploying the EXRA Network on the Peaq blockchain.

## 1. Prerequisites
- **Ubuntu 22.04+** recommended for server.
- **Go 1.21+**
- **Docker & Docker Compose**
- **Peaq Account** with $PEAQ for gas.
- **Substrate Keyring** (sr25519) for oracles.

## 2. Infrastructure Setup

### 2.1 Database (Supabase / Postgres)
1. Initialize a Postgres database.
2. Run migrations located in `/server/migrations/*.sql`.
3. Set your `SUPABASE_URL` to point to this instance.

### 2.2 Redis Cache
```bash
docker run -d --name exra-redis -p 6379:6379 redis:alpine
```

## 3. Server Deployment

### 3.1 Environment Configuration
1. Copy `production.env.example` to `.env`.
2. Generate secure secrets for `NODE_SECRET`, `ADMIN_SECRET`, and `PROXY_SECRET`.
3. Set `PEAQ_ORACLE_SEED` with your oracle account seed.

### 3.2 Running the Server
```bash
cd server
go mod download
go run .
```
Verify the server starts and connects to Peaq RPC.

## 4. Blockchain Setup (Peaq)

### 4.1 Pallet Deployment
1. Integrate `pallet_exra` into your Peaq node runtime.
2. Ensure the following constants are set:
   - `MinStake`: 100 EXRA
   - `AnonTax`: 25%

### 4.2 Oracle Registration
On the Peaq L1, call `PalletExra.add_oracle(oracle_account)` via Sudo or Governance for each of your 3 oracle accounts.

## 5. Dashboard Setup
1. `cd dashboard`
2. `npm install`
3. Set `.env.local` to point to the server API and Peaq RPC.
4. `npm run dev` or `npm run build`

## 6. Daily Operations

### Monitoring
- **Metrics**: Access `http://localhost:8081/metrics` (Control Plane).
- **Consensus**: Check `oracle_batches` table for `status = 'applied'`.

### Incident Response
- If a batch fails consensus, check `oracle_signatures` for mismatching hashes.
- Use `POST /api/admin/circuit-breaker` to pause minting if a farm is detected.

---
*EXRA Team - 2026*
