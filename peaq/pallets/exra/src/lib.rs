//! # pallet-exra v2.4.1
//!
//! Production-immutable EXRA token distribution pallet for peaq.
//! Spec: `docs/PALLET_EXRA_V2_4_1_SPEC.md`.
//!
//! Security invariants (must hold always):
//! - `CirculatingSupply ≤ HARD_CAP`.
//! - `IsFinalized: false → true` is one-way.
//! - After finalize: no storage item gated by `!IsFinalized` can change.
//! - `UsedBatchIds` / `UsedPriceNonces` entries are never deleted.
//! - Oracle sigs bind full payload via domain-separated blake2_256 commitment.

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;
pub mod types;

#[cfg(test)]
mod mock;
#[cfg(test)]
mod tests;

#[frame_support::pallet]
pub mod pallet {
	use crate::types::{
		domain, BurnRecord, Claim, ClaimInfo, EpochInfo, NodeStat, PriceEntry, SlashRecord,
		TierType,
	};
	use codec::Encode;
	use frame_support::{
		pallet_prelude::*,
		sp_runtime::{
			traits::{AccountIdConversion, CheckedAdd, Saturating, Zero},
			ArithmeticError, Permill,
		},
		traits::{
			fungible::{Inspect, Mutate, MutateHold},
			tokens::{Precision, Preservation},
			BuildGenesisConfig,
		},
		PalletId,
	};
	use frame_system::pallet_prelude::*;
	use sp_core::{sr25519, H256, U256};
	use sp_io::{crypto::sr25519_verify, hashing::blake2_256};
	use sp_std::{collections::btree_set::BTreeSet, vec::Vec};

	pub type BalanceOf<T> =
		<<T as Config>::Currency as Inspect<<T as frame_system::Config>::AccountId>>::Balance;

	/// Oracle set capacity. Threshold is ceil(2N/3); N must be in [3, 7].
	pub const ORACLE_SET_MAX: u32 = 7;

	/// Hard cap on circulating supply. Denominated in plancks (9 decimals).
	/// 1_000_000_000 EXRA * 10^9 = 10^18.
	pub const HARD_CAP_PLANCKS: u128 = 1_000_000_000u128 * 1_000_000_000u128;

	/// Halving trigger: net-minted per epoch.
	pub const EPOCH_SIZE_PLANCKS: u128 = 100_000_000u128 * 1_000_000_000u128;

	/// Residual epoch-mult floor (0.001%): emission never fully stops.
	pub const EPOCH_MULT_FLOOR: Permill = Permill::from_parts(10_000);

	/// Blocks per day at 12s target block time.
	pub const DAILY_BLOCKS: u32 = 7_200;

	/// Price feed staleness window (2 hours).
	pub const PRICE_STALENESS_BLOCKS: u32 = 600;

	/// Price deviation threshold beyond which unanimous oracle consent is required.
	/// 20% = 2000 bps.
	pub const PRICE_DEVIATION_BPS: u32 = 2_000;

	/// Vault spread on EXRA→USDT exchange (2%).
	pub const VAULT_SPREAD_BPS: u32 = 200;

	/// PEAK stake amount: 100 EXRA in plancks.
	pub const PEAK_STAKE_PLANCKS: u128 = 100 * 1_000_000_000u128;

	/// Heartbeat reward constant (0.00005 EXRA per 5-min heartbeat).
	pub const HEARTBEAT_REWARD_PLANCKS: u128 = 50_000;

	/// Per-GB reward constant (0.30 EXRA per verified GB).
	pub const GB_RATE_PLANCKS: u128 = 300_000_000;

	/// Max entries per `batch_mint` / `update_stats` call.
	pub const MAX_ENTRIES_PER_CALL: u32 = 512;

	/// Min GS required to unstake (prevents abusers cashing out post-slash).
	pub const MIN_UNSTAKE_GS: u16 = 100;

	/// Unstake cooldown (7 days).
	pub const UNSTAKE_COOLDOWN_BLOCKS: u32 = DAILY_BLOCKS * 7;

	/// Min interval between `claim` calls per account (1 hour).
	pub const CLAIM_VELOCITY_BLOCKS: u32 = DAILY_BLOCKS / 24;

	/// Pending-claim TTL after which bounded cleanup removes it.
	pub const CLAIM_TTL_BLOCKS: u32 = DAILY_BLOCKS * 30;

	/// Slash percent on canary failure (5%).
	pub const SLASH_BPS: u32 = 500;

	/// Abstract handler for USDT leg of the exchange. The runtime implements
	/// this against `pallet_assets` (or any fungibles instance) so the pallet
	/// stays decoupled from the USDT asset choice.
	pub trait UsdtHandler<AccountId, Balance> {
		fn vault_balance() -> Balance;
		fn pay_from_vault(to: &AccountId, amount: Balance) -> DispatchResult;
		fn deposit_to_vault(from: &AccountId, amount: Balance) -> DispatchResult;
	}

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>>
			+ IsType<<Self as frame_system::Config>::RuntimeEvent>;

		/// Native EXRA currency. Must support holds so stake is real, not burnt.
		type Currency: Inspect<Self::AccountId>
			+ Mutate<Self::AccountId>
			+ MutateHold<Self::AccountId, Reason = Self::RuntimeHoldReason>;

		/// Runtime-level hold reason enum. Runtime wraps `HoldReason::PeakStake`.
		type RuntimeHoldReason: From<HoldReason>;

		/// USDT leg of `exchange_exra_to_usdt`.
		type Usdt: UsdtHandler<Self::AccountId, BalanceOf<Self>>;

		/// PalletId used to derive treasury and vault sub-accounts deterministically.
		#[pallet::constant]
		type PalletIdTreasury: Get<PalletId>;

		#[pallet::constant]
		type PalletIdVault: Get<PalletId>;

		/// Genesis hash of this chain. Mixed into the signature commitment to
		/// prevent replay across chains that share oracle keys.
		#[pallet::constant]
		type ChainGenesisHash: Get<H256>;
	}

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	/// Composite hold reason — construct_runtime! picks this up and generates
	/// the matching `RuntimeHoldReason::Exra(HoldReason)` variant.
	#[pallet::composite_enum]
	pub enum HoldReason {
		PeakStake,
	}

	impl<T: Config> Pallet<T> {
		/// Deterministic treasury sub-account. Holds the 1B EXRA pre-minted
		/// at genesis; `batch_mint` distributes from here — no mint path exists.
		pub fn treasury_account() -> T::AccountId {
			T::PalletIdTreasury::get().into_account_truncating()
		}

		/// Deterministic vault sub-account. Holds USDT inflows from B2B.
		pub fn vault_account() -> T::AccountId {
			T::PalletIdVault::get().into_account_truncating()
		}
	}

	// ---------- Storage ----------

	#[pallet::storage]
	pub type CirculatingSupply<T: Config> = StorageValue<_, BalanceOf<T>, ValueQuery>;

	#[pallet::storage]
	pub type OracleSet<T: Config> =
		StorageValue<_, BoundedVec<sr25519::Public, ConstU32<ORACLE_SET_MAX>>, ValueQuery>;

	#[pallet::storage]
	pub type NodeStats<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, NodeStat, ValueQuery>;

	#[pallet::storage]
	pub type Tier<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, TierType, ValueQuery>;

	#[pallet::storage]
	pub type CurrentEpoch<T: Config> =
		StorageValue<_, EpochInfo<BalanceOf<T>>, ValueQuery>;

	#[pallet::storage]
	pub type PendingClaims<T: Config> = StorageDoubleMap<
		_,
		Blake2_128Concat,
		T::AccountId,
		Blake2_128Concat,
		H256,
		ClaimInfo<BalanceOf<T>, BlockNumberFor<T>>,
		OptionQuery,
	>;

	#[pallet::storage]
	pub type UsedBatchIds<T: Config> =
		StorageMap<_, Blake2_128Concat, H256, (), OptionQuery>;

	#[pallet::storage]
	pub type UsedPriceNonces<T: Config> =
		StorageMap<_, Blake2_128Concat, u64, (), OptionQuery>;

	#[pallet::storage]
	pub type LastClaimBlock<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, BlockNumberFor<T>, ValueQuery>;

	#[pallet::storage]
	pub type IsFinalized<T: Config> = StorageValue<_, bool, ValueQuery>;

	#[pallet::storage]
	pub type Price<T: Config> =
		StorageValue<_, PriceEntry<BlockNumberFor<T>>, ValueQuery>;

	#[pallet::storage]
	pub type StakeLedger<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, BalanceOf<T>, ValueQuery>;

	#[pallet::storage]
	pub type PendingUnstake<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, BlockNumberFor<T>, OptionQuery>;

	#[pallet::storage]
	pub type BurnSeq<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, u64, ValueQuery>;

	#[pallet::storage]
	pub type BurnHistory<T: Config> = StorageDoubleMap<
		_,
		Blake2_128Concat,
		T::AccountId,
		Blake2_128Concat,
		u64,
		BurnRecord<BalanceOf<T>, BlockNumberFor<T>>,
		OptionQuery,
	>;

	#[pallet::storage]
	pub type SlashRecords<T: Config> = StorageMap<
		_,
		Blake2_128Concat,
		T::AccountId,
		SlashRecord<BalanceOf<T>, BlockNumberFor<T>>,
		OptionQuery,
	>;

	// ---------- Events ----------

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		OracleSetUpdated { oracle_count: u32, set_hash: H256 },
		ProtocolFinalized { oracle_set_hash: H256, treasury_balance: BalanceOf<T> },
		BatchMinted {
			batch_id: H256,
			payload_hash: H256,
			total_net: BalanceOf<T>,
			epoch: u32,
		},
		StatsUpdated { batch_id: H256, payload_hash: H256, count: u32 },
		ClaimSettled {
			account: T::AccountId,
			batch_id: H256,
			amount: BalanceOf<T>,
		},
		Exchanged {
			account: T::AccountId,
			exra_burned: BalanceOf<T>,
			usdt_paid: BalanceOf<T>,
			price: u128,
			burn_seq: u64,
		},
		PriceSubmitted { nonce: u64, price: u128, deviation_bps: u32 },
		Staked { account: T::AccountId, amount: BalanceOf<T> },
		UnstakeRequested { account: T::AccountId, unlock_at: BlockNumberFor<T> },
		Unstaked { account: T::AccountId, amount: BalanceOf<T> },
		Slashed {
			account: T::AccountId,
			amount_burned: BalanceOf<T>,
			reason_hash: H256,
		},
		TierChanged { account: T::AccountId, tier: TierType },
		EpochAdvanced { epoch: u32, new_mult: Permill },
	}

	// ---------- Errors ----------

	#[pallet::error]
	pub enum Error<T> {
		NotFinalized,
		AlreadyFinalized,
		InvalidOracleSetSize,
		InvalidOracleIndex,
		DuplicateOracleSignature,
		InsufficientOracleConsensus,
		InvalidSignature,
		ReplayedBatchId,
		ReplayedPriceNonce,
		HardCapExceeded,
		OverflowInRewardCalc,
		ClaimMismatch,
		TooManyEntries,
		NoStatsForAccount,
		TimelockActive,
		VelocityLimit,
		NoPendingClaim,
		PriceFeedStale,
		PriceDeviationTooHigh,
		VaultInsufficientUsdt,
		TreasuryUnderfunded,
		StakeAlreadyHeld,
		StakeNotHeld,
		UnstakeCooldownActive,
		UnstakePending,
		GsTooLowForUnstake,
		PriceNotInitialized,
	}

	// ---------- Dispatchables ----------

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		/// Root-only. Set the oracle pubkey set. Blocked after finalize.
		#[pallet::call_index(0)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().writes(1)))]
		pub fn set_oracles(
			origin: OriginFor<T>,
			oracles: Vec<sr25519::Public>,
		) -> DispatchResult {
			ensure_root(origin)?;
			ensure!(!IsFinalized::<T>::get(), Error::<T>::AlreadyFinalized);
			ensure!(
				oracles.len() >= 3 && oracles.len() <= ORACLE_SET_MAX as usize,
				Error::<T>::InvalidOracleSetSize
			);
			let bounded: BoundedVec<_, ConstU32<ORACLE_SET_MAX>> =
				BoundedVec::try_from(oracles).map_err(|_| Error::<T>::InvalidOracleSetSize)?;
			let set_hash = H256(blake2_256(&bounded.encode()));
			let count = bounded.len() as u32;
			OracleSet::<T>::put(bounded);
			Self::deposit_event(Event::OracleSetUpdated { oracle_count: count, set_hash });
			Ok(())
		}

		/// Root-only. One-way transition to immutable mode. Preconditions:
		/// oracle set ≥3, price feed initialized, treasury pre-funded to HARD_CAP.
		#[pallet::call_index(1)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().writes(1)))]
		pub fn finalize_protocol(origin: OriginFor<T>) -> DispatchResult {
			ensure_root(origin)?;
			ensure!(!IsFinalized::<T>::get(), Error::<T>::AlreadyFinalized);

			let oracles = OracleSet::<T>::get();
			ensure!(oracles.len() >= 3, Error::<T>::InvalidOracleSetSize);

			let price = Price::<T>::get();
			ensure!(
				!price.submitted_at.is_zero() && price.usdt_per_exra_e9 > 0,
				Error::<T>::PriceNotInitialized
			);

			let treasury = Self::treasury_account();
			let balance = T::Currency::balance(&treasury);
			let cap: BalanceOf<T> = HARD_CAP_PLANCKS
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			ensure!(balance >= cap, Error::<T>::TreasuryUnderfunded);

			IsFinalized::<T>::put(true);
			let set_hash = H256(blake2_256(&oracles.encode()));
			Self::deposit_event(Event::ProtocolFinalized {
				oracle_set_hash: set_hash,
				treasury_balance: balance,
			});
			Ok(())
		}

		/// Oracle multisig. Overwrites `NodeStats` for each account in `entries`.
		#[pallet::call_index(2)]
		#[pallet::weight(
			Weight::from_parts(10_000, 0).saturating_add(
				T::DbWeight::get().reads_writes(1, entries.len() as u64)
			)
		)]
		pub fn update_stats(
			origin: OriginFor<T>,
			batch_id: H256,
			entries: Vec<(T::AccountId, NodeStat)>,
			signatures: Vec<(u8, [u8; 64])>,
		) -> DispatchResult {
			let _ = ensure_signed(origin)?;
			ensure!(IsFinalized::<T>::get(), Error::<T>::NotFinalized);
			ensure!(entries.len() as u32 <= MAX_ENTRIES_PER_CALL, Error::<T>::TooManyEntries);
			ensure!(!UsedBatchIds::<T>::contains_key(batch_id), Error::<T>::ReplayedBatchId);

			let payload_hash = Self::commit_stats(batch_id, &entries);
			Self::verify_oracle_multisig(domain::UPDATE_STATS, &payload_hash, &signatures)?;

			UsedBatchIds::<T>::insert(batch_id, ());
			let count = entries.len() as u32;
			for (acc, stat) in entries {
				NodeStats::<T>::insert(&acc, stat);
			}
			Self::deposit_event(Event::StatsUpdated {
				batch_id,
				payload_hash: H256(payload_hash),
				count,
			});
			Ok(())
		}

		/// Oracle multisig. Computes reward from on-chain `NodeStats`, transfers
		/// from treasury to pallet-held pending pool (via PendingClaims), resets
		/// NodeStats to zero for each processed account.
		#[pallet::call_index(3)]
		#[pallet::weight(
			Weight::from_parts(10_000, 0).saturating_add(
				T::DbWeight::get().reads_writes(2, claims.len() as u64 * 2)
			)
		)]
		pub fn batch_mint(
			origin: OriginFor<T>,
			batch_id: H256,
			claims: Vec<Claim<T::AccountId, BalanceOf<T>>>,
			signatures: Vec<(u8, [u8; 64])>,
		) -> DispatchResult {
			let _ = ensure_signed(origin)?;
			ensure!(IsFinalized::<T>::get(), Error::<T>::NotFinalized);
			ensure!(claims.len() as u32 <= MAX_ENTRIES_PER_CALL, Error::<T>::TooManyEntries);
			ensure!(!UsedBatchIds::<T>::contains_key(batch_id), Error::<T>::ReplayedBatchId);

			let payload_hash = Self::commit_mint(batch_id, &claims);
			Self::verify_oracle_multisig(domain::BATCH_MINT, &payload_hash, &signatures)?;

			// Re-derive reward for each claim, verify equality with oracle-proposed net.
			let mut total_net: BalanceOf<T> = Zero::zero();
			let epoch = CurrentEpoch::<T>::get();
			let now = <frame_system::Pallet<T>>::block_number();

			// Collect computed (account, net, tier) for atomic apply phase.
			let mut computed: Vec<(T::AccountId, BalanceOf<T>, TierType)> =
				Vec::with_capacity(claims.len());
			for claim in &claims {
				let stat = NodeStats::<T>::get(&claim.account);
				ensure!(stat != NodeStat::default(), Error::<T>::NoStatsForAccount);
				let tier = Tier::<T>::get(&claim.account);
				let net = Self::compute_reward(&stat, tier, epoch.mult)?;
				ensure!(net == claim.net, Error::<T>::ClaimMismatch);
				total_net = total_net
					.checked_add(&net)
					.ok_or(ArithmeticError::Overflow)?;
				computed.push((claim.account.clone(), net, tier));
			}

			// Cap check on cumulative circulating post-batch.
			let cap: BalanceOf<T> = HARD_CAP_PLANCKS
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			let new_supply = CirculatingSupply::<T>::get()
				.checked_add(&total_net)
				.ok_or(ArithmeticError::Overflow)?;
			ensure!(new_supply <= cap, Error::<T>::HardCapExceeded);

			// Treasury must hold enough liquid EXRA to move.
			let treasury = Self::treasury_account();
			let t_bal = T::Currency::balance(&treasury);
			ensure!(t_bal >= total_net, Error::<T>::TreasuryUnderfunded);

			// Apply phase (after all checks — no partial state on failure above).
			UsedBatchIds::<T>::insert(batch_id, ());
			CirculatingSupply::<T>::put(new_supply);

			let mut new_epoch = epoch;
			new_epoch.minted_net = new_epoch.minted_net.saturating_add(total_net);
			let epoch_size: BalanceOf<T> = EPOCH_SIZE_PLANCKS
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			while new_epoch.minted_net >= epoch_size {
				new_epoch.minted_net = new_epoch.minted_net.saturating_sub(epoch_size);
				new_epoch.current = new_epoch.current.saturating_add(1);
				new_epoch.mult = Self::halve_with_floor(new_epoch.mult);
				Self::deposit_event(Event::EpochAdvanced {
					epoch: new_epoch.current,
					new_mult: new_epoch.mult,
				});
			}
			CurrentEpoch::<T>::put(new_epoch);

			for (acc, net, tier) in computed {
				// Move funds from treasury to a per-pallet "pending" holding via transfer
				// to the pallet account; pending is tracked in PendingClaims.
				T::Currency::transfer(
					&treasury,
					&Self::pending_account(),
					net,
					Preservation::Expendable,
				)?;
				let unlock = match tier {
					TierType::Peak => now,
					TierType::Anon => now.saturating_add(DAILY_BLOCKS.into()),
				};
				// If a claim for (acc, batch_id) already exists (impossible given
				// UsedBatchIds, but defensive), merge by summing.
				PendingClaims::<T>::mutate(&acc, batch_id, |slot| {
					let existing = slot.map(|c| c.net).unwrap_or_else(Zero::zero);
					*slot = Some(ClaimInfo {
						net: existing.saturating_add(net),
						unlock_block: unlock,
					});
				});
				// Consume stats — each NodeStat window is spent exactly once.
				NodeStats::<T>::remove(&acc);
			}

			Self::deposit_event(Event::BatchMinted {
				batch_id,
				payload_hash: H256(payload_hash),
				total_net,
				epoch: new_epoch.current,
			});
			Ok(())
		}

		/// Post-timelock withdrawal. Rate-limited per account.
		#[pallet::call_index(4)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(2, 2)))]
		pub fn claim(origin: OriginFor<T>, batch_id: H256) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let info =
				PendingClaims::<T>::get(&who, batch_id).ok_or(Error::<T>::NoPendingClaim)?;
			let now = <frame_system::Pallet<T>>::block_number();
			ensure!(now >= info.unlock_block, Error::<T>::TimelockActive);

			let last = LastClaimBlock::<T>::get(&who);
			let velocity: BlockNumberFor<T> = CLAIM_VELOCITY_BLOCKS.into();
			if !last.is_zero() {
				ensure!(now.saturating_sub(last) >= velocity, Error::<T>::VelocityLimit);
			}

			T::Currency::transfer(
				&Self::pending_account(),
				&who,
				info.net,
				Preservation::Expendable,
			)?;
			PendingClaims::<T>::remove(&who, batch_id);
			LastClaimBlock::<T>::insert(&who, now);
			Self::deposit_event(Event::ClaimSettled { account: who, batch_id, amount: info.net });
			Ok(())
		}

		/// Burns EXRA, pays out USDT from vault at current on-chain price minus spread.
		#[pallet::call_index(5)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(3, 3)))]
		pub fn exchange_exra_to_usdt(
			origin: OriginFor<T>,
			exra_amount: BalanceOf<T>,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			ensure!(!exra_amount.is_zero(), Error::<T>::ClaimMismatch);

			let price = Price::<T>::get();
			ensure!(
				!price.submitted_at.is_zero() && price.usdt_per_exra_e9 > 0,
				Error::<T>::PriceNotInitialized
			);
			let now = <frame_system::Pallet<T>>::block_number();
			let staleness: BlockNumberFor<T> = PRICE_STALENESS_BLOCKS.into();
			ensure!(
				now.saturating_sub(price.submitted_at) <= staleness,
				Error::<T>::PriceFeedStale
			);

			let exra_u128: u128 = exra_amount
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			let gross_u128 = U256::from(exra_u128)
				.checked_mul(U256::from(price.usdt_per_exra_e9))
				.ok_or(Error::<T>::OverflowInRewardCalc)?
				/ U256::from(1_000_000_000u128);
			let spread_u128 = gross_u128 * U256::from(VAULT_SPREAD_BPS) / U256::from(10_000u32);
			let payout_u128: u128 = (gross_u128 - spread_u128)
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			let payout: BalanceOf<T> = payout_u128
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;

			ensure!(
				T::Usdt::vault_balance() >= payout,
				Error::<T>::VaultInsufficientUsdt
			);

			// Real burn via fungible::Mutate.
			T::Currency::burn_from(
				&who,
				exra_amount,
				Precision::Exact,
				frame_support::traits::tokens::Fortitude::Polite,
			)?;
			CirculatingSupply::<T>::mutate(|s| *s = s.saturating_sub(exra_amount));

			T::Usdt::pay_from_vault(&who, payout)?;

			let seq = BurnSeq::<T>::mutate(&who, |s| {
				*s = s.saturating_add(1);
				*s
			});
			BurnHistory::<T>::insert(
				&who,
				seq,
				BurnRecord {
					exra_burned: exra_amount,
					usdt_paid: payout,
					price_at_burn: price.usdt_per_exra_e9,
					at_block: now,
				},
			);
			Self::deposit_event(Event::Exchanged {
				account: who,
				exra_burned: exra_amount,
				usdt_paid: payout,
				price: price.usdt_per_exra_e9,
				burn_seq: seq,
			});
			Ok(())
		}

		/// Oracle multisig. Sets on-chain price feed. Deviation >20% vs prior
		/// price requires unanimous oracle consent.
		#[pallet::call_index(6)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(2, 2)))]
		pub fn submit_price(
			origin: OriginFor<T>,
			nonce: u64,
			price: u128,
			signatures: Vec<(u8, [u8; 64])>,
		) -> DispatchResult {
			let _ = ensure_signed(origin)?;
			ensure!(price > 0, Error::<T>::PriceNotInitialized);
			ensure!(!UsedPriceNonces::<T>::contains_key(nonce), Error::<T>::ReplayedPriceNonce);

			let payload_hash = Self::commit_price(nonce, price);
			let verified = Self::verify_oracle_multisig_count(
				domain::SUBMIT_PRICE,
				&payload_hash,
				&signatures,
			)?;

			let prior = Price::<T>::get();
			let mut deviation_bps: u32 = 0;
			if prior.usdt_per_exra_e9 > 0 {
				let prev = prior.usdt_per_exra_e9 as i128;
				let curr = price as i128;
				let diff = (curr - prev).unsigned_abs();
				// bps = diff * 10_000 / prev
				let d_bps: u128 = diff
					.checked_mul(10_000)
					.and_then(|v| v.checked_div(prior.usdt_per_exra_e9))
					.unwrap_or(u128::MAX);
				deviation_bps = d_bps.min(u32::MAX as u128) as u32;

				if deviation_bps > PRICE_DEVIATION_BPS {
					let total_oracles = OracleSet::<T>::get().len() as u32;
					ensure!(verified >= total_oracles, Error::<T>::PriceDeviationTooHigh);
				}
			}

			UsedPriceNonces::<T>::insert(nonce, ());
			let now = <frame_system::Pallet<T>>::block_number();
			Price::<T>::put(PriceEntry {
				usdt_per_exra_e9: price,
				submitted_at: now,
			});
			Self::deposit_event(Event::PriceSubmitted { nonce, price, deviation_bps });
			Ok(())
		}

		/// Hold PEAK_STAKE and set tier = Peak.
		#[pallet::call_index(7)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(2, 3)))]
		pub fn stake_for_peak(origin: OriginFor<T>) -> DispatchResult {
			let who = ensure_signed(origin)?;
			ensure!(StakeLedger::<T>::get(&who).is_zero(), Error::<T>::StakeAlreadyHeld);
			let amount: BalanceOf<T> = PEAK_STAKE_PLANCKS
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			T::Currency::hold(&HoldReason::PeakStake.into(), &who, amount)?;
			StakeLedger::<T>::insert(&who, amount);
			Tier::<T>::insert(&who, TierType::Peak);
			Self::deposit_event(Event::Staked { account: who.clone(), amount });
			Self::deposit_event(Event::TierChanged { account: who, tier: TierType::Peak });
			Ok(())
		}

		/// Request unstake. Enforces minimum GS + queues release after cooldown.
		#[pallet::call_index(8)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(2, 1)))]
		pub fn request_unstake(origin: OriginFor<T>) -> DispatchResult {
			let who = ensure_signed(origin)?;
			ensure!(!StakeLedger::<T>::get(&who).is_zero(), Error::<T>::StakeNotHeld);
			ensure!(
				PendingUnstake::<T>::get(&who).is_none(),
				Error::<T>::UnstakePending
			);
			let gs = NodeStats::<T>::get(&who).gs;
			ensure!(gs >= MIN_UNSTAKE_GS, Error::<T>::GsTooLowForUnstake);
			let now = <frame_system::Pallet<T>>::block_number();
			let unlock = now.saturating_add(UNSTAKE_COOLDOWN_BLOCKS.into());
			PendingUnstake::<T>::insert(&who, unlock);
			Self::deposit_event(Event::UnstakeRequested { account: who, unlock_at: unlock });
			Ok(())
		}

		/// Finalize unstake after cooldown.
		#[pallet::call_index(9)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(3, 3)))]
		pub fn finalize_unstake(origin: OriginFor<T>) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let unlock = PendingUnstake::<T>::get(&who).ok_or(Error::<T>::StakeNotHeld)?;
			let now = <frame_system::Pallet<T>>::block_number();
			ensure!(now >= unlock, Error::<T>::UnstakeCooldownActive);
			let amount = StakeLedger::<T>::get(&who);
			ensure!(!amount.is_zero(), Error::<T>::StakeNotHeld);
			T::Currency::release(
				&HoldReason::PeakStake.into(),
				&who,
				amount,
				Precision::Exact,
			)?;
			StakeLedger::<T>::remove(&who);
			PendingUnstake::<T>::remove(&who);
			Tier::<T>::insert(&who, TierType::Anon);
			Self::deposit_event(Event::Unstaked { account: who.clone(), amount });
			Self::deposit_event(Event::TierChanged { account: who, tier: TierType::Anon });
			Ok(())
		}

		/// Oracle multisig. Slash SLASH_BPS of held stake and zero GS.
		#[pallet::call_index(10)]
		#[pallet::weight(Weight::from_parts(10_000, 0).saturating_add(T::DbWeight::get().reads_writes(2, 3)))]
		pub fn slash_node(
			origin: OriginFor<T>,
			nonce_id: H256,
			target: T::AccountId,
			reason_hash: H256,
			signatures: Vec<(u8, [u8; 64])>,
		) -> DispatchResult {
			let _ = ensure_signed(origin)?;
			ensure!(!UsedBatchIds::<T>::contains_key(nonce_id), Error::<T>::ReplayedBatchId);
			let payload_hash = Self::commit_slash(nonce_id, &target, reason_hash);
			Self::verify_oracle_multisig(domain::SLASH_NODE, &payload_hash, &signatures)?;

			UsedBatchIds::<T>::insert(nonce_id, ());

			let held = StakeLedger::<T>::get(&target);
			let slash_amount: BalanceOf<T> = if held.is_zero() {
				Zero::zero()
			} else {
				let h_u128: u128 = held.try_into().map_err(|_| Error::<T>::OverflowInRewardCalc)?;
				let s = h_u128.saturating_mul(SLASH_BPS as u128) / 10_000u128;
				s.try_into().map_err(|_| Error::<T>::OverflowInRewardCalc)?
			};
			if !slash_amount.is_zero() {
				T::Currency::burn_held(
					&HoldReason::PeakStake.into(),
					&target,
					slash_amount,
					Precision::BestEffort,
					frame_support::traits::tokens::Fortitude::Force,
				)?;
				StakeLedger::<T>::mutate(&target, |s| *s = s.saturating_sub(slash_amount));
			}

			NodeStats::<T>::mutate(&target, |st| st.gs = 0);

			let now = <frame_system::Pallet<T>>::block_number();
			SlashRecords::<T>::insert(
				&target,
				SlashRecord { amount_burned: slash_amount, reason_hash, at_block: now },
			);
			Self::deposit_event(Event::Slashed {
				account: target,
				amount_burned: slash_amount,
				reason_hash,
			});
			Ok(())
		}
	}

	// ---------- Internal helpers ----------

	impl<T: Config> Pallet<T> {
		fn pending_account() -> T::AccountId {
			// Reuse treasury-pallet-id with a different sub-seed so claims do
			// not mingle with undisbursed treasury balance.
			T::PalletIdTreasury::get().into_sub_account_truncating(b"pending")
		}

		fn halve_with_floor(m: Permill) -> Permill {
			let halved = Permill::from_parts(m.deconstruct() / 2);
			if halved < EPOCH_MULT_FLOOR {
				EPOCH_MULT_FLOOR
			} else {
				halved
			}
		}

		fn tier_mult(tier: TierType) -> Permill {
			match tier {
				TierType::Peak => Permill::one(),
				TierType::Anon => Permill::from_percent(75),
			}
		}

		/// Core Breathe reward calculation. Overflow-safe via U256. Returns
		/// saturated Balance (clamped at Balance::MAX, never panics).
		pub fn compute_reward(
			stat: &NodeStat,
			tier: TierType,
			epoch_mult: Permill,
		) -> Result<BalanceOf<T>, DispatchError> {
			let r_uptime = (stat.heartbeats as u128).saturating_mul(HEARTBEAT_REWARD_PLANCKS);
			let r_work = (stat.gb_verified as u128).saturating_mul(GB_RATE_PLANCKS);
			let base = r_uptime.saturating_add(r_work);

			// RS_mult = min(gs, 1000) * 2 / 1000, range [0, 2.0]
			let rs_num = (stat.gs.min(1000) as u128).saturating_mul(2);
			let rs_den: u128 = 1_000;
			let epoch_ppm = epoch_mult.deconstruct() as u128;
			let tier_ppm = Self::tier_mult(tier).deconstruct() as u128;

			let pre = U256::from(base)
				.saturating_mul(U256::from(rs_num))
				.saturating_mul(U256::from(epoch_ppm))
				.saturating_mul(U256::from(tier_ppm));
			let denom = U256::from(rs_den)
				.saturating_mul(U256::from(1_000_000u128))
				.saturating_mul(U256::from(1_000_000u128));
			let reward_u256 = pre / denom;

			// Saturating cast to BalanceOf<T>.
			let reward_u128: u128 = reward_u256.try_into().unwrap_or(u128::MAX);
			let reward: BalanceOf<T> = reward_u128
				.try_into()
				.map_err(|_| Error::<T>::OverflowInRewardCalc)?;
			Ok(reward)
		}

		// Commitment helpers — domain-separated hash over full payload.
		fn commit_mint(
			batch_id: H256,
			claims: &[Claim<T::AccountId, BalanceOf<T>>],
		) -> [u8; 32] {
			let claims_root = blake2_256(&claims.encode());
			let genesis = T::ChainGenesisHash::get();
			blake2_256(&(domain::BATCH_MINT, genesis, batch_id, claims_root).encode())
		}

		fn commit_stats(batch_id: H256, entries: &[(T::AccountId, NodeStat)]) -> [u8; 32] {
			let root = blake2_256(&entries.encode());
			let genesis = T::ChainGenesisHash::get();
			blake2_256(&(domain::UPDATE_STATS, genesis, batch_id, root).encode())
		}

		fn commit_price(nonce: u64, price: u128) -> [u8; 32] {
			let genesis = T::ChainGenesisHash::get();
			blake2_256(&(domain::SUBMIT_PRICE, genesis, nonce, price).encode())
		}

		fn commit_slash(nonce_id: H256, target: &T::AccountId, reason: H256) -> [u8; 32] {
			let genesis = T::ChainGenesisHash::get();
			blake2_256(&(domain::SLASH_NODE, genesis, nonce_id, target, reason).encode())
		}

		/// Verify a multisig meets threshold `ceil(2N/3)`. Dedupes by oracle index.
		fn verify_oracle_multisig(
			domain_tag: &[u8],
			message: &[u8; 32],
			signatures: &[(u8, [u8; 64])],
		) -> DispatchResult {
			let got = Self::verify_oracle_multisig_count(domain_tag, message, signatures)?;
			let n = OracleSet::<T>::get().len() as u32;
			let threshold = (n.saturating_mul(2).saturating_add(2)) / 3; // ceil(2n/3)
			ensure!(got >= threshold, Error::<T>::InsufficientOracleConsensus);
			Ok(())
		}

		/// Shared sig-verification that returns the count of distinct valid oracles.
		/// Used by `submit_price` to enforce unanimity on big deviations.
		fn verify_oracle_multisig_count(
			_domain_tag: &[u8],
			message: &[u8; 32],
			signatures: &[(u8, [u8; 64])],
		) -> Result<u32, DispatchError> {
			let oracles = OracleSet::<T>::get();
			ensure!(oracles.len() >= 3, Error::<T>::InvalidOracleSetSize);
			let mut seen: BTreeSet<u8> = BTreeSet::new();
			let mut valid: u32 = 0;
			for (idx, sig_bytes) in signatures {
				let i = *idx as usize;
				ensure!(i < oracles.len(), Error::<T>::InvalidOracleIndex);
				ensure!(seen.insert(*idx), Error::<T>::DuplicateOracleSignature);
				let pubkey = oracles[i];
				let sig = sr25519::Signature::from_raw(*sig_bytes);
				if sr25519_verify(&sig, message, &pubkey) {
					valid = valid.saturating_add(1);
				} else {
					return Err(Error::<T>::InvalidSignature.into());
				}
			}
			Ok(valid)
		}
	}

	// ---------- Genesis ----------

	#[pallet::genesis_config]
	pub struct GenesisConfig<T: Config> {
		/// Initial oracle pubkey set. Must be size in [3, ORACLE_SET_MAX].
		pub oracles: Vec<sr25519::Public>,
		/// Pre-funded treasury balance in plancks. Must equal `HARD_CAP_PLANCKS`
		/// prior to `finalize_protocol`.
		pub treasury_plancks: u128,
		#[serde(skip)]
		pub _phantom: sp_std::marker::PhantomData<T>,
	}

	impl<T: Config> Default for GenesisConfig<T> {
		fn default() -> Self {
			Self {
				oracles: Vec::new(),
				treasury_plancks: 0,
				_phantom: Default::default(),
			}
		}
	}

	#[pallet::genesis_build]
	impl<T: Config> BuildGenesisConfig for GenesisConfig<T> {
		fn build(&self) {
			if !self.oracles.is_empty() {
				let bounded: BoundedVec<_, ConstU32<ORACLE_SET_MAX>> =
					BoundedVec::try_from(self.oracles.clone())
						.expect("genesis oracle set size within bounds; qed");
				OracleSet::<T>::put(bounded);
			}
			if self.treasury_plancks > 0 {
				if let Ok(bal) = self.treasury_plancks.try_into() {
					let _ = T::Currency::mint_into(&Pallet::<T>::treasury_account(), bal);
				}
			}
		}
	}
}
