# pallet-exra v2.4.1 — Production-Immutable Spec (Fixed)

**Status:** Supersedes `EXRA v2.4 «Century Edition»`. Fixes 14 spec-level bugs identified in security audit. **Source of truth for implementation.**

**Design goals (in priority order):**
1. Zero admin mint after `finalize_protocol` — cryptographically enforced, not honor-system.
2. No replay, no payload-swap, no double-counting of oracle signatures.
3. Overflow-safe arithmetic (U256 intermediate, saturating final).
4. Bounded iteration everywhere (no DoS via batch size or cleanup).
5. Hard cap 1B EXRA enforced **at mint site**, not in spec comments.
6. Fail-closed: if oracle feed stale, exchange blocked. If `IsFinalized=false`, mint blocked.

---

## 0. Runtime Integration Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Balance API | `fungible::Mutate` + `fungible::hold::Mutate` | Native `burn_from`, no imbalance juggling, hold for stake = slashable without burning. |
| Admin origin | `EnsureRoot` | Governance/sudo gate; no custom owner storage → one less attack surface. |
| Block time | 12s (peaq) → `DAILY_BLOCKS = 7_200` | Used only for claim timelock & price staleness window. |
| Price oracle | On-chain `submit_price` extrinsic (oracle multisig, per-submission nonce, staleness + deviation guard) | No off-chain trust; exchange fails closed when stale. |
| PalletId | `PalletId(*b"exra/pal")` → treasury sub-account; `PalletId(*b"exra/vlt")` → USDT vault sub-account | Deterministic, no admin key. |
| Max claims per batch | 512 | Fits block weight at 12s; oracles chunk beyond. |
| Max oracles | 7 (`ORACLE_SET_MAX`) | Enough for 5-of-7; small enough for on-chain sig verify loop. |
| Oracle threshold | `ceil(2 * N / 3)`, minimum 2 (enforced at `set_oracles`: N ≥ 3) | True 2/3 supermajority; no single-oracle path. |

---

## 1. Breathe Model (Math, Fixed)

### 1.1 Formula

```
base        = R_uptime + R_work                                   [u128, plancks]
R_uptime    = heartbeats × HEARTBEAT_REWARD
R_work      = gb_verified × GB_RATE                               (gb_verified in whole GB)
reward_pre  = base × RS_mult × Epoch_mult × Tier_mult             [U256 intermediate]
reward      = saturating_into::<Balance>(reward_pre)
```

- **RS_mult = `Permill::from_rational(min(GS, 1000).saturating_mul(2), 1_000)`** — GS∈[0,1000] ⇒ RS_mult∈[0, 2.0]. GS=500 ⇒ 1.0.
- **Epoch_mult = `Permill` starting at `Permill::one()`**, halved each epoch transition via `mult = mult / 2`. Never reaches zero (floor at `Permill::from_parts(1)`; past that, emission effectively stops — **intended behavior**).
- **Tier_mult** = `Peak → Permill::one()`, `Anon → Permill::from_percent(75)`. **No separate tax.** Double-taxation bug from v2.4 removed.
- **GB_RATE** = `300_000_000` (0.30 EXRA with 9 decimals, per GB).
- **HEARTBEAT_REWARD** = `50_000` (0.00005 EXRA per 5-min heartbeat).

### 1.2 Overflow Safety

All reward math runs through `U256`:

```rust
let pre = U256::from(base)
    .saturating_mul(rs_mult.deconstruct().into())      // ≤ 2_000_000
    .saturating_mul(epoch_mult.deconstruct().into())   // ≤ 1_000_000
    .saturating_mul(tier_mult.deconstruct().into())    // ≤ 1_000_000
    / U256::from(1_000_000u128.pow(3));                // denormalize 3 × Permill
let reward: Balance = pre.saturated_into();
```

Max worst-case: `base ≤ 2^96` leaves > 2^160 headroom → no overflow. Final `saturated_into` clamps to `Balance::MAX` (never panics).

### 1.3 Hard Cap Check

Cap check **before** mint, on **sum of net**:

```rust
let total_net: Balance = claims.iter().map(|c| c.net).try_fold(0, Balance::checked_add)?;
ensure!(
    CirculatingSupply::<T>::get().checked_add(total_net).ok_or(ArithmeticError::Overflow)? <= HARD_CAP,
    Error::<T>::HardCapExceeded
);
```

If the cap would be breached, **the entire batch reverts** — no partial mint.

---

## 2. Storage

```rust
CirculatingSupply   : Balance                                                    // ≤ HARD_CAP
OracleSet           : BoundedVec<AccountId, ConstU32<ORACLE_SET_MAX>>            // ≥3, ≤7
NodeStats           : Map<AccountId, NodeStat>                                   // uptime, gb, gs
Tier                : Map<AccountId, TierType>                                   // Anon(default) | Peak
EpochInfo           : Epoch { current: u32, mult: Permill, minted_net: Balance }
PendingClaims       : DoubleMap<AccountId, BatchId, ClaimInfo { net, unlock_block }>
UsedBatchIds        : Map<BatchId, ()>                                           // replay guard
LastClaimBlock      : Map<AccountId, BlockNumber>                                // velocity 1/24h
IsFinalized         : bool                                                       // one-way → true
PriceFeed           : PriceEntry { usdt_per_exra_e9: u128, submitted_at: BlockNumber }
UsedPriceNonces     : Map<u64, ()>
StakeLedger         : Map<AccountId, Balance>                                    // Peak stake amount
BurnHistory         : Map<(AccountId, u64), BurnRecord>                          // audit trail
BurnCounter         : Map<AccountId, u64>                                        // BurnHistory key
SlashRecords        : Map<AccountId, SlashRecord>                                // canary failures
```

**BatchId** = `H256` (blake2_256 hash; oracles pick it; `UsedBatchIds` prevents reuse).

---

## 3. Signature Binding (Anti-Replay + Anti-Swap)

Oracle signs a **commitment over the full payload**, not just an id:

```rust
fn mint_message(batch_id: H256, claims: &[Claim], chain_genesis: H256) -> [u8; 32] {
    let claims_root = blake2_256(&claims.encode());
    blake2_256(&(
        b"exra/batch_mint/v1",
        chain_genesis,
        batch_id,
        claims_root,
    ).encode())
}
```

Same pattern for `update_stats`, `submit_price`, `slash_node`. Each message type has a **unique domain-separator** (`exra/batch_mint/v1`, `exra/update_stats/v1`, …) → no cross-protocol reuse.

**Signature vector processing:**
1. Accept `Vec<(oracle_index, Signature)>` (indices into `OracleSet`), not AccountId.
2. Track `seen: BTreeSet<u8>` — **reject duplicate indices**.
3. Verify each against `OracleSet[index]` pubkey → message.
4. Require `valid_count >= threshold`.
5. Mark `UsedBatchIds[batch_id] = ()` (or `UsedPriceNonces[nonce]`).

Fixes: payload-swap, replay, duplicate-sig-counting — all closed.

---

## 4. Extrinsics

### 4.1 `update_stats` (Oracle multisig)
Writes `NodeStats`. Same sig-binding as `batch_mint`. Uses its own `batch_id`. Bounded vec of `(AccountId, NodeStat)`, max 512.

### 4.2 `batch_mint` (Oracle multisig)
- `ensure!(IsFinalized::<T>::get(), NotFinalized)` — **mint refuses before finalization.**
- Verify multisig over `mint_message`.
- Reject if `UsedBatchIds` contains id.
- For each claim: look up `NodeStats`, compute reward per §1, apply tier, verify sum ≤ cap.
- Mint from `PalletExraAccount` to self (pre-reserve not required — deposit directly to treasury), record `PendingClaims` with `unlock_block = now + timelock(tier)`.
- Update `CirculatingSupply`, `EpochInfo` (halve when `minted_net ≥ EPOCH_SIZE`, rollover remainder).
- Timelock: Peak = 0 blocks (instant), Anon = `DAILY_BLOCKS` (24h).

### 4.3 `claim`
- Load `PendingClaims`, check `now ≥ unlock_block`.
- Anti-spam: `now - LastClaimBlock[did] ≥ DAILY_BLOCKS / 24` (≥ 1h between claims, hard rate limit).
- Transfer from treasury sub-account to user via `fungible::Mutate::transfer`.
- Remove `PendingClaims` entry.

### 4.4 `exchange_exra_to_usdt`
- Load `PriceFeed`; reject if `now - submitted_at > PRICE_STALENESS` (600 blocks = 2h).
- `usdt_gross = exra_amount × price / 1e9`.
- `spread = usdt_gross × VAULT_SPREAD_BPS / 10_000` (2%).
- `payout = usdt_gross - spread`; ensure `VaultUsdt ≥ payout`.
- `fungible::Mutate::burn_from(did, exra_amount)` — **real burn.**
- `CirculatingSupply -= exra_amount` (saturating).
- USDT transfer from vault sub-account to user via `pallet_assets`.
- Record `BurnHistory[(did, counter)]`, increment counter, emit event with **payload hash**.

### 4.5 `submit_price` (Oracle multisig)
- Nonce in payload; reject if in `UsedPriceNonces`.
- Message = `hash("exra/submit_price/v1", chain_genesis, nonce, price)`.
- **Deviation guard:** if previous price exists, reject if `|new - prev| / prev > 20%` **unless** supermajority is unanimous (all N oracles, not 2/3).
- Write `PriceFeed`, mark nonce used.

### 4.6 `stake_for_peak`
- `fungible::hold::Mutate::hold(HoldReason::PeakStake, did, PEAK_STAKE)` — actual hold, not burn.
- `StakeLedger[did] = PEAK_STAKE`.
- `Tier[did] = Peak`.

### 4.7 `unstake_from_peak`
- Require `NodeStats[did].gs >= MIN_UNSTAKE_GS` (e.g., 100) to prevent abusers cashing out after slash.
- 7-day cooldown queue (`PendingUnstake`) — prevents gaming tier right before claim.
- Release hold, `StakeLedger[did] = 0`, `Tier[did] = Anon`.

### 4.8 `slash_node` (Oracle multisig)
- Canary failure → `NodeStats[did].gs = 0`, slash `5% of held stake` via `fungible::hold::Mutate::burn_held`.
- `SlashRecords[did]` appended.

### 4.9 Admin (Root, blocked after finalize)
- `set_oracles(Vec<AccountId>)` — require `N ∈ [3, 7]`; reject if `IsFinalized`.
- `fund_treasury(amount)` / `fund_vault(amount)` — prefunding; reject if `IsFinalized` **for fund_treasury** (vault can be topped up post-finalize — B2B USDT inflow is expected).
- `finalize_protocol()` — preconditions: `OracleSet.len() ≥ 3`, `PriceFeed.submitted_at > 0`, treasury pre-funded to ≥ `HARD_CAP`. Sets `IsFinalized=true`. **Irreversible.**

---

## 5. Hooks

```rust
fn on_initialize(n: BlockNumber) -> Weight {
    // Bounded cleanup: process at most CLEANUP_PER_BLOCK entries per block via cursor
    CleanupCursor::<T>::mutate(|cur| {
        for _ in 0..CLEANUP_PER_BLOCK {
            // drain expired PendingClaims older than CLAIM_TTL (e.g., 30 days)
            // using storage iterator resumed from `cur`.
        }
    });
    weight
}
```

No `on_finalize` allocation scaling with storage size.

---

## 6. Events (Forensic-Grade)

All mint/burn/slash events carry **payload hash** so an indexer can reconstruct without trusting storage reads alone:

```rust
BatchMinted { batch_id: H256, payload_hash: [u8; 32], total_net: Balance, epoch: u32 }
ClaimSettled { did, batch_id, amount }
Exchanged { did, exra_burned, usdt_paid, price, burn_seq }
PriceSubmitted { nonce: u64, price: u128, deviation_bps: u16 }
Slashed { did, amount_burned, reason_hash }
ProtocolFinalized { oracle_set_hash: [u8; 32], treasury_balance: Balance }
```

---

## 7. Errors (Exhaustive)

```
NotFinalized, AlreadyFinalized, NotEnoughOracles, InvalidOracleIndex,
DuplicateOracleSignature, InsufficientOracleConsensus, ReplayedBatchId,
ReplayedPriceNonce, HardCapExceeded, OverflowInRewardCalc, NoStatsForAccount,
TimelockActive, VelocityLimit, NoPendingClaim, PriceFeedStale, PriceDeviationTooHigh,
VaultInsufficientUsdt, TreasuryUnderfunded, StakeAlreadyHeld, StakeNotHeld,
UnstakeCooldownActive, GsTooLowForUnstake, InvalidOracleSetSize
```

---

## 8. Test Matrix (Mandatory)

Every attack vector from the v2.4 audit must have a test that **fails without the fix and passes with it**:

1. `batch_mint_before_finalize_rejected`
2. `replay_same_batch_id_rejected`
3. `duplicate_oracle_sig_in_same_batch_rejected`
4. `payload_swap_with_valid_sigs_rejected` ← critical
5. `threshold_math_for_n3_requires_2`, `for_n5_requires_4`, `for_n7_requires_5`
6. `hard_cap_breach_reverts_whole_batch`
7. `reward_overflow_saturates_not_panics`
8. `anon_tier_applies_0_75_no_double_tax`
9. `epoch_halving_at_100m_net`
10. `stake_is_held_not_burned` (balance-before = balance-after-unstake)
11. `slash_burns_5_percent_of_held_not_free_balance`
12. `exchange_fails_when_price_stale`
13. `submit_price_rejects_20_percent_deviation_below_unanimous`
14. `finalize_requires_prefunded_treasury_and_oracles_and_price`
15. `admin_setters_blocked_post_finalize` (sudo cannot unlock mint)
16. `claim_timelock_enforced_anon_24h`
17. `velocity_1_per_hour_enforced`
18. `cleanup_bounded_per_block` (weight test)

---

## 9. Out of Scope (v2.4.1)

- XCM / cross-chain mint (no v2.4 requirement).
- On-chain governance of oracle set changes post-finalize (by design: protocol is frozen).
- ZK proof verification for heartbeats (Sentinel layer is off-chain + slash via `slash_node`).

---

## 10. Invariants (Must Hold Always)

- `CirculatingSupply ≤ HARD_CAP`
- `sum(StakeLedger) == held balance of pallet under HoldReason::PeakStake`
- `IsFinalized: false → true` is a **one-way transition**; no extrinsic writes `false`.
- `UsedBatchIds` / `UsedPriceNonces` entries are **never deleted** (forensic trail).
- After `finalize_protocol`: no code path mutates `OracleSet`, `HARD_CAP` constant, reward constants, or `IsFinalized`.

---

*End of spec. Ready for implementation.*
