# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary (auto-detect architecture)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /rayne ./cmd/migrate/main.go

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 rayne && \
    adduser -u 1000 -G rayne -s /bin/sh -D rayne

# Copy binary from builder
COPY --from=builder /rayne /app/rayne

# Set ownership
RUN chown -R rayne:rayne /app

USER rayne

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/rayne"]
