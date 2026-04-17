package middleware

import (
	"net/http"
	"os"
	"strings"
)

// CORS returns a middleware that sets cross-origin headers for allowed origins.
// Allowed origins are read from the ALLOWED_ORIGINS environment variable
// (comma-separated list). If unset, defaults to denying all cross-origin requests.
func CORS(next http.Handler) http.Handler {
	raw := os.Getenv("ALLOWED_ORIGINS")
	allowed := map[string]bool{}
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, X-Exra-Token, X-API-Key, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
