// Package gwclaims issues and verifies Gateway session tokens.
//
// Security posture (§1 D1 AUDIT_MARKETPLACE_v2.4.1.md):
//
//   - Primary algorithm: EdDSA (Ed25519). Control Plane holds a private key and
//     signs; Gateway holds only the public key and verifies. This breaks
//     symmetry of the prior HS256 scheme where a compromised Gateway could
//     forge tokens for Control Plane.
//   - No hardcoded fallback. If neither an Ed25519 keypair nor an explicit
//     GATEWAY_JWT_SECRET is configured, MustInitSigner/MustInitVerifier fail
//     fast at process start.
//   - HS256 is accepted as a transitional mode only when GATEWAY_JWT_SECRET is
//     explicitly set AND the Ed25519 key is unset. This is deliberate for
//     dev/CI; production deployments should use EdDSA.
//
// Env contract:
//
//	GATEWAY_JWT_ED25519_PRIV  base64 raw 32-byte Ed25519 seed (Control Plane)
//	GATEWAY_JWT_ED25519_PUB   base64 raw 32-byte Ed25519 public key (Gateway)
//	GATEWAY_JWT_SECRET        transitional HMAC secret (both sides)
package gwclaims

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	envEd25519Priv = "GATEWAY_JWT_ED25519_PRIV"
	envEd25519Pub  = "GATEWAY_JWT_ED25519_PUB"
	envHMACSecret  = "GATEWAY_JWT_SECRET"

	// Issuer and audience pin tokens to this service pair. Audit finding A4.
	Issuer   = "exra-control-plane"
	Audience = "exra-gateway"

	// DefaultTTL is the lifetime of a gateway session JWT. Kept short to limit
	// the blast radius if a token leaks (A4: the previous 5-minute TTL was too
	// long for a one-shot session JWT).
	DefaultTTL = 90 * time.Second
)

// Claims carries the session and role identifiers plus standard JWT claims.
type Claims struct {
	SessionID string `json:"sid"`
	BuyerID   string `json:"buyer_id,omitempty"`
	Role      string `json:"role"` // "node" or "buyer"
	jwt.RegisteredClaims
}

var (
	signOnce   sync.Once
	signErr    error
	signEd25519 ed25519.PrivateKey
	signHMAC    []byte

	verifyOnce    sync.Once
	verifyErr     error
	verifyEd25519 ed25519.PublicKey
	verifyHMAC    []byte
)

var ErrNotConfigured = errors.New("gwclaims: no signing/verifying key configured (set GATEWAY_JWT_ED25519_PRIV / GATEWAY_JWT_ED25519_PUB, or GATEWAY_JWT_SECRET for dev)")

func loadSigner() {
	privB64 := os.Getenv(envEd25519Priv)
	secret := os.Getenv(envHMACSecret)

	if privB64 != "" {
		seed, err := base64.StdEncoding.DecodeString(privB64)
		if err != nil {
			signErr = fmt.Errorf("gwclaims: decode %s: %w", envEd25519Priv, err)
			return
		}
		if len(seed) != ed25519.SeedSize {
			signErr = fmt.Errorf("gwclaims: %s must be %d-byte seed, got %d", envEd25519Priv, ed25519.SeedSize, len(seed))
			return
		}
		signEd25519 = ed25519.NewKeyFromSeed(seed)
		return
	}

	if secret != "" {
		signHMAC = []byte(secret)
		return
	}

	signErr = ErrNotConfigured
}

func loadVerifier() {
	pubB64 := os.Getenv(envEd25519Pub)
	secret := os.Getenv(envHMACSecret)

	if pubB64 != "" {
		pub, err := base64.StdEncoding.DecodeString(pubB64)
		if err != nil {
			verifyErr = fmt.Errorf("gwclaims: decode %s: %w", envEd25519Pub, err)
			return
		}
		if len(pub) != ed25519.PublicKeySize {
			verifyErr = fmt.Errorf("gwclaims: %s must be %d-byte key, got %d", envEd25519Pub, ed25519.PublicKeySize, len(pub))
			return
		}
		verifyEd25519 = ed25519.PublicKey(pub)
		return
	}

	if secret != "" {
		verifyHMAC = []byte(secret)
		return
	}

	verifyErr = ErrNotConfigured
}

// Sign issues a session JWT for sid/role. Lazy-loads the signing key on first
// call. Returns an error (never panics) if the key is not configured; callers
// at process start should instead invoke MustInitSigner to fail fast.
func Sign(sessionID, buyerID, role string, ttl time.Duration) (string, error) {
	signOnce.Do(loadSigner)
	if signErr != nil {
		return "", signErr
	}

	if ttl <= 0 {
		ttl = DefaultTTL
	}
	now := time.Now()
	claims := Claims{
		SessionID: sessionID,
		BuyerID:   buyerID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Audience:  []string{Audience},
			Subject:   sessionID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), // small skew tolerance
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	if signEd25519 != nil {
		tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		return tok.SignedString(signEd25519)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(signHMAC)
}

// Verify parses and validates a session JWT. Lazy-loads the verifying key.
func Verify(tokenString string) (*Claims, error) {
	verifyOnce.Do(loadVerifier)
	if verifyErr != nil {
		return nil, verifyErr
	}

	parserOpts := []jwt.ParserOption{
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
		jwt.WithLeeway(5 * time.Second),
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		switch token.Method.Alg() {
		case jwt.SigningMethodEdDSA.Alg():
			if verifyEd25519 == nil {
				return nil, errors.New("gwclaims: EdDSA token but no Ed25519 public key configured")
			}
			return verifyEd25519, nil
		case jwt.SigningMethodHS256.Alg():
			if len(verifyHMAC) == 0 {
				return nil, errors.New("gwclaims: HS256 token but no GATEWAY_JWT_SECRET configured")
			}
			return verifyHMAC, nil
		default:
			return nil, fmt.Errorf("gwclaims: unexpected signing method: %v", token.Method.Alg())
		}
	}, parserOpts...)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("gwclaims: invalid token")
	}
	if claims.SessionID == "" {
		return nil, errors.New("gwclaims: invalid claims: sid is empty")
	}
	if claims.Role != "node" && claims.Role != "buyer" {
		return nil, errors.New("gwclaims: invalid claims: unrecognized role")
	}
	return claims, nil
}

// MustInitSigner is called at Control Plane startup. Fails the process if no
// signing key is configured.
func MustInitSigner() {
	signOnce.Do(loadSigner)
	if signErr != nil {
		log.Fatalf("[gwclaims] signer init failed: %v", signErr)
	}
}

// MustInitVerifier is called at Gateway startup. Fails the process if no
// verifying key is configured.
func MustInitVerifier() {
	verifyOnce.Do(loadVerifier)
	if verifyErr != nil {
		log.Fatalf("[gwclaims] verifier init failed: %v", verifyErr)
	}
}
