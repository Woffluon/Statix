# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency manifests first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o statix ./cmd/statix

# Final Stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/statix /usr/local/bin/statix

# Expose default HTTP listening port
EXPOSE 8080

# Config directory volume
VOLUME ["/etc/statix"]

ENTRYPOINT ["/usr/local/bin/statix", "--config", "/etc/statix/config.yaml"]
