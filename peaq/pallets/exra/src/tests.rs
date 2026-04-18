//! Attack-vector coverage tests for pallet-exra v2.4.1.
//!
//! Each test targets a specific audit finding from the v2.4 stress-test
//! report. Names map 1:1 to the test matrix in
//! `docs/PALLET_EXRA_V2_4_1_SPEC.md` §8.

use crate as pallet_exra;
use crate::mock::*;
use crate::pallet::{
	CirculatingSupply, CurrentEpoch, IsFinalized, NodeStats, PendingClaims, Price, StakeLedger,
	Tier as TierStorage, UsedBatchIds,
};
use crate::pallet::{
	DAILY_BLOCKS, EPOCH_SIZE_PLANCKS, HARD_CAP_PLANCKS, PEAK_STAKE_PLANCKS,
	PRICE_STALENESS_BLOCKS,
};
use crate::types::{domain, Claim, NodeStat, TierType};
use codec::Encode;
use frame_support::{assert_noop, assert_ok};
use sp_core::{sr25519, Pair, H256};
use sp_io::hashing::blake2_256;
use sp_runtime::Permill;

// ---------- Helpers ----------

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

fn user(n: u8) -> sr25519::Public {
	sr25519::Public::from_raw([n; 32])
}

fn commit_mint(batch_id: H256, claims: &[Claim<sr25519::Public, u128>]) -> [u8; 32] {
	let claims_root = blake2_256(&claims.encode());
	blake2_256(&(domain::BATCH_MINT, MockGenesis::get(), batch_id, claims_root).encode())
}

fn commit_price(nonce: u64, price: u128) -> [u8; 32] {
	blake2_256(&(domain::SUBMIT_PRICE, MockGenesis::get(), nonce, price).encode())
}

fn sign_all(keys: &[sr25519::Pair; 3], msg: &[u8; 32]) -> Vec<(u8, [u8; 64])> {
	keys.iter()
		.enumerate()
		.map(|(i, k)| (i as u8, k.sign(msg).0))
		.collect()
}

fn sign_n(keys: &[sr25519::Pair; 3], msg: &[u8; 32], n: usize) -> Vec<(u8, [u8; 64])> {
	keys.iter()
		.take(n)
		.enumerate()
		.map(|(i, k)| (i as u8, k.sign(msg).0))
		.collect()
}

/// Bootstrap oracle set, a minimal price, pre-fund treasury to HARD_CAP, finalize.
fn boot_finalized(keys: &[sr25519::Pair; 3]) {
	assert_ok!(Exra::set_oracles(RuntimeOrigin::root(), oracle_pubs(keys)));
	// Seed price first (submit_price requires IsFinalized=false is NOT required,
	// but oracle set must already be set — which it is).
	let nonce = 1u64;
	let price = 1_000_000_000u128; // 1 USDT per EXRA
	let msg = commit_price(nonce, price);
	let sigs = sign_all(keys, &msg);
	assert_ok!(Exra::submit_price(
		RuntimeOrigin::signed(user(99)),
		nonce,
		price,
		sigs
	));
	// Pre-fund treasury to HARD_CAP.
	let treasury = Exra::treasury_account();
	assert_ok!(<Balances as frame_support::traits::fungible::Mutate<_>>::mint_into(
		&treasury,
		HARD_CAP_PLANCKS,
	));
	assert_ok!(Exra::finalize_protocol(RuntimeOrigin::root()));
	assert!(IsFinalized::<Test>::get());
}

// ---------- Tests ----------

#[test]
fn test_01_batch_mint_before_finalize_rejected() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		assert_ok!(Exra::set_oracles(RuntimeOrigin::root(), oracle_pubs(&keys)));
		// Inject stats so we wouldn't fail on NoStatsForAccount first.
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });
		let claims = vec![Claim { account: user(10), net: 1 }];
		let batch_id = H256([11u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		let sigs = sign_all(&keys, &msg);

		assert_noop!(
			Exra::batch_mint(RuntimeOrigin::signed(user(99)), batch_id, claims, sigs),
			pallet_exra::Error::<Test>::NotFinalized
		);
	});
}

#[test]
fn test_02_replay_same_batch_id_rejected() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });
		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 },
			TierType::Anon,
			Permill::one(),
		)
		.unwrap();
		let claims = vec![Claim { account: user(10), net }];
		let batch_id = H256([11u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		let sigs = sign_all(&keys, &msg);

		assert_ok!(Exra::batch_mint(
			RuntimeOrigin::signed(user(99)),
			batch_id,
			claims.clone(),
			sigs.clone()
		));
		// Re-insert stats to try again (they were consumed). Same batch_id → rejected.
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });
		assert_noop!(
			Exra::batch_mint(RuntimeOrigin::signed(user(99)), batch_id, claims, sigs),
			pallet_exra::Error::<Test>::ReplayedBatchId
		);
	});
}

#[test]
fn test_03_duplicate_oracle_sig_rejected() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });
		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 },
			TierType::Anon,
			Permill::one(),
		)
		.unwrap();
		let claims = vec![Claim { account: user(10), net }];
		let batch_id = H256([22u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		// Same oracle signs twice + one legit.
		let sig0 = keys[0].sign(&msg).0;
		let sigs = vec![(0u8, sig0), (0u8, sig0), (1u8, keys[1].sign(&msg).0)];

		assert_noop!(
			Exra::batch_mint(RuntimeOrigin::signed(user(99)), batch_id, claims, sigs),
			pallet_exra::Error::<Test>::DuplicateOracleSignature
		);
	});
}

#[test]
fn test_04_payload_swap_with_valid_sigs_rejected() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });

		let legit_claims = vec![Claim { account: user(10), net: 1_000 }];
		let batch_id = H256([33u8; 32]);
		let msg = commit_mint(batch_id, &legit_claims);
		let sigs = sign_all(&keys, &msg);

		// Attacker swaps payload: sends to user(99) with huge net, keeps same sigs.
		let evil_claims = vec![Claim {
			account: user(99),
			net: 1_000_000_000_000u128,
		}];
		assert_noop!(
			Exra::batch_mint(
				RuntimeOrigin::signed(user(99)),
				batch_id,
				evil_claims,
				sigs
			),
			pallet_exra::Error::<Test>::InvalidSignature
		);
	});
}

#[test]
fn test_05_threshold_math() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		// N=3, threshold=ceil(6/3)=2. 1 sig must fail, 2 must pass.
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });
		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 },
			TierType::Anon,
			Permill::one(),
		)
		.unwrap();
		let claims = vec![Claim { account: user(10), net }];
		let batch_id = H256([44u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		let one_sig = sign_n(&keys, &msg, 1);
		assert_noop!(
			Exra::batch_mint(
				RuntimeOrigin::signed(user(99)),
				batch_id,
				claims.clone(),
				one_sig
			),
			pallet_exra::Error::<Test>::InsufficientOracleConsensus
		);
		let two_sigs = sign_n(&keys, &msg, 2);
		assert_ok!(Exra::batch_mint(
			RuntimeOrigin::signed(user(99)),
			batch_id,
			claims,
			two_sigs
		));
	});
}

#[test]
fn test_06_hard_cap_breach_reverts_whole_batch() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		// Place CirculatingSupply just 1 below cap.
		CirculatingSupply::<Test>::put(HARD_CAP_PLANCKS - 1);
		// Stat that produces net > 1 guaranteed: 100 heartbeats + 1 GB = sizable.
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 });
		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 100, gb_verified: 1, gs: 500 },
			TierType::Anon,
			Permill::one(),
		)
		.unwrap();
		assert!(net > 1);
		let claims = vec![Claim { account: user(10), net }];
		let batch_id = H256([55u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		let sigs = sign_all(&keys, &msg);
		assert_noop!(
			Exra::batch_mint(RuntimeOrigin::signed(user(99)), batch_id, claims, sigs),
			pallet_exra::Error::<Test>::HardCapExceeded
		);
		// Nothing applied: NodeStats still present, CirculatingSupply unchanged.
		assert_eq!(CirculatingSupply::<Test>::get(), HARD_CAP_PLANCKS - 1);
		assert!(!UsedBatchIds::<Test>::contains_key(batch_id));
	});
}

#[test]
fn test_07_reward_overflow_saturates_not_panics() {
	new_test_ext().execute_with(|| {
		// u32::MAX heartbeats + u32::MAX GB + gs=1000 + epoch=1.0 + Peak.
		// Should not panic; should produce large-but-bounded Balance.
		let stat = NodeStat {
			heartbeats: u32::MAX,
			gb_verified: u32::MAX,
			gs: 1000,
		};
		let r = Exra::compute_reward(&stat, TierType::Peak, Permill::one()).unwrap();
		assert!(r > 0);
	});
}

#[test]
fn test_08_anon_tier_applies_0_75_no_double_tax() {
	new_test_ext().execute_with(|| {
		let stat = NodeStat { heartbeats: 200, gb_verified: 10, gs: 500 };
		let peak = Exra::compute_reward(&stat, TierType::Peak, Permill::one()).unwrap();
		let anon = Exra::compute_reward(&stat, TierType::Anon, Permill::one()).unwrap();
		// Anon must equal exactly 75% of Peak (± integer-division rounding of at most 1 planck).
		let expected = peak * 75 / 100;
		let diff = if anon > expected { anon - expected } else { expected - anon };
		assert!(diff <= 1, "anon={} expected={} diff={}", anon, expected, diff);
	});
}

#[test]
fn test_09_epoch_halving_at_100m_net() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		// Force epoch near threshold directly.
		CurrentEpoch::<Test>::mutate(|e| {
			e.minted_net = EPOCH_SIZE_PLANCKS - 1;
		});
		// Use a Peak user so timelock = 0.
		TierStorage::<Test>::insert(user(10), TierType::Peak);
		NodeStats::<Test>::insert(user(10), NodeStat { heartbeats: 1, gb_verified: 1, gs: 500 });
		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 1, gb_verified: 1, gs: 500 },
			TierType::Peak,
			Permill::one(),
		)
		.unwrap();
		assert!(net > 0);
		let claims = vec![Claim { account: user(10), net }];
		let batch_id = H256([66u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		let sigs = sign_all(&keys, &msg);
		assert_ok!(Exra::batch_mint(
			RuntimeOrigin::signed(user(99)),
			batch_id,
			claims,
			sigs
		));
		let e = CurrentEpoch::<Test>::get();
		assert_eq!(e.current, 1);
		assert_eq!(e.mult, Permill::from_parts(500_000));
	});
}

#[test]
fn test_10_stake_is_held_not_burned_and_unstakes_intact() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		let who = user(20);
		// Fund user with exactly PEAK_STAKE + ED.
		assert_ok!(<Balances as frame_support::traits::fungible::Mutate<_>>::mint_into(
			&who,
			PEAK_STAKE_PLANCKS + 10,
		));
		let before_total_issuance = Balances::total_issuance();

		assert_ok!(Exra::stake_for_peak(RuntimeOrigin::signed(who)));
		assert_eq!(StakeLedger::<Test>::get(&who), PEAK_STAKE_PLANCKS);
		assert_eq!(TierStorage::<Test>::get(&who), TierType::Peak);
		// Total issuance must be unchanged — hold is not burn.
		assert_eq!(Balances::total_issuance(), before_total_issuance);

		// To unstake: GS must be ≥ MIN_UNSTAKE_GS.
		NodeStats::<Test>::insert(&who, NodeStat { heartbeats: 0, gb_verified: 0, gs: 500 });
		assert_ok!(Exra::request_unstake(RuntimeOrigin::signed(who)));
		System::set_block_number(
			System::block_number() + (DAILY_BLOCKS * 7 + 1) as u64,
		);
		assert_ok!(Exra::finalize_unstake(RuntimeOrigin::signed(who)));
		assert_eq!(StakeLedger::<Test>::get(&who), 0);
		assert_eq!(TierStorage::<Test>::get(&who), TierType::Anon);
		assert_eq!(Balances::total_issuance(), before_total_issuance);
	});
}

#[test]
fn test_11_slash_burns_5pct_of_held_not_free() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		let who = user(30);
		assert_ok!(<Balances as frame_support::traits::fungible::Mutate<_>>::mint_into(
			&who,
			PEAK_STAKE_PLANCKS + 1_000_000,
		));
		assert_ok!(Exra::stake_for_peak(RuntimeOrigin::signed(who)));
		let issuance_before = Balances::total_issuance();
		let free_before = Balances::free_balance(&who);

		let nonce_id = H256([77u8; 32]);
		let reason = H256([88u8; 32]);
		let msg = blake2_256(
			&(domain::SLASH_NODE, MockGenesis::get(), nonce_id, who, reason).encode(),
		);
		let sigs = sign_all(&keys, &msg);
		assert_ok!(Exra::slash_node(
			RuntimeOrigin::signed(user(99)),
			nonce_id,
			who,
			reason,
			sigs
		));

		let expected_burn = PEAK_STAKE_PLANCKS * 500 / 10_000; // 5%
		assert_eq!(StakeLedger::<Test>::get(&who), PEAK_STAKE_PLANCKS - expected_burn);
		assert_eq!(Balances::total_issuance(), issuance_before - expected_burn);
		// Free balance untouched — slash comes from held, not free.
		assert_eq!(Balances::free_balance(&who), free_before);
	});
}

#[test]
fn test_12_exchange_fails_when_price_stale() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		let who = user(40);
		assert_ok!(<Balances as frame_support::traits::fungible::Mutate<_>>::mint_into(
			&who,
			1_000_000_000_000u128,
		));
		// Advance past staleness.
		let start = Price::<Test>::get().submitted_at;
		System::set_block_number(start + (PRICE_STALENESS_BLOCKS + 1) as u64);
		VAULT_USDT.with(|v| *v.borrow_mut() = u128::MAX / 2);
		assert_noop!(
			Exra::exchange_exra_to_usdt(RuntimeOrigin::signed(who), 1_000_000_000u128),
			pallet_exra::Error::<Test>::PriceFeedStale
		);
	});
}

#[test]
fn test_13_submit_price_rejects_large_deviation_without_unanimity() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys); // boots with price=1e9
		// Try to set price to 2e9 (+100%) with only 2 of 3 sigs.
		let nonce = 2u64;
		let price = 2_000_000_000u128;
		let msg = commit_price(nonce, price);
		let two = sign_n(&keys, &msg, 2);
		assert_noop!(
			Exra::submit_price(RuntimeOrigin::signed(user(99)), nonce, price, two),
			pallet_exra::Error::<Test>::PriceDeviationTooHigh
		);
		// With all 3 (unanimous), allowed.
		let all = sign_all(&keys, &msg);
		assert_ok!(Exra::submit_price(
			RuntimeOrigin::signed(user(99)),
			nonce,
			price,
			all
		));
		assert_eq!(Price::<Test>::get().usdt_per_exra_e9, price);
	});
}

#[test]
fn test_14_finalize_requires_prefund_oracles_price() {
	new_test_ext().execute_with(|| {
		// No oracles yet — must fail.
		assert_noop!(
			Exra::finalize_protocol(RuntimeOrigin::root()),
			pallet_exra::Error::<Test>::InvalidOracleSetSize
		);
		let keys = oracle_keys();
		assert_ok!(Exra::set_oracles(RuntimeOrigin::root(), oracle_pubs(&keys)));
		// Price not set — must fail.
		assert_noop!(
			Exra::finalize_protocol(RuntimeOrigin::root()),
			pallet_exra::Error::<Test>::PriceNotInitialized
		);
		let msg = commit_price(1, 1_000_000_000);
		assert_ok!(Exra::submit_price(
			RuntimeOrigin::signed(user(99)),
			1,
			1_000_000_000,
			sign_all(&keys, &msg)
		));
		// Treasury underfunded — must fail.
		assert_noop!(
			Exra::finalize_protocol(RuntimeOrigin::root()),
			pallet_exra::Error::<Test>::TreasuryUnderfunded
		);
	});
}

#[test]
fn test_15_admin_setters_blocked_post_finalize() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		// set_oracles must now fail — even from root.
		assert_noop!(
			Exra::set_oracles(RuntimeOrigin::root(), oracle_pubs(&keys)),
			pallet_exra::Error::<Test>::AlreadyFinalized
		);
		// finalize_protocol must fail — already finalized.
		assert_noop!(
			Exra::finalize_protocol(RuntimeOrigin::root()),
			pallet_exra::Error::<Test>::AlreadyFinalized
		);
	});
}

#[test]
fn test_16_claim_timelock_enforced_for_anon() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		let who = user(50);
		// Anon by default.
		NodeStats::<Test>::insert(&who, NodeStat { heartbeats: 10, gb_verified: 1, gs: 500 });
		let net = Exra::compute_reward(
			&NodeStat { heartbeats: 10, gb_verified: 1, gs: 500 },
			TierType::Anon,
			Permill::one(),
		)
		.unwrap();
		let claims = vec![Claim { account: who, net }];
		let batch_id = H256([99u8; 32]);
		let msg = commit_mint(batch_id, &claims);
		assert_ok!(Exra::batch_mint(
			RuntimeOrigin::signed(user(99)),
			batch_id,
			claims,
			sign_all(&keys, &msg)
		));
		// Too early.
		assert_noop!(
			Exra::claim(RuntimeOrigin::signed(who), batch_id),
			pallet_exra::Error::<Test>::TimelockActive
		);
		// After 24h.
		System::set_block_number(System::block_number() + DAILY_BLOCKS as u64 + 1);
		assert_ok!(Exra::claim(RuntimeOrigin::signed(who), batch_id));
		assert!(PendingClaims::<Test>::get(&who, batch_id).is_none());
	});
}

#[test]
fn test_17_price_nonce_replay_rejected() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		assert_ok!(Exra::set_oracles(RuntimeOrigin::root(), oracle_pubs(&keys)));
		let msg = commit_price(1, 1_000_000_000);
		assert_ok!(Exra::submit_price(
			RuntimeOrigin::signed(user(99)),
			1,
			1_000_000_000,
			sign_all(&keys, &msg)
		));
		// Same nonce reused — rejected.
		assert_noop!(
			Exra::submit_price(
				RuntimeOrigin::signed(user(99)),
				1,
				1_000_000_000,
				sign_all(&keys, &msg)
			),
			pallet_exra::Error::<Test>::ReplayedPriceNonce
		);
	});
}

#[test]
fn test_18_claim_mismatch_rejected() {
	new_test_ext().execute_with(|| {
		let keys = oracle_keys();
		boot_finalized(&keys);
		let who = user(60);
		NodeStats::<Test>::insert(&who, NodeStat { heartbeats: 10, gb_verified: 1, gs: 500 });
		// Oracles propose an inflated net — pallet recomputes and rejects.
		let claims = vec![Claim { account: who, net: u128::MAX / 2 }];
		let batch_id = H256([0xAAu8; 32]);
		let msg = commit_mint(batch_id, &claims);
		assert_noop!(
			Exra::batch_mint(
				RuntimeOrigin::signed(user(99)),
				batch_id,
				claims,
				sign_all(&keys, &msg)
			),
			pallet_exra::Error::<Test>::ClaimMismatch
		);
	});
}
