# Exra Protocol Economy Spec

## 1. Scope

This document is the canonical source of truth for Exra protocol economics and settlement.
If any other document conflicts with this file, this file wins.

## 2. Immutable Policy Targets

- `token_symbol`: `EXRA`
- `premine`: `0`
- `max_supply`: `1_000_000_000` EXRA
- `epoch_policy`: supply-based halving each `100_000_000` minted EXRA
- `halving_factor`: `0.5` per epoch transition
- `tail_emission`: `0`
- `policy_finalized`: immutable in production (`EXRA_POLICY_FINALIZED=true`)

## 3. Reward Model

Exra has two reward streams:

1. Presence rewards (PoP): paid for stable online participation.
2. Usage rewards: paid for verified traffic/compute execution.

### 3.1 Presence Reward

- Heartbeat target interval: 5 minutes.
- Base PoP credit per valid heartbeat: `0.00005` credits.
- Effective reward multiplier is derived from reputation score (`RS_mult`).

### 3.2 Usage Reward

- Base traffic price baseline: `$0.30` per verified GB.
- Settlement is always based on verified usage and reputation multiplier.

`usage_credits = (verified_bytes / 1e9) * 0.30 * RS_mult`

## 4. Identity Tiers and Payout Gates

Anon:
- No stake required.
- 24h timelock after batch settlement.
- 25% treasury tax.

Peak:
- Requires stake + verified credential.
- No timelock.
- No anon tax.

## 5. Oracle Batch and Consensus

Daily batch flow:

1. Oracles collect signed usage/heartbeat evidence.
2. At least 2 of 3 oracles must confirm no fraud.
3. Fraudulent credits are burned or excluded before mint.
4. Batch mint is submitted on-chain for validated DIDs.

## 6. Anti-Fraud Guarantees

- Canary checks must invalidate fake-work paths.
- Feeder audits must be multi-party and cross-subnet.
- Repeated fraud may trigger slashing and DID revocation.

## 7. On-Chain Enforcement Requirements

Contract-side guards:
- cap enforcement (`minted_total + amount <= max_supply`)
- epoch schedule enforcement
- post-finalization economics mutation disabled
- oracle key rotation allowed without changing economics

## 8. Trust Surface (Public Metrics)

- minted total vs cap
- current epoch and remaining mint budget
- policy finalized flag
- oracle address and rotation history
- dispute and batch settlement status

## 9. Non-Negotiable Invariants

- No settlement without verified usage.
- No mint outside policy bounds.
- No silent economics changes after policy finalization.
- No reward path that bypasses DID and oracle attestations.
