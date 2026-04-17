package middleware

import (
	"context"
	"encoding/json"
	"exra/models"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const BuyerContextKey contextKey = "buyer"
const AdminContextKey contextKey = "admin"

// NodeAuth validates the request using a global shared secret.
// Used for internal/proxy authentication (e.g. buyer registration).
func NodeAuth(secret string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" || token != secret {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
}

type AdminActor struct {
	ID    string
	Email string
	Role  string
}

// BuyerAuth validates the buyer's API key from Authorization or X-API-Key header.
func BuyerAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractBearerToken(r)
		if apiKey == "" {
			jsonError(w, "missing authorization header", http.StatusUnauthorized)
			return
		}
		buyer, err := models.GetBuyerByAPIKey(apiKey)
		if err != nil {
			jsonError(w, "invalid api key", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), BuyerContextKey, buyer)
		next(w, r.WithContext(ctx))
	}
}

// DIDAuth validates the node signature against its registered PEAQ DID (public_key).
// Requirements: Headers X-Device-ID, X-Signature, X-Timestamp (ISO8601).
// Signature = Sign(DeviceID + ":" + Timestamp)
func DIDAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := r.Header.Get("X-Device-ID")
		did := r.Header.Get("X-DID")
		signature := r.Header.Get("X-Signature")
		timestamp := r.Header.Get("X-Timestamp")

		if deviceID == "" || signature == "" || timestamp == "" || did == "" {
			jsonError(w, "missing DID auth headers (X-Device-ID, X-DID, X-Signature, X-Timestamp)", http.StatusUnauthorized)
			return
		}

		// 1. Replay protection: verify timestamp is within last 5 minutes.
		ts, err := time.Parse(time.RFC3339, timestamp)
		if err != nil || time.Since(ts).Abs() > 5*time.Minute {
			jsonError(w, "invalid or expired timestamp (X-Timestamp)", http.StatusUnauthorized)
			return
		}

		// 2. Fetch public key from DB by DID (identity centric)
		var pubKeyHex string
		err = models.GetNodePublicKeyByDID(did, &pubKeyHex)
		if err != nil || pubKeyHex == "" {
			// Fallback: try by deviceID for legacy transition if DID is not yet indexed
			_ = models.GetNodePublicKey(deviceID, &pubKeyHex)
		}
		
		if pubKeyHex == "" {
			jsonError(w, "identity not registered or missing public key", http.StatusUnauthorized)
			return
		}

		// 3. Verify signature of the message (DeviceID:DID:Timestamp).
		msg := deviceID + ":" + did + ":" + timestamp
		ok, err := VerifyDIDSignature(pubKeyHex, msg, signature)
		if err != nil || !ok {
			jsonError(w, "invalid peaq DID signature", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func BuyerFromContext(r *http.Request) *models.Buyer {
	buyer, _ := r.Context().Value(BuyerContextKey).(*models.Buyer)
	return buyer
}

// AdminAuth validates the token against the admin_users table.
func AdminAuth(allowedRoles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				jsonError(w, "missing admin token", http.StatusUnauthorized)
				return
			}
			admin, err := models.GetAdminUserByAPIKey(token)
			if err != nil || !admin.IsActive {
				jsonError(w, "invalid admin token or inactive user", http.StatusForbidden)
				return
			}
			if len(allowedRoles) > 0 && !roleAllowed(admin.Role, allowedRoles) {
				jsonError(w, "insufficient admin role", http.StatusForbidden)
				return
			}
			ctx := context.WithValue(r.Context(), AdminContextKey, &AdminActor{
				ID:    admin.ID,
				Email: admin.Email,
				Role:  admin.Role,
			})
			next(w, r.WithContext(ctx))
		}
	}
}

func AdminFromContext(r *http.Request) *AdminActor {
	out, _ := r.Context().Value(AdminContextKey).(*AdminActor)
	return out
}

func roleAllowed(role string, allowed []string) bool {
	for _, v := range allowed {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(role)) {
			return true
		}
	}
	return false
}

func extractBearerToken(r *http.Request) string {
	if ExraToken := r.Header.Get("X-Exra-Token"); ExraToken != "" {
		return ExraToken
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	return ""
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
