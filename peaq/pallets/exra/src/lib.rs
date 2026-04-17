#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

#[frame_support::pallet]
pub mod pallet {
	use frame_support::pallet_prelude::*;
	use frame_system::pallet_prelude::*;
	use sp_std::vec::Vec;
	use frame_support::traits::Currency;
	use codec::Encode;

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
		type Currency: frame_support::traits::Currency<Self::AccountId>;
		
		#[pallet::constant]
		type MaxOracles: Get<u32>;
		
		#[pallet::constant]
		type MaxMintsPerBatch: Get<u32>;
	}

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::storage]
	pub(super) type TrustedOracles<T: Config> = StorageValue<_, BoundedVec<T::AccountId, T::MaxOracles>, ValueQuery>;

	#[pallet::storage]
	pub(super) type TotalExraSupply<T: Config> = StorageValue<_, u128, ValueQuery>;

	#[pallet::storage]
	pub(super) type ReputationScores<T: Config> = StorageMap<_, Blake2_128Concat, T::AccountId, u32, ValueQuery>;

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		OracleAdded { oracle: T::AccountId },
		MintExecuted { batch_id: Vec<u8>, total_amount: u128 },
		StakeLocked { account: T::AccountId, amount: u128 },
		ReputationUpdated { account: T::AccountId, new_score: u32 },
	}

	#[pallet::error]
	pub enum Error<T> {
		NotAnOracle,
		InsufficientConsensus,
		SupplyCapExceeded,
		InvalidSignature,
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		#[pallet::call_index(0)]
		#[pallet::weight(T::DbWeight::get().writes(1))]
		pub fn add_oracle(origin: OriginFor<T>, oracle: T::AccountId) -> DispatchResult {
			ensure_root(origin)?;
			TrustedOracles::<T>::try_append(oracle.clone())
				.map_err(|_| Error::<T>::SupplyCapExceeded)?;
			Self::deposit_event(Event::OracleAdded { oracle });
			Ok(())
		}

		#[pallet::call_index(1)]
		#[pallet::weight(T::DbWeight::get().reads_writes(1, payouts.len() as u64))]
		pub fn batch_mint(
			origin: OriginFor<T>,
			batch_id: Vec<u8>,
			payouts: Vec<(T::AccountId, u128)>,
			signatures: Vec<(T::AccountId, [u8; 64])>,
		) -> DispatchResult {
			let sender = ensure_signed(origin)?;
			let oracles = TrustedOracles::<T>::get();
			ensure!(oracles.contains(&sender), Error::<T>::NotAnOracle);
			
			let threshold = (oracles.len() * 2 / 3).max(1);
			let mut valid_sigs = 0;
			
			for (oracle_id, sig_bytes) in signatures {
				if oracles.contains(&oracle_id) {
					let sig = sp_core::sr25519::Signature::from_raw(sig_bytes);
					let pubkey = sp_core::sr25519::Public::from_raw(oracle_id.encode().try_into().unwrap_or([0u8; 32]));
					
					if sp_io::crypto::sr25519_verify(&sig, &batch_id, &pubkey) {
						valid_sigs += 1;
					}
				}
			}
			
			ensure!(valid_sigs >= threshold, Error::<T>::InsufficientConsensus);
			
			let mut total_minted: u128 = 0;
			for (account, amount) in payouts {
				let amount_balance = amount.try_into().map_err(|_| Error::<T>::InvalidSignature)?;
				let _ = T::Currency::deposit_creating(&account, amount_balance);
				total_minted += amount;
			}
			
			TotalExraSupply::<T>::mutate(|s| *s += total_minted);
			Self::deposit_event(Event::MintExecuted { batch_id, total_amount: total_minted });
			Ok(())
		}

		#[pallet::call_index(2)]
		#[pallet::weight(T::DbWeight::get().writes(1))]
		pub fn stake_for_peak(origin: OriginFor<T>) -> DispatchResult {
			let sender = ensure_signed(origin)?;
			let amount: u128 = 100_000_000_000; // 100 EXRA
			let amount_balance = amount.try_into().map_err(|_| Error::<T>::InvalidSignature)?;
			
			let imbalance = T::Currency::withdraw(
				&sender,
				amount_balance,
				frame_support::traits::WithdrawReasons::RESERVE,
				frame_support::traits::ExistenceRequirement::KeepAlive,
			)?;
			
			// Explicitly handle the imbalance to ensure total supply consistency
			drop(imbalance);
			
			Self::deposit_event(Event::StakeLocked { account: sender, amount });
			Ok(())
		}

		#[pallet::call_index(3)]
		#[pallet::weight(T::DbWeight::get().reads_writes(1, updates.len() as u64))]
		pub fn update_reputations(
			origin: OriginFor<T>,
			update_id: Vec<u8>,
			updates: Vec<(T::AccountId, u32)>,
			signatures: Vec<(T::AccountId, [u8; 64])>,
		) -> DispatchResult {
			let sender = ensure_signed(origin)?;
			let oracles = TrustedOracles::<T>::get();
			ensure!(oracles.contains(&sender), Error::<T>::NotAnOracle);

			let threshold = (oracles.len() * 2 / 3).max(1);
			let mut valid_sigs = 0;
			
			for (oracle_id, sig_bytes) in signatures {
				if oracles.contains(&oracle_id) {
					let sig = sp_core::sr25519::Signature::from_raw(sig_bytes);
					let pubkey = sp_core::sr25519::Public::from_raw(oracle_id.encode().try_into().unwrap_or([0u8; 32]));
					
					if sp_io::crypto::sr25519_verify(&sig, &update_id, &pubkey) {
						valid_sigs += 1;
					}
				}
			}
			
			ensure!(valid_sigs >= threshold, Error::<T>::InsufficientConsensus);

			for (account, score) in updates {
				ReputationScores::<T>::insert(&account, score);
				Self::deposit_event(Event::ReputationUpdated { account, new_score: score });
			}
			
			Ok(())
		}
	}
}
