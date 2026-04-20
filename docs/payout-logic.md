# Payout and Claim Logic (peaq DePIN)

This document defines payout gates and safety constraints for EXRA claims in the current peaq-based model.

## 1. Core Flow

1. Node earnings are accumulated off-chain from verified traffic/compute and PoP events.
2. Oracles run daily batch attestation and mint EXRA on-chain for validated DIDs.
3. User initiates claim (`/claim/{did}`) when balance is available.
4. Claim execution applies tier-dependent gates (tax and timelock).

## 2. Tier Rules

### Anon Tier

- Timelock: 24h after batch settlement.
- Tax: 25% treasury deduction.
- Velocity: max 1 payout per 24h per DID.

### Peak Tier

- Timelock: none.
- Tax: none.
- Velocity: max 1 payout per 24h per DID (unless explicitly changed in policy).

## 3. Safety Constraints

- No payout without prior oracle-attested credits.
- Balance checks and mutation must happen in one DB transaction (`SELECT ... FOR UPDATE` guard).
- Claims must be idempotent and auditable.
- Failed/disputed oracle batch must block dependent claims until resolved.

## 4. Implementation Anchors

- `server/handlers/payout.go` - request validation and claim endpoint behavior.
- `server/models/payout.go` - persistence and state transitions.
- `server/models/payout_markpaid_test.go` and related tests - payout lifecycle integrity.

## 5. Operational Notes

- Minimum payout target remains low-friction (`$1`) for user accessibility.
- Network/gas costs are reflected in claim UX and deducted according to current payout policy.
- Admin interventions must be logged in audit tables and linked to chain events where applicable.
