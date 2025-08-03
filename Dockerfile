# Purpose    : Multi-stage Docker build for pg-metako PostgreSQL replication manager
# Context    : Production-ready containerization with security and optimization
# Constraints: Must use minimal base image, non-root user, and efficient layer caching

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o pg-metako ./cmd/pg-metako

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1001 -S pgmetako && \
    adduser -u 1001 -S pgmetako -G pgmetako

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/pg-metako .

# Copy configuration directory
COPY --from=builder /app/configs ./configs

# Create directories for runtime
RUN mkdir -p /app/data /app/logs && \
    chown -R pgmetako:pgmetako /app

# Switch to non-root user
USER pgmetako

# Expose default port (if needed for future HTTP API)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep pg-metako || exit 1

# Default command
ENTRYPOINT ["./pg-metako"]
CMD ["--config", "configs/docker.yaml"]