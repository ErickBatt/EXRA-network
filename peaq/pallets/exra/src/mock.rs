//! Mock runtime for pallet-exra tests.

use crate::{self as pallet_exra, pallet::UsdtHandler};
use frame_support::{
	parameter_types,
	traits::{ConstU128, ConstU16, ConstU32, ConstU64},
	PalletId,
};
use sp_core::H256;
use sp_runtime::{
	traits::{BlakeTwo256, IdentityLookup},
	BuildStorage, DispatchResult,
};

type Block = frame_system::mocking::MockBlock<Test>;
pub type AccountId = sp_core::sr25519::Public;
pub type Balance = u128;

frame_support::construct_runtime!(
	pub enum Test {
		System: frame_system,
		Balances: pallet_balances,
		Exra: pallet_exra,
	}
);

impl frame_system::Config for Test {
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

impl pallet_balances::Config for Test {
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
	pub const ExraTreasury: PalletId = PalletId(*b"exra/trs");
	pub const ExraVault: PalletId = PalletId(*b"exra/vlt");
	pub const MockGenesis: H256 = H256([7u8; 32]);
	pub const MockUsdtAssetId: u32 = 31337;
}

pub struct MockUsdt;

std::thread_local! {
	pub static VAULT_USDT: std::cell::RefCell<Balance> = std::cell::RefCell::new(0);
	pub static USDT_PAID: std::cell::RefCell<Vec<(AccountId, Balance)>> =
		std::cell::RefCell::new(Vec::new());
}

impl UsdtHandler<AccountId, Balance> for MockUsdt {
	fn vault_balance() -> Balance {
		VAULT_USDT.with(|v| *v.borrow())
	}
	fn pay_from_vault(to: &AccountId, amount: Balance) -> DispatchResult {
		VAULT_USDT.with(|v| {
			let mut b = v.borrow_mut();
			*b = b
				.checked_sub(amount)
				.ok_or(sp_runtime::DispatchError::Other("vault under"))?;
			Ok::<(), sp_runtime::DispatchError>(())
		})?;
		USDT_PAID.with(|p| p.borrow_mut().push((*to, amount)));
		Ok(())
	}
	fn deposit_to_vault(_from: &AccountId, amount: Balance) -> DispatchResult {
		VAULT_USDT.with(|v| *v.borrow_mut() += amount);
		Ok(())
	}
}

impl pallet_exra::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type RuntimeHoldReason = RuntimeHoldReason;
	type Usdt = MockUsdt;
	type PalletIdTreasury = ExraTreasury;
	type PalletIdVault = ExraVault;
	type UsdtAssetId = MockUsdtAssetId;
	type ChainGenesisHash = MockGenesis;
}

pub fn new_test_ext() -> sp_io::TestExternalities {
	let t = frame_system::GenesisConfig::<Test>::default()
		.build_storage()
		.unwrap();
	let mut ext: sp_io::TestExternalities = t.into();
	ext.execute_with(|| System::set_block_number(1));
	ext
}
