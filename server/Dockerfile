# --- Build Stage ---
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

# --- Final Stage ---
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates libc6-compat

# Run as non-root user
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
USER appuser

COPY --from=builder --chown=appuser:appgroup /app/main .
COPY --chown=appuser:appgroup GeoLite2-City.mmdb .

# NOTE: Do NOT copy .env into the image.
# Inject secrets at runtime via environment variables or a secrets manager
# (e.g., Docker secrets, AWS Secrets Manager, HashiCorp Vault).

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./main"]
