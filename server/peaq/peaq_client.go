package peaq

import (
	"fmt"
	"os"
	"sync"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

// BlockchainClient defines the interface for interacting with peaq L1 (pallet-exra v2.4.1).
type BlockchainClient interface {
	// SendBatchMint submits oracle-signed reward claims via pallet_exra::batch_mint.
	// batchID is a 32-byte H256 commitment hash.
	// sigs must use oracle set index (IndexedSignature.Index), NOT AccountID.
	SendBatchMint(batchID [32]byte, claims []ClaimEntry, sigs []IndexedSignature) (string, error)

	// SendUpdateStats submits oracle-signed node stat updates via pallet_exra::update_stats.
	SendUpdateStats(batchID [32]byte, entries []StatEntry, sigs []IndexedSignature) (string, error)

	// GetOracleSet returns on-chain oracle public keys in storage order.
	// Required to map oracle DIDs to IndexedSignature.Index before on-chain submission.
	GetOracleSet() ([][32]byte, error)
}

// PeaqClient manages the connection to peaq L1.
type PeaqClient struct {
	api      *gsrpc.SubstrateAPI
	metadata *types.Metadata
	keyring  signature.KeyringPair
	mu       sync.Mutex
	nonce    uint32
}

// ClaimEntry matches types::Claim<AccountId, Balance> in pallet-exra v2.4.1.
// batch_mint takes Vec<Claim>; Net is the post-tier reward in plancks (9 decimals).
type ClaimEntry struct {
	Account types.AccountID
	Net     types.U128
}

// IndexedSignature matches (u8, [u8; 64]) in pallet-exra v2.4.1.
// Index is the oracle's position in on-chain OracleSet, NOT the AccountID.
// Sending AccountID here would cause a SCALE-decode panic on-chain.
type IndexedSignature struct {
	Index     uint8
	Signature [64]byte
}

// StatEntry matches (T::AccountId, NodeStat) for pallet_exra::update_stats (v2.4.1).
// NodeStat = { heartbeats: u32, gb_verified: u32, gs: u16 }.
type StatEntry struct {
	Account    types.AccountID
	Heartbeats uint32
	GbVerified uint32
	Gs         uint16
}

// OracleSignature is used off-chain (DB storage and peer p2p transport).
// Must be converted to IndexedSignature via GetOracleSet() before on-chain submission.
type OracleSignature struct {
	Account   types.AccountID
	Signature [64]byte
}

// RewardEntry is retained for the DB aggregation layer (backward compat).
type RewardEntry struct {
	Account types.AccountID
	Amount  types.U128
}

// ReputationUpdate is retained for backward compat; use StatEntry for on-chain calls.
type ReputationUpdate struct {
	Account types.AccountID
	Score   types.U32
}

// InitPeaqClient initializes the blockchain client from environment variables.
func InitPeaqClient() (*PeaqClient, error) {
	rpcURL := os.Getenv("PEAQ_RPC")
	if rpcURL == "" {
		return nil, fmt.Errorf("PEAQ_RPC environment variable not set")
	}
	seed := os.Getenv("PEAQ_ORACLE_SEED")
	if seed == "" {
		return nil, fmt.Errorf("PEAQ_ORACLE_SEED environment variable not set")
	}

	api, err := gsrpc.NewSubstrateAPI(rpcURL)
	if err != nil {
		return nil, err
	}
	metadata, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}
	keyring, err := signature.KeyringPairFromSecret(seed, 42)
	if err != nil {
		return nil, err
	}

	sKey, err := types.CreateStorageKey(metadata, "System", "Account", keyring.PublicKey)
	if err != nil {
		return nil, err
	}
	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(sKey, &accountInfo)
	if err != nil || !ok {
		return nil, fmt.Errorf("failed to fetch initial nonce: %v", err)
	}

	return &PeaqClient{
		api:      api,
		metadata: metadata,
		keyring:  keyring,
		nonce:    uint32(accountInfo.Nonce),
	}, nil
}

// GetOracleSet fetches BoundedVec<sr25519::Public, 7> from on-chain OracleSet storage.
// Returns keys in storage order; position i == IndexedSignature.Index for oracle i.
func (c *PeaqClient) GetOracleSet() ([][32]byte, error) {
	sKey, err := types.CreateStorageKey(c.metadata, "PalletExra", "OracleSet")
	if err != nil {
		return nil, fmt.Errorf("OracleSet storage key: %w", err)
	}
	var raw []byte
	ok, err := c.api.RPC.State.GetStorageLatest(sKey, &raw)
	if err != nil {
		return nil, fmt.Errorf("fetch OracleSet: %w", err)
	}
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("OracleSet not found or empty")
	}
	// BoundedVec<[u8;32]> SCALE-encodes as compact-length prefix + contiguous 32-byte keys.
	count, prefixLen := decodeCompactUint(raw)
	data := raw[prefixLen:]
	if uint64(len(data)) < count*32 {
		return nil, fmt.Errorf("OracleSet SCALE decode: expected %d×32 bytes, got %d", count, len(data))
	}
	keys := make([][32]byte, count)
	for i := uint64(0); i < count; i++ {
		copy(keys[i][:], data[i*32:(i+1)*32])
	}
	return keys, nil
}

// decodeCompactUint decodes a SCALE compact-encoded uint from the front of b.
// Returns (value, bytesConsumed).
func decodeCompactUint(b []byte) (uint64, int) {
	if len(b) == 0 {
		return 0, 0
	}
	mode := b[0] & 0x3
	switch mode {
	case 0:
		return uint64(b[0] >> 2), 1
	case 1:
		return uint64(b[0]>>2) | uint64(b[1])<<6, 2
	case 2:
		v := uint64(b[0]>>2) | uint64(b[1])<<6 | uint64(b[2])<<14 | uint64(b[3])<<22
		return v, 4
	default:
		byteCount := int(b[0] >> 2)
		v := uint64(0)
		for i := 0; i < byteCount && i < 8; i++ {
			v |= uint64(b[1+i]) << (8 * i)
		}
		return v, 1 + byteCount
	}
}

// submitExtrinsic signs and submits a call, manages nonce under mutex.
func (c *PeaqClient) submitExtrinsic(call types.Call) (string, error) {
	ext := types.NewExtrinsic(call)
	genesisHash, err := c.api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return "", err
	}
	rv, err := c.api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	options := types.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(c.nonce)),
		TransactionVersion: types.U32(rv.TransactionVersion),
		SpecVersion:        types.U32(rv.SpecVersion),
		Tip:                types.NewUCompactFromUInt(0),
	}
	if err = ext.Sign(c.keyring, options); err != nil {
		return "", err
	}
	hash, err := c.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return "", err
	}
	c.nonce++
	return hash.Hex(), nil
}

// SendBatchMint triggers pallet_exra::batch_mint (v2.4.1).
func (c *PeaqClient) SendBatchMint(batchID [32]byte, claims []ClaimEntry, sigs []IndexedSignature) (string, error) {
	call, err := types.NewCall(c.metadata, "PalletExra.batch_mint",
		types.H256(batchID),
		claims,
		sigs,
	)
	if err != nil {
		return "", err
	}
	return c.submitExtrinsic(call)
}

// SendUpdateStats triggers pallet_exra::update_stats (v2.4.1).
func (c *PeaqClient) SendUpdateStats(batchID [32]byte, entries []StatEntry, sigs []IndexedSignature) (string, error) {
	call, err := types.NewCall(c.metadata, "PalletExra.update_stats",
		types.H256(batchID),
		entries,
		sigs,
	)
	if err != nil {
		return "", err
	}
	return c.submitExtrinsic(call)
}
