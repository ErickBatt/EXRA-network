package peaq

import (
	"fmt"
	"os"
	"sync"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

// BlockchainClient defines the interface for interacting with peaq L1
type BlockchainClient interface {
	SendBatchMint(batchID []byte, rewards []RewardEntry, sigs []OracleSignature) (string, error)
	SendReputationUpdates(updateID []byte, updates []ReputationUpdate, sigs []OracleSignature) (string, error)
}

// PeaqClient manages connection to peaq L1
type PeaqClient struct {
	api      *gsrpc.SubstrateAPI
	metadata *types.Metadata
	keyring  signature.KeyringPair
	
	// Nonce management
	mu    sync.Mutex
	nonce uint32
}

// RewardEntry matches Vec<(T::AccountId, u128)> in Rust
type RewardEntry struct {
	Account types.AccountID
	Amount  types.U128
}

// OracleSignature matches (T::AccountId, [u8; 64]) in Rust
type OracleSignature struct {
	Account   types.AccountID
	Signature [64]byte
}

// ReputationUpdate matches (T::AccountId, u32) in Rust
type ReputationUpdate struct {
	Account types.AccountID
	Score   types.U32
}

// InitPeaqClient initializes the blockchain client
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

	keyring, err := signature.KeyringPairFromSecret(seed, 42) // 42 is standard Substrate prefix
	if err != nil {
		return nil, err
	}

	// Fetch initial nonce
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

// SendBatchMint triggers the on-chain reward distribution
func (c *PeaqClient) SendBatchMint(batchID []byte, rewards []RewardEntry, sigs []OracleSignature) (string, error) {
	call, err := types.NewCall(c.metadata, "PalletExra.batch_mint", 
		types.NewBytes(batchID),
		rewards,
		sigs,
	)
	if err != nil {
		return "", err
	}

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

	err = ext.Sign(c.keyring, options)
	if err != nil {
		return "", err
	}

	hash, err := c.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return "", err
	}

	c.nonce++ // Increment nonce for next call
	return hash.Hex(), nil
}

// SendReputationUpdates triggers the on-chain reputation scoring update
func (c *PeaqClient) SendReputationUpdates(updateID []byte, updates []ReputationUpdate, sigs []OracleSignature) (string, error) {
	call, err := types.NewCall(c.metadata, "PalletExra.update_reputations",
		types.NewBytes(updateID),
		updates,
		sigs,
	)
	if err != nil {
		return "", err
	}

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

	err = ext.Sign(c.keyring, options)
	if err != nil {
		return "", err
	}

	hash, err := c.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return "", err
	}

	c.nonce++
	return hash.Hex(), nil
}
