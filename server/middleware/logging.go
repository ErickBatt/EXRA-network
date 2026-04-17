package middleware

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

type reqIDKey struct{}

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

// RequestLogger is a middleware that injects X-Request-ID and logs in JSON
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), reqIDKey{}, reqID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", reqID)

		start := time.Now()

		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)

		slog.Info("request processed",
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Int("status", rw.status),
			slog.Duration("latency", time.Since(start)),
		)
	})
}

// responseWriter captures the HTTP status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("responseWriter does not support hijacking")
	}
	return h.Hijack()
}

// RequestIDFromContext retrieves the request ID from a context
func RequestIDFromContext(ctx context.Context) string {
	if reqID, ok := ctx.Value(reqIDKey{}).(string); ok {
		return reqID
	}
	return ""
}

