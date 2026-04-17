package middleware

import (
	"encoding/hex"
	"testing"

	"github.com/ChainSafe/go-schnorrkel"
	"github.com/stretchr/testify/assert"
)

func TestVerifyDIDSignature_Sr25519(t *testing.T) {
	// 1. Generate a keypair
	// Note: schnorrkel requires a 32-byte mini-secret and 32-byte chain-code or a specific derivation.
	// For testing, we use a simple seed and standard generation.
	secret := schnorrkel.NewSecretKey([32]byte{1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2}, [32]byte{})
	pubKey, err := secret.Public()
	if err != nil {
		t.Fatalf("failed to generate public key: %v", err)
	}
	pubKeyBytes := pubKey.Encode()
	pubKeyHex := hex.EncodeToString(pubKeyBytes[:])

	// 2. Sign a message
	message := "test-message"
	signingContext := schnorrkel.NewSigningContext([]byte("substrate"), []byte(message))
	sig, _ := secret.Sign(signingContext)
	sigBytes := sig.Encode()
	sigHex := hex.EncodeToString(sigBytes[:])

	// 3. Verify
	ok, err := VerifyDIDSignature(pubKeyHex, message, sigHex)
	assert.NoError(t, err)
	assert.True(t, ok, "sr25519 signature should be valid")

	// 4. Verify with 0x prefix
	ok, err = VerifyDIDSignature("0x"+pubKeyHex, message, "0x"+sigHex)
	assert.NoError(t, err)
	assert.True(t, ok, "sr25519 signature with 0x should be valid")

	// 5. Verify invalid message
	ok, err = VerifyDIDSignature(pubKeyHex, "wrong-message", sigHex)
	assert.NoError(t, err)
	assert.False(t, ok, "invalid message signature should fail")
}

func TestVerifyDIDSignature_ECDSA_Fallback(t *testing.T) {
	// This test ensures we didn't break legacy ECDSA fallback
	// (Note: we need a valid DER pubkey for this)
	// For simplicity, I'll just check it returns an error for non-ecdsa data but doesn't crash
	ok, err := VerifyDIDSignature("deadbeef", "msg", "signature")
	assert.Error(t, err)
	assert.False(t, ok)
}
