//! Type definitions for pallet-exra v2.4.1.
//!
//! All types are SCALE-encodable and live on-chain. Balance parameter is the
//! native EXRA balance unit. See `docs/PALLET_EXRA_V2_4_1_SPEC.md`.

use codec::{Decode, Encode, MaxEncodedLen};
use scale_info::TypeInfo;
use sp_core::H256;
use sp_runtime::{Permill, RuntimeDebug};

/// Node work statistics for one reward window. Written by oracles via
/// `update_stats`. Read by `batch_mint` when computing Breathe reward.
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq, Default)]
pub struct NodeStat {
	/// Count of 5-minute heartbeats observed this window.
	pub heartbeats: u32,
	/// Whole gigabytes of verified traffic this window.
	pub gb_verified: u32,
	/// Gear Score on 0..=1000 scale (2 decimal places effective).
	pub gs: u16,
}

/// User tier — decides `Tier_mult` and claim timelock.
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq)]
pub enum TierType {
	/// Anonymous: `Tier_mult = 0.75`, claim timelock = `DAILY_BLOCKS`.
	Anon,
	/// Staked node: `Tier_mult = 1.00`, claim timelock = 0.
	Peak,
}

impl Default for TierType {
	fn default() -> Self {
		TierType::Anon
	}
}

/// Current epoch state. Halving happens on transition when `minted_net`
/// crosses `EPOCH_SIZE`. `mult` is clamped at a residual floor (`EPOCH_MULT_FLOOR`).
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq)]
pub struct EpochInfo<Balance> {
	pub current: u32,
	pub mult: Permill,
	pub minted_net: Balance,
}

impl<Balance: Default> Default for EpochInfo<Balance> {
	fn default() -> Self {
		Self {
			current: 0,
			mult: Permill::one(),
			minted_net: Balance::default(),
		}
	}
}

/// A single payout inside a `batch_mint` call. `net` is the post-tier amount
/// computed by oracles and re-verified by the pallet from `NodeStats`.
#[derive(Clone, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq)]
pub struct Claim<AccountId, Balance> {
	pub account: AccountId,
	/// Oracles' proposed net reward. Pallet re-derives and requires equality.
	pub net: Balance,
}

/// Pending claim awaiting timelock expiry before user can withdraw.
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq)]
pub struct ClaimInfo<Balance, BlockNumber> {
	pub net: Balance,
	pub unlock_block: BlockNumber,
}

/// On-chain price feed entry: how many USDT (9 decimals) per 1 EXRA (9 decimals).
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq, Default)]
pub struct PriceEntry<BlockNumber> {
	pub usdt_per_exra_e9: u128,
	pub submitted_at: BlockNumber,
}

/// Immutable burn-history record for EXRA→USDT exchanges.
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq)]
pub struct BurnRecord<Balance, BlockNumber> {
	pub exra_burned: Balance,
	pub usdt_paid: Balance,
	pub price_at_burn: u128,
	pub at_block: BlockNumber,
}

/// Record of a slashing action taken against a node.
#[derive(Clone, Copy, Encode, Decode, MaxEncodedLen, TypeInfo, RuntimeDebug, PartialEq, Eq)]
pub struct SlashRecord<Balance, BlockNumber> {
	pub amount_burned: Balance,
	pub reason_hash: H256,
	pub at_block: BlockNumber,
}

/// Domain separators for oracle signatures. Prevents cross-message replay.
pub mod domain {
	pub const BATCH_MINT: &[u8] = b"exra/batch_mint/v1";
	pub const UPDATE_STATS: &[u8] = b"exra/update_stats/v1";
	pub const SUBMIT_PRICE: &[u8] = b"exra/submit_price/v1";
	pub const SLASH_NODE: &[u8] = b"exra/slash_node/v1";
}

// HoldReason is defined inside the pallet module as a composite_enum so that
// `construct_runtime!` auto-generates the matching `RuntimeHoldReason` variant.
