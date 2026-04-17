package middleware

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/ChainSafe/go-schnorrkel"
)

// VerifyDIDSignature verifies a signature against a message and a public key.
// Supports both sr25519 (64-byte) and ECDSA (compact format).
func VerifyDIDSignature(pubKeyHex, message, signatureHex string) (bool, error) {
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil {
		return false, errors.New("invalid signature hex")
	}

	pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(pubKeyHex, "0x"))
	if err != nil {
		return false, errors.New("invalid public key hex")
	}

	// 1. Try sr25519 (Schnorrkel) - signatures are 64 bytes
	if len(sigBytes) == 64 && len(pubKeyBytes) == 32 {
		var signature [64]byte
		copy(signature[:], sigBytes)
		
		var pubKeyRaw [32]byte
		copy(pubKeyRaw[:], pubKeyBytes)

		publicKey, err := schnorrkel.NewPublicKey(pubKeyRaw)
		if err != nil {
			return false, err
		}
		signingContext := schnorrkel.NewSigningContext([]byte("substrate"), []byte(message))
		
		sig := &schnorrkel.Signature{}
		if err := sig.Decode(signature); err != nil {
			return false, err
		}

		res, err := publicKey.Verify(sig, signingContext)
		return res, err
	}

	// 2. Fallback to ECDSA
	genericPubKey, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return false, errors.New("failed to parse public key: " + err.Error())
	}

	pubKey, ok := genericPubKey.(*ecdsa.PublicKey)
	if !ok {
		return false, errors.New("not an ECDSA public key")
	}

	if len(sigBytes) != 64 {
		return false, errors.New("invalid signature length")
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	hash := sha256.Sum256([]byte(message))
	return ecdsa.Verify(pubKey, hash[:], r, s), nil
}
