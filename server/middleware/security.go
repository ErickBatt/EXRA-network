package middleware

import "net/http"

// maxBodyBytes is the global request body size limit (1 MB).
// Prevents memory exhaustion from oversized payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

// LimitRequestBody wraps every request body with http.MaxBytesReader,
// capping it at 1 MiB. Handlers that attempt to read beyond this limit
// will receive an error, which surfaces as a 413 to the client.
func LimitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders adds defensive HTTP headers to every response.
// For a pure JSON API these are conservative but safe defaults.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Prevent MIME-type sniffing
		h.Set("X-Content-Type-Options", "nosniff")
		// Deny framing (clickjacking)
		h.Set("X-Frame-Options", "DENY")
		// Disable legacy XSS filter (modern browsers ignore it; older ones benefit)
		h.Set("X-XSS-Protection", "1; mode=block")
		// Strict HTTPS for 1 year (only meaningful when served over TLS)
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Minimal CSP for a JSON API — no scripts/styles served here
		h.Set("Content-Security-Policy", "default-src 'none'")
		// Don't leak the Referer header to third parties
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Disable browser features not needed by the API
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		next.ServeHTTP(w, r)
	})
}
