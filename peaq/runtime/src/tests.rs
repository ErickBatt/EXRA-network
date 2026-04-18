//! End-to-end integration tests for pallet-exra ↔ pallet-assets.
//!
//! These tests exercise the real `pallet-assets` implementation of USDT
//! via `UsdtBridge`, not the thread-local stub used inside the pallet's
//! own mock. They prove:
//! - `fund_vault` moves real asset balance into the vault sub-account.
//! - `exchange_exra_to_usdt` burns native EXRA and transfers USDT from
//!   the vault via `fungibles::Mutate`.
//! - Vault solvency errors surface when USDT is insufficient.

use crate::{
	AccountId, Assets, Balance, Balances, Exra, Runtime, RuntimeOrigin, System, UsdtBridge,
	USDT_ASSET_ID,
};
use codec::Encode;
use frame_support::{
	assert_noop, assert_ok,
	traits::fungible::Mutate as FungibleMutate,
	traits::fungibles::{Create, Inspect as FungiblesInspect, Mutate as FungiblesMutate},
};
use pallet_exra::pallet::{HARD_CAP_PLANCKS, PEAK_STAKE_PLANCKS};
use sp_runtime::Permill;
use pallet_exra::types::{domain, Claim, NodeStat, TierType};
use sp_core::{sr25519, Pair, H256};
use sp_io::hashing::blake2_256;
use sp_runtime::BuildStorage;

fn new_ext() -> sp_io::TestExternalities {
	let t = frame_system::GenesisConfig::<Runtime>::default()
		.build_storage()
		.unwrap();
	let mut ext: sp_io::TestExternalities = t.into();
	ext.execute_with(|| System::set_block_number(1));
	ext
}

fn oracle_keys() -> [sr25519::Pair; 3] {
	[
		sr25519::Pair::from_seed(&[1u8; 32]),
		sr25519::Pair::from_seed(&[2u8; 32]),
		sr25519::Pair::from_seed(&[3u8; 32]),
	]
}

fn oracle_pubs(keys: &[sr25519::Pair; 3]) -> Vec<sr25519::Public> {
	keys.iter().map(|k| k.public()).collect()
}

fn user(n: u8) -> AccountId {
	sr25519::Public::from_raw([n; 32])
}

fn commit_price(nonce: u64, price: u128) -> [u8; 32] {
	let g = <Runtime as pallet_exra::Config>::ChainGenesisHash::get();
	blake2_256(&(domain::SUBMIT_PRICE, g, nonce, price).encode())
}

fn commit_mint(batch_id: H256, claims: &[Claim<AccountId, Balance>]) -> [u8; 32] {
	let g = <Runtime as pallet_exra::Config>::ChainGenesisHash::get();
	let claims_root = blake2_256(&claims.encode());
	blake2_256(&(domain::BATCH_MINT, g, batch_id, claims_root).encode())
}

fn sign_all(keys: &[sr25519::Pair; 3], msg: &[u8; 32]) -> Vec<(u8, [u8; 64])> {
	keys.iter()
		.enumerate()
		.map(|(i, k)| (i as u8, k.sign(msg).0))
		.collect()
}

/// Bootstrap: create USDT asset, set oracles, submit a price, mint 1B treasury,
/// finalize protocol. Mirrors the pallet's own boot_finalized helper but uses
/// real pallet-assets for USDT.
fn boot(keys: &[sr25519::Pair; 3]) {
	let admin = user(250);
	// Give the asset admin some native balance for the asset-creation deposit.
	assert_ok!(<Balances as FungibleMutate<_>>::mint_into(&admin, 1_000_000));
	// Create USDT asset (id 31337) with `admin` as owner/issuer.
	assert_ok!(<Assets as Create<AccountId>>::create(
		USDT_ASSET_ID,
		admin.clone(),
		true,
		1,
	));

	// Oracle set.
	assert_ok!(Exra::set_oracles(RuntimeOrigin::root(), oracle_pubs(keys)));

	// Seed price: 1 USDT per EXRA, using 10^9 scale.
	let nonce = 1u64;
	let price = 1_000_000_000u128;
	let msg = commit_price(nonce, price);
	let sigs = sign_all(keys, &msg);
	assert_ok!(Exra::submit_price(
		RuntimeOrigin::signed(user(99)),
		nonce,
		price,
		sigs
	));

	// Pre-fund treasury to HARD_CAP (distribution model, never mint after finalize).
	let treasury = Exra::treasury_account();
	assert_ok!(<Balances as FungibleMutate<_>>::mint_into(
		&treasury,
		HARD_CAP_PLANCKS,
	));

	assert_ok!(Exra::finalize_protocol(RuntimeOrigin::root()));
}

/// Issue USDT to an external B2B account and fund the vault via root.
fn fund_vault_with(b2b: &AccountId, amount: Balance) {
	// Mint the asset to the B2B provider.
	assert_ok!(<Assets as FungiblesMutate<_>>::mint_into(
		USDT_ASSET_ID,
		b2b,
		amount,
	));
	assert_ok!(Exra::fund_vault(
		RuntimeOrigin::root(),
		b2b.clone(),
		amount,
	));
}

#[test]
fn integration_fund_vault_moves_real_asset_balance() {
	new_ext().execute_with(|| {
		let keys = oracle_keys();
		boot(&keys);

		let b2b = user(200);
		assert_ok!(<Assets as FungiblesMutate<_>>::mint_into(
			USDT_ASSET_ID,
			&b2b,
			10_000_000_000u128,
		));

		let vault = UsdtBridge::vault_account();
		let vault_before = <Assets as FungiblesInspect<_>>::balance(USDT_ASSET_ID, &vault);
		let b2b_before = <Assets as FungiblesInspect<_>>::balance(USDT_ASSET_ID, &b2b);

		assert_ok!(Exra::fund_vault(
			RuntimeOrigin::root(),
			b2b.clone(),
			5_000_000_000u128,
		));

		let vault_after = <Assets as FungiblesInspect<_>>::balance(USDT_ASSET_ID, &vault);
		let b2b_after = <Assets as FungiblesInspect<_>>::balance(USDT_ASSET_ID, &b2b);

		assert_eq!(vault_after - vault_before, 5_000_000_000u128);
		assert_eq!(b2b_before - b2b_after, 5_000_000_000u128);
	});
}

#[test]
fn integration_fund_vault_non_root_rejected() {
	new_ext().execute_with(|| {
		let keys = oracle_keys();
		boot(&keys);

		let b2b = user(200);
		assert_ok!(<Assets as FungiblesMutate<_>>::mint_into(
			USDT_ASSET_ID,
			&b2b,
			1_000_000u128,
		));
		assert_noop!(
			Exra::fund_vault(
				RuntimeOrigin::signed(user(99)),
				b2b.clone(),
				1_000u128,
			),
			sp_runtime::DispatchError::BadOrigin
		);
	});
}

#[test]
fn integration_exchange_exra_to_usdt_full_flow() {
	new_ext().execute_with(|| {
		let keys = oracle_keys();
		boot(&keys);

		// Fund vault with 1_000 USDT (1e12 plancks @ 1e9 decimals).
		let b2b = user(200);
		fund_vault_with(&b2b, 1_000_000_000_000u128);

		// Distribute some EXRA to a user via batch_mint to have something to burn.
		let alice = user(10);
		pallet_exra::pallet::NodeStats::<Runtime>::insert(
			&alice,
			NodeStat { heartbeats: 1000, gb_verified: 10, gs: 500 },
		);
		pallet_exra::pallet::Tier::<Runtime>::insert(&alice, TierType::Anon);

		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 1000, gb_verified: 10, gs: 500 },
			TierType::Anon,
			Permill::one(),
		)
		.unwrap();
		assert!(net > 0);

		let claims = vec![Claim { account: alice.clone(), net }];
		let batch_id = H256([42u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		let sigs = sign_all(&keys, &msg);
		assert_ok!(Exra::batch_mint(
			RuntimeOrigin::signed(user(99)),
			batch_id,
			claims,
			sigs
		));

		// Advance past timelock (Anon = DAILY_BLOCKS = 7200) and claim.
		System::set_block_number(System::block_number() + 10_000);
		assert_ok!(Exra::claim(RuntimeOrigin::signed(alice.clone()), batch_id));

		// Price went stale during the block advance — resubmit with a fresh nonce.
		let nonce2 = 2u64;
		let price2 = 1_000_000_000u128;
		let msg2 = commit_price(nonce2, price2);
		let sigs2 = sign_all(&keys, &msg2);
		assert_ok!(Exra::submit_price(
			RuntimeOrigin::signed(user(99)),
			nonce2,
			price2,
			sigs2,
		));

		// Exchange a portion of the claimed EXRA to USDT.
		let to_exchange: Balance = 1_000_000_000; // 1 EXRA
		let alice_exra_before = Balances::free_balance(&alice);
		let alice_usdt_before =
			<Assets as FungiblesInspect<_>>::balance(USDT_ASSET_ID, &alice);
		let vault_usdt_before = <Assets as FungiblesInspect<_>>::balance(
			USDT_ASSET_ID,
			&UsdtBridge::vault_account(),
		);

		assert_ok!(Exra::exchange_exra_to_usdt(
			RuntimeOrigin::signed(alice.clone()),
			to_exchange,
		));

		let alice_exra_after = Balances::free_balance(&alice);
		let alice_usdt_after =
			<Assets as FungiblesInspect<_>>::balance(USDT_ASSET_ID, &alice);
		let vault_usdt_after = <Assets as FungiblesInspect<_>>::balance(
			USDT_ASSET_ID,
			&UsdtBridge::vault_account(),
		);

		// EXRA was burned from alice.
		assert_eq!(alice_exra_before - alice_exra_after, to_exchange);
		// Alice received USDT; vault lost the same amount.
		let usdt_gained = alice_usdt_after - alice_usdt_before;
		let vault_delta = vault_usdt_before - vault_usdt_after;
		assert!(usdt_gained > 0);
		assert_eq!(usdt_gained, vault_delta);
	});
}

#[test]
fn integration_exchange_fails_when_vault_empty() {
	new_ext().execute_with(|| {
		let keys = oracle_keys();
		boot(&keys);

		// Give alice some EXRA directly for simplicity.
		let alice = user(10);
		assert_ok!(<Balances as FungibleMutate<_>>::mint_into(
			&alice,
			10 * PEAK_STAKE_PLANCKS,
		));

		// Do NOT fund vault. Exchange must fail.
		assert!(Exra::exchange_exra_to_usdt(
			RuntimeOrigin::signed(alice.clone()),
			1_000_000_000u128,
		)
		.is_err());
	});
}
