# TON Policy Lock Notes

This document describes how Exra prevents uncontrolled emission in runtime and contract adaptation.

## Runtime Guards (Current Server)

`server/ton/mint.go` enforces:

- total minted guard against `EXRA_MAX_SUPPLY`,
- epoch budget guard against halving schedule,
- first halving timestamp enforcement (`EXRA_FIRST_HALVING_AT`),
- rejection path with explicit `policy_guard: cap_or_epoch_budget_exceeded`.

## Contract Adaptation Target

The TON Jetton minter adaptation must mirror the same limits:

- `max_supply`,
- `first_halving_at`,
- per-epoch budget and per-epoch minted accounting,
- one-way `policy_finalized`,
- oracle rotation without economics mutation.

## Verification

Operators should continuously monitor:

- minted total vs cap,
- epoch minted vs epoch budget,
- rejected mints due to policy guard.
