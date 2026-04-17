# Exra Economy & Rewards

Canonical protocol policy is defined in `docs/PROTOCOL_ECONOMY_SPEC.md`.

## Recent Change Notes (2026-04)

- Hard-cap token policy fixed at `1_000_000_000 EXRA` with explicit halving schedule and policy finalization flag.
- Managed marketplace mechanics implemented: worker pricing (`price_per_gb`/`auto_price`), buyer offers, deterministic oracle assignment, and locked session price.
- Oracle queue upgraded for production operations: retries/backoff, DLQ reason tracking, and manual retry path.
- Instant swap safety added with circuit breaker to pause execution during abnormal volatility.
- Smoke verification performed locally for end-to-end API path; real TON mainnet execution remains a separate production run.

## 1. Proof of Presence (PoP) Heartbeat Model

Exra is transitioning from a purely traffic-based reward generation system to a deterministic **Proof of Presence (PoP)** heartbeat model. 

Tokens are emitted continuously based on stable connection and presence in the network, independent of the actual gigabytes routed through the device. This provides a baseline passive income for all active workers.

### Heartbeat Ticks
Every heartbeat (target: once every 5 minutes), the server calculates a fixed `total_emission` for the node (configured via `POP_EMISSION_PER_HEARTBEAT`). 

This emission is immediately split into **three independent streams**:
1. **Worker Reward**: The device running the node.
2. **Referral Reward**: The person who invited the worker.
3. **Treasury Reward**: The Exra protocol treasury.

## 2. The 3-Stream Split Formulas

The heartbeat emission is strictly divided based on the following invariants:

* **Worker**: Always receives exactly **50%** of the `total_emission`.
* **Referral**: Receives between **10% and 30%** of the `total_emission`, depending on their tier.
* **Treasury**: Receives the remainder.
  * If the worker has a referrer: `Treasury = 100% - 50% - Referral%`
  * If the worker has **no** referrer: `Treasury = 100% - 50% = 50%`

> [!IMPORTANT]  
> **Fairness Guarantee**: The worker always gets their 50%. The referrer's reward is paid from the *overall emission pool*, not deducted from the worker. Nobody takes anything away from anyone.

## 3. Referral Tiers

The referral percentage is calculated dynamically based on the number of active referrals the referrer has brought into the network. 

We use a 4-level inclusive scale:

| Level | Rank | Active Referrals Required | Percentage Reward |
|-------|------|---------------------------|-------------------|
| Lvl 1 | Street Scout | 1 – 100 | **10%** |
| Lvl 2 | Network Builder | 101 – 300 | **15%** |
| Lvl 3 | Crypto Boss | 301 – 600 | **20%** |
| Lvl 4 | Ambassador | 601 – 1000 | **30%** |

*(Note: Maximum referral reward is capped at 30%, guaranteeing the Treasury always receives at least 20%).*

## 4. Idempotency and Anti-Abuse
Heartbeat rewards are guarded by an idempotency key tied to the `device_id` and a 30-second time window. If a node sends duplicate heartbeats or `pong` responses within the same 30-second window, they are inherently ignored by the database.

## 5. Auditability
Every PoP distribution is recorded in the `pop_reward_events` ledger. Each record explicitly states the `total_emission`, the exact splits applied, the `referrer_device_id`, and a JSON snapshot of the distribution policy at that exact moment in time (`policy_snapshot`).
