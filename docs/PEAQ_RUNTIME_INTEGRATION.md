# peaq Runtime Integration Patch-Pack — `pallet-exra` v2.4.1

**Audience:** peaq runtime engineers integrating `pallet-exra` into the
production `peaq-node` / `peaq-parachain-runtime`.

**Scope:** copy-paste deltas for `Cargo.toml`, `construct_runtime!`,
`Config` impls, `UsdtHandler` adapter, genesis, and mainnet migration.

**Reference implementation:** `peaq/runtime/` in this repo is a working
test-harness runtime that proves end-to-end behavior (see
`peaq/runtime/src/tests.rs` — 5 passing integration tests covering
`fund_vault`, `exchange_exra_to_usdt`, and vault-insolvency paths).

---

## 0. Prerequisites

- peaq runtime must already include `pallet-assets` with `AssetId = u32`.
- The USDT asset must exist (wormhole-bridged USDT on peaq). Note its
  on-chain `AssetId` — that value replaces `USDT_ASSET_ID` below.
- Native balance pallet must already implement `fungible::{Inspect,
  Mutate, MutateHold}` with a `RuntimeHoldReason` composite enum.

---

## 1. `Cargo.toml` additions

In the runtime's `Cargo.toml`:

```toml
[dependencies]
# ...existing deps...
pallet-exra = { path = "../pallets/exra", default-features = false }

[features]
std = [
    # ...existing...
    "pallet-exra/std",
]
runtime-benchmarks = [
    # ...existing...
    # "pallet-exra/runtime-benchmarks",  # uncomment once benches are added
]
try-runtime = [
    # ...existing...
    # "pallet-exra/try-runtime",
]
```

---

## 2. `construct_runtime!` entry

```rust
construct_runtime!(
    pub enum Runtime {
        // ...System, Balances, Assets, ...existing pallets...
        Exra: pallet_exra = 99, // choose an unused pallet index
    }
);
```

`pallet-exra` declares `#[pallet::composite_enum] pub enum HoldReason`
for `PeakStake`. `construct_runtime!` auto-wires it into the runtime's
`RuntimeHoldReason`.

---

## 3. USDT AssetId — **CHANGE FOR MAINNET**

```rust
/// TODO(mainnet): replace with the canonical wormhole-bridged USDT
/// asset id once wormhole deployment is frozen.
pub const USDT_ASSET_ID: u32 = 31337;

parameter_types! {
    pub const ExraTreasuryPalletId: PalletId = PalletId(*b"exra/trs");
    pub const ExraVaultPalletId: PalletId = PalletId(*b"exra/vlt");
    pub const ExraUsdtAssetId: u32 = USDT_ASSET_ID;
    /// Genesis hash binds oracle signatures to *this* chain; domain
    /// separation against cross-chain signature replay.
    pub ExraChainGenesisHash: H256 = H256(
        frame_system::Pallet::<Runtime>::block_hash(0u32.into()).into()
    );
}
```

> **Warning.** `ExraChainGenesisHash` must resolve to a stable, chain-
> unique value at runtime construction. If you cannot evaluate
> `block_hash(0)` at that point, hardcode the genesis hash once the
> chainspec is frozen and re-deploy with a migration. Oracle clients
> must sign over this same value — a mismatch rejects every oracle call.

---

## 4. `UsdtBridge` adapter

Paste verbatim into the runtime crate (e.g., `runtime/src/exra_bridge.rs`
or inline in `lib.rs`):

```rust
use frame_support::traits::{
    fungibles::{Inspect as FungiblesInspect, Mutate as FungiblesMutate},
    tokens::Preservation,
};

pub struct UsdtBridge;

impl UsdtBridge {
    fn vault_account() -> AccountId {
        <pallet_exra::Pallet<Runtime>>::vault_account()
    }
}

impl pallet_exra::UsdtHandler<AccountId, Balance> for UsdtBridge {
    fn vault_balance() -> Balance {
        <Assets as FungiblesInspect<AccountId>>::balance(
            USDT_ASSET_ID,
            &Self::vault_account(),
        )
    }
    fn pay_from_vault(to: &AccountId, amount: Balance) -> sp_runtime::DispatchResult {
        <Assets as FungiblesMutate<AccountId>>::transfer(
            USDT_ASSET_ID,
            &Self::vault_account(),
            to,
            amount,
            Preservation::Expendable,
        ).map(|_| ())
    }
    fn deposit_to_vault(from: &AccountId, amount: Balance) -> sp_runtime::DispatchResult {
        <Assets as FungiblesMutate<AccountId>>::transfer(
            USDT_ASSET_ID,
            from,
            &Self::vault_account(),
            amount,
            Preservation::Expendable,
        ).map(|_| ())
    }
}
```

`Preservation::Expendable` is correct here: the vault sub-account holds
no existential consumer bond on the USDT asset, so keep-alive is not
required.

---

## 5. `pallet_exra::Config` impl

```rust
impl pallet_exra::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type Currency = Balances;
    type RuntimeHoldReason = RuntimeHoldReason;
    type Usdt = UsdtBridge;
    type PalletIdTreasury = ExraTreasuryPalletId;
    type PalletIdVault = ExraVaultPalletId;
    type UsdtAssetId = ExraUsdtAssetId;
    type ChainGenesisHash = ExraChainGenesisHash;
}
```

---

## 6. Genesis — 1B EXRA pre-funded treasury

`pallet-exra` follows a **distribution model**: the full 1 000 000 000
EXRA supply is pre-minted to the treasury sub-account at genesis and
never minted again. `batch_mint` moves native balance from treasury to
pending; it cannot inflate supply past `HARD_CAP_PLANCKS`.

In the runtime's `GenesisConfig` constructor (chainspec builder):

```rust
GenesisConfig {
    // ...
    exra: pallet_exra::GenesisConfig {
        // 1_000_000_000 EXRA * 10^9 plancks
        treasury_plancks: 1_000_000_000_000_000_000u128,
        ..Default::default()
    },
    // ...
}
```

The pallet's `build()` function mints this amount directly to the
treasury sub-account (derived from `PalletIdTreasury`) via
`Currency::mint_into`. After `finalize_protocol`, further mint calls
are impossible — every supply-increasing path is gated on
`!IsFinalized`.

---

## 7. Post-deploy bootstrap (one-shot, pre-finalize)

Execute in this order as `sudo` (or via the governance track that owns
root):

1. `exra.set_oracles(vec![oracle_pub_1, .., oracle_pub_N])` — `N ∈ [3, 7]`.
2. One or more `exra.submit_price(nonce, price, signatures)` calls to
   seed the price feed (required for `finalize_protocol`).
3. `exra.finalize_protocol()` — **one-way**. After this call:
   - `set_oracles` is permanently disabled.
   - The only admin extrinsic still callable is `fund_vault`.

---

## 8. Ongoing operations — `fund_vault`

`fund_vault(origin: root, from: AccountId, amount: Balance)` is the
canonical path for B2B providers to top up USDT liquidity for the
exchange leg. It is:

- Root-only (governance-gated on peaq production).
- Callable **post-finalize** (by design — vault top-ups are the only
  mutable surface after finalize).
- A pure wrapper over `UsdtHandler::deposit_to_vault`, which transfers
  USDT from `from` → vault sub-account via `pallet-assets`.

The B2B provider must approve (or otherwise authorize) the asset
transfer. In the reference runtime we mint directly to the provider,
then call `fund_vault`; on mainnet, the provider's account must hold the
USDT and the governance proposal must quote the exact `amount`.

---

## 9. Oracle client notes (off-chain component)

The oracle signs **domain-separated commitments** over full payloads.
Any off-chain oracle implementation MUST reproduce this exactly:

```rust
// batch_mint
msg = blake2_256(&(
    b"exra/batch-mint/v1",   // pallet_exra::types::domain::BATCH_MINT
    chain_genesis_hash,
    batch_id,                // H256
    blake2_256(&claims.encode()),
).encode());

// submit_price
msg = blake2_256(&(
    b"exra/submit-price/v1", // pallet_exra::types::domain::SUBMIT_PRICE
    chain_genesis_hash,
    nonce,                   // u64
    price,                   // u128
).encode());

// update_stats, slash_node — see types::domain
```

Signatures are sr25519 over this `msg`. The on-chain call accepts
`Vec<(oracle_idx, [u8; 64])>` where `oracle_idx` is the oracle's
position in `OracleSet`. Duplicate indices are rejected.

---

## 10. Upgrade & migration checklist

- [ ] Confirm `pallet-assets` `AssetId = u32` (same as `USDT_ASSET_ID` type).
- [ ] Confirm USDT wormhole asset is created and has adequate decimals
      matching the 10^9 plancks convention used in price/exchange math.
      Mismatched decimals will silently distort every exchange rate —
      scale in the oracle client if needed.
- [ ] Add `pallet-exra` at a stable index in `construct_runtime!`.
- [ ] Verify `RuntimeHoldReason` compiles — `construct_runtime!`
      auto-expands `HoldReason::Exra(pallet_exra::HoldReason::PeakStake)`.
- [ ] Chainspec injects `treasury_plancks = 1e18`.
- [ ] First block after genesis: run the post-deploy bootstrap in §7.
- [ ] Finalize only after oracle keys + price feed are confirmed live.
- [ ] Governance track documented for `fund_vault` (the only
      ongoing-admin extrinsic).

---

## 11. What this pallet **deliberately does not** provide

- No proxy admin, no "emergency mint", no "pause". After
  `finalize_protocol`, economic parameters are frozen code.
- No off-chain worker. Oracle logic lives entirely off-chain; on-chain
  code only verifies signatures and replay tokens.
- No governance hooks for parameter updates. The supply cap, epoch
  schedule, reward constants, and slash bps are compile-time
  constants — changing them requires a runtime upgrade.

These are the security invariants; reviewers should treat any change
that weakens them as a protocol fork, not a bug fix.
