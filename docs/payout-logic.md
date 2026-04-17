# Payout Calculation Logic (TON Native)

This document defines the mathematical models and verification rules for user payouts in the Exra Network using the TON blockchain.

## 1. Core Logic

Payouts are triggered via the `/api/payout/request` endpoint. Before execution, the server performs a **Precheck** to ensure the user has sufficient funds to cover network fees.

### Fee Breakdown
- **ton_gas_fee**: The variable cost of executing the transfer and mint operations on the TON blockchain.
- **storage_fee**: The mandatory TON fee for creating or maintaining the recipient's Jetton wallet (replacing legacy Solana ATA Rent).
- **total_fee_ton**: `ton_gas_fee + storage_fee`

## 2. Calculation Formula

```go
// US Dollar values for UX
TotalFeeUSD := (fees.GasFeeTON + fees.StorageFeeTON) * tonUSDPrice
NetAmountUSD := RequestedAmountUSD - TotalFeeUSD
```

### Constraints:
1. `NetAmountUSD` must be `> 0`.
2. `UserBalanceUSD` must be `>= RequestedAmountUSD`.
3. If `!WalletReady`, the `StorageFeeTON` must be included in the calculation to ensure the transaction doesn't fail on-chain.

## 3. Implementation Details

The implementation is split between the following components:
- [models/payout.go](file:///c:/Users/user/exra/server/models/payout.go) — Data structures and DB mapping.
- [handlers/payout.go](file:///c:/Users/user/exra/server/handlers/payout.go) — API logic and validation.
- [ton/client.go](file:///c:/Users/user/exra/server/ton/client.go) — Integration with TON center for fee estimation.

## 4. Anti-Abuse Rules

- **Idempotency**: Every payout request uses a unique `request_id`.
- **Approval Queue**: Payouts above a defined threshold (Default: $50) require manual admin approval.
- **Velocity Limit**: Maximum 1 withdrawal per 24 hours per device.

---
*Last Updated: April 2026. Reflected transition to TON Jettons (TEP-74).*
