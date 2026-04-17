package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("GATEWAY_JWT_SECRET"))

func init() {
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("default_gateway_secret_change_me_in_production")
	}
}

type GatewayClaims struct {
	SessionID string `json:"session_id"`
	BuyerID   string `json:"buyer_id,omitempty"`
	Role      string `json:"role"` // "node" or "buyer"
	jwt.RegisteredClaims
}

func VerifyToken(tokenString string) (*GatewayClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &GatewayClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*GatewayClaims); ok && token.Valid {
		if claims.SessionID == "" {
			return nil, errors.New("invalid claims: session_id is empty")
		}
		if claims.Role != "node" && claims.Role != "buyer" {
			return nil, errors.New("invalid claims: unrecognized role")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
