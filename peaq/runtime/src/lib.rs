//! Minimal integration runtime for pallet-exra + pallet-assets.
//!
//! This crate proves end-to-end that `pallet-exra` interoperates with the
//! real `pallet-assets` implementation for the USDT leg of
//! `exchange_exra_to_usdt` — it is not a production peaq node.
//!
//! Layout:
//! - `Runtime`: aggregates System/Balances/Assets/Exra.
//! - `UsdtBridge`: implements `pallet_exra::UsdtHandler` on top of
//!   `pallet_assets::Pallet<Runtime>` at `UsdtAssetId`.
//! - Tests in `tests.rs` exercise fund_vault → exchange → USDT payout.

#![cfg_attr(not(feature = "std"), no_std)]

#[cfg(test)]
mod tests;

use frame_support::{
	parameter_types,
	traits::{
		fungibles::{Inspect as FungiblesInspect, Mutate as FungiblesMutate},
		AsEnsureOriginWithArg, ConstU128, ConstU16, ConstU32, ConstU64,
	},
	PalletId,
};
use frame_system::{EnsureRoot, EnsureSigned};
use sp_core::H256;
use sp_runtime::{
	traits::{BlakeTwo256, IdentityLookup},
	DispatchResult,
};

pub type Block = frame_system::mocking::MockBlock<Runtime>;
pub type AccountId = sp_core::sr25519::Public;
pub type Balance = u128;
pub type AssetId = u32;

/// Canonical USDT asset id inside this runtime. Real peaq mainnet will
/// substitute the wormhole-bridged USDT id here.
pub const USDT_ASSET_ID: AssetId = 31337;

frame_support::construct_runtime!(
	pub enum Runtime {
		System: frame_system,
		Balances: pallet_balances,
		Assets: pallet_assets,
		Exra: pallet_exra,
	}
);

impl frame_system::Config for Runtime {
	type BaseCallFilter = frame_support::traits::Everything;
	type BlockWeights = ();
	type BlockLength = ();
	type DbWeight = ();
	type RuntimeOrigin = RuntimeOrigin;
	type RuntimeCall = RuntimeCall;
	type Nonce = u64;
	type Block = Block;
	type Hash = H256;
	type Hashing = BlakeTwo256;
	type AccountId = AccountId;
	type Lookup = IdentityLookup<AccountId>;
	type RuntimeEvent = RuntimeEvent;
	type BlockHashCount = ConstU64<250>;
	type Version = ();
	type PalletInfo = PalletInfo;
	type AccountData = pallet_balances::AccountData<Balance>;
	type OnNewAccount = ();
	type OnKilledAccount = ();
	type SystemWeightInfo = ();
	type SS58Prefix = ConstU16<42>;
	type OnSetCode = ();
	type MaxConsumers = ConstU32<16>;
}

impl pallet_balances::Config for Runtime {
	type MaxLocks = ConstU32<50>;
	type MaxReserves = ();
	type ReserveIdentifier = [u8; 8];
	type Balance = Balance;
	type RuntimeEvent = RuntimeEvent;
	type DustRemoval = ();
	type ExistentialDeposit = ConstU128<1>;
	type AccountStore = System;
	type WeightInfo = ();
	type FreezeIdentifier = ();
	type MaxFreezes = ();
	type RuntimeHoldReason = RuntimeHoldReason;
	type MaxHolds = ConstU32<4>;
}

parameter_types! {
	pub const AssetDeposit: Balance = 1;
	pub const AssetAccountDeposit: Balance = 1;
	pub const MetadataDepositBase: Balance = 1;
	pub const MetadataDepositPerByte: Balance = 1;
	pub const ApprovalDeposit: Balance = 1;
	pub const StringLimit: u32 = 50;
	pub const RemoveItemsLimit: u32 = 1000;
}

impl pallet_assets::Config for Runtime {
	type RuntimeEvent = RuntimeEvent;
	type Balance = Balance;
	type AssetId = AssetId;
	type AssetIdParameter = codec::Compact<AssetId>;
	type Currency = Balances;
	type CreateOrigin = AsEnsureOriginWithArg<EnsureSigned<AccountId>>;
	type ForceOrigin = EnsureRoot<AccountId>;
	type AssetDeposit = AssetDeposit;
	type AssetAccountDeposit = AssetAccountDeposit;
	type MetadataDepositBase = MetadataDepositBase;
	type MetadataDepositPerByte = MetadataDepositPerByte;
	type ApprovalDeposit = ApprovalDeposit;
	type StringLimit = StringLimit;
	type Freezer = ();
	type Extra = ();
	type CallbackHandle = ();
	type WeightInfo = ();
	type RemoveItemsLimit = RemoveItemsLimit;
	#[cfg(feature = "runtime-benchmarks")]
	type BenchmarkHelper = ();
}

parameter_types! {
	pub const ExraTreasury: PalletId = PalletId(*b"exra/trs");
	pub const ExraVault: PalletId = PalletId(*b"exra/vlt");
	pub const ExraUsdtAssetId: AssetId = USDT_ASSET_ID;
	pub ExraGenesisHash: H256 = H256([0xEEu8; 32]);
}

/// Adapter between `pallet_exra::UsdtHandler` and `pallet_assets`.
///
/// - `vault_balance` → assets balance of the vault sub-account for USDT.
/// - `pay_from_vault` → assets transfer vault → user (keep-alive not required,
///   vault is a pure sub-account with no existential consumer bond).
/// - `deposit_to_vault` → assets transfer external → vault.
pub struct UsdtBridge;

impl UsdtBridge {
	fn vault_account() -> AccountId {
		<pallet_exra::Pallet<Runtime>>::vault_account()
	}
}

impl pallet_exra::UsdtHandler<AccountId, Balance> for UsdtBridge {
	fn vault_balance() -> Balance {
		<Assets as FungiblesInspect<AccountId>>::balance(USDT_ASSET_ID, &Self::vault_account())
	}
	fn pay_from_vault(to: &AccountId, amount: Balance) -> DispatchResult {
		<Assets as FungiblesMutate<AccountId>>::transfer(
			USDT_ASSET_ID,
			&Self::vault_account(),
			to,
			amount,
			frame_support::traits::tokens::Preservation::Expendable,
		)
		.map(|_| ())
	}
	fn deposit_to_vault(from: &AccountId, amount: Balance) -> DispatchResult {
		<Assets as FungiblesMutate<AccountId>>::transfer(
			USDT_ASSET_ID,
			from,
			&Self::vault_account(),
			amount,
			frame_support::traits::tokens::Preservation::Expendable,
		)
		.map(|_| ())
	}
}

impl pallet_exra::Config for Runtime {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type RuntimeHoldReason = RuntimeHoldReason;
	type Usdt = UsdtBridge;
	type PalletIdTreasury = ExraTreasury;
	type PalletIdVault = ExraVault;
	type UsdtAssetId = ExraUsdtAssetId;
	type ChainGenesisHash = ExraGenesisHash;
}
