# Exra Protocol Economy Spec

## 1. Scope

This document is the canonical source of truth for Exra protocol economics and settlement.
If any other document conflicts with this file, this file wins.

## 2. Immutable Policy Targets

- `token_symbol`: `EXRA`
- `premine`: `0`
- `max_supply`: `1_000_000_000` EXRA
- `epoch_duration`: `12 months`
- `halving_factor`: `0.5`
- `tail_emission`: `0`
- `first_halving_at`: `2027-01-01T00:00:00Z`
- `policy_finalized`: must become `true` in production and cannot be reverted

## 3. Reward Model

Exra has two reward streams:

1. Presence rewards (PoP): paid for stable online participation.
2. Usage rewards: paid for verified traffic/compute execution.

### 3.1 Presence Reward

- Heartbeat target interval: 5 minutes.
- Each valid heartbeat produces `total_emission` and splits:
  - Worker: 50%
  - Referrer: 10-30% (tier-driven)
  - Treasury: remainder, never below 20%

### 3.2 Usage Reward

- Worker sets `price_per_gb` or enables `auto_price`.
- Oracle locks `locked_price_per_gb` at assignment time.
- Settlement is always based on verified usage and locked price:

`usage_reward_exra = (verified_bytes / 1e9) * locked_price_per_gb * worker_share`

## 4. Offer and Settlement Lifecycle

1. Buyer creates offer with budget/constraints.
2. Protocol reserves budget in EXRA accounting units.
3. Oracle selects nodes by deterministic rule:
   - lower effective price first,
   - then higher reliability score.
4. Session runs and usage is verified.
5. Settlement writes immutable ledger events.
6. Treasury and worker balances update atomically.

## 5. Oracle Determinism and Failover

- Oracle must produce deterministic assignment from same input state.
- Mid-session node failure triggers transparent failover:
  - session continues on replacement node,
  - buyer-facing continuity is preserved,
  - settlement attributes usage by segment.

## 6. Instant Swap

Supported rails for V1:
- EXRA -> USDT on TON
- EXRA -> TON on TON

Swap execution is treasury-backed with spread.

### 6.1 Circuit Breaker

Swap must pause temporarily when abnormal DEX volatility is detected to protect treasury from negative execution.
Minimum controls:
- max slippage threshold,
- stale-quote timeout,
- short cool-down window before retry.

## 7. On-Chain Enforcement Requirements

Contract-side guards:
- cap enforcement (`minted_total + amount <= max_supply`)
- epoch budget enforcement
- halving schedule enforcement by epoch
- `first_halving_at` stored in state
- post-finalization economics mutation disabled
- oracle key rotation allowed without changing economics

## 8. Trust Surface (Public Metrics)

- minted total vs cap
- current epoch, epoch budget, remaining epoch mint
- policy finalized flag
- oracle address and rotation history
- swap pause status

## 9. Non-Negotiable Invariants

- No settlement without verified usage.
- No mint outside policy bounds.
- No silent economics changes after policy finalization.
- No buyer-visible failure on single node drop if failover path is available.
