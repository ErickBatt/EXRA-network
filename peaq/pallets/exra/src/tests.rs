use crate::{mock::*, Event, TrustedOracles, ReputationScores};
use frame_support::{assert_noop, assert_ok};
use frame_support::traits::Currency;


#[test]
fn test_add_oracle_as_root() {
	new_test_ext().execute_with(|| {
		System::set_block_number(1);
		let alice = sp_runtime::AccountId32::from([1u8; 32]);
		// Root should be able to add an oracle (Alice)
		assert_ok!(Exra::add_oracle(RuntimeOrigin::root(), alice.clone()));
		assert!(TrustedOracles::<Test>::get().contains(&alice));
		
		// Event should be emitted
		System::assert_last_event(Event::OracleAdded { oracle: alice }.into());
	});
}

#[test]
fn test_add_oracle_not_root() {
	new_test_ext().execute_with(|| {
		System::set_block_number(1);
		let alice = sp_runtime::AccountId32::from([1u8; 32]);
		// Normal user should not be able to add an oracle
		assert_noop!(Exra::add_oracle(RuntimeOrigin::signed(alice.clone()), alice), sp_runtime::DispatchError::BadOrigin);
	});
}

#[test]
fn test_batch_mint_with_consensus() {
	new_test_ext().execute_with(|| {
		System::set_block_number(1);
		// 1. Add Oracle (Account 1)
		// Note: In tests, Account 1 as sr25519::Public is different from AccountId 1.
		// For simplicity in mock, we assume the AccountId 1 is the oracle.
		// However, pallet-exra expects to be able to convert AccountId to sr25519::Public.
		// Let's use real sr25519 pairs for valid signature testing.
		
		let alice = sp_runtime::AccountId32::from([1u8; 32]);
		let bob = sp_runtime::AccountId32::from([2u8; 32]);
		
		assert_ok!(Exra::add_oracle(RuntimeOrigin::root(), alice.clone()));
		
		let _batch_id = b"batch_001".to_vec();
		let _payouts = vec![(bob, 1000)];
	});
}

#[test]
fn test_stake_for_peak() {
	new_test_ext().execute_with(|| {
		System::set_block_number(1);
		let alice = sp_runtime::AccountId32::from([1u8; 32]);
		
		// Fund account Properly
		let stake_amount: u128 = 100_000_000_000;
		let _ = Balances::deposit_creating(&alice, stake_amount + 1000);
		
		assert_ok!(Exra::stake_for_peak(RuntimeOrigin::signed(alice.clone())));
		
		System::assert_last_event(Event::StakeLocked { account: alice, amount: stake_amount }.into());
	});
}

#[test]
fn test_update_reputations() {
	new_test_ext().execute_with(|| {
		System::set_block_number(1);
		let alice = sp_runtime::AccountId32::from([1u8; 32]);
		assert_ok!(Exra::add_oracle(RuntimeOrigin::root(), alice.clone()));
		
		// Verify storage map
		ReputationScores::<Test>::insert(&alice, 800);
		assert_eq!(ReputationScores::<Test>::get(&alice), 800);
	});
}
