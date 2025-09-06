# Multi-stage Dockerfile for MCP ACMG/AMP Server
# Optimized for production deployment with security and performance

# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    make \
    gcc \
    musl-dev \
    sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o mcp-server \
    ./cmd/mcp-server

# Stage 2: Security scanning stage (optional)
FROM alpine:3.18 AS security-scan

# Install security scanning tools
RUN apk add --no-cache trivy

# Copy binary for scanning
COPY --from=builder /app/mcp-server /tmp/

# Run security scan (this stage can be skipped in CI if needed)
RUN trivy fs --exit-code 0 --no-progress --severity HIGH,CRITICAL /tmp/

# Stage 3: Production stage
FROM alpine:3.18 AS production

# Create non-root user for security
RUN addgroup -g 10001 -S mcpuser && \
    adduser -u 10001 -S mcpuser -G mcpuser

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    jq \
    postgresql-client \
    redis \
    && rm -rf /var/cache/apk/*

# Set timezone
ENV TZ=UTC

# Create necessary directories
RUN mkdir -p /app/config /app/logs /app/data /app/scripts && \
    chown -R mcpuser:mcpuser /app

# Copy binary from builder stage
COPY --from=builder --chown=mcpuser:mcpuser /app/mcp-server /app/

# Copy configuration files
COPY --chown=mcpuser:mcpuser config/ /app/config/
COPY --chown=mcpuser:mcpuser scripts/ /app/scripts/

# Copy health check script
COPY --chown=mcpuser:mcpuser <<'EOF' /app/scripts/health-check.sh
#!/bin/sh
set -e

# Health check endpoint
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/health}"
TIMEOUT="${TIMEOUT:-10}"

# Check if server is responding
if curl -f -s --max-time "$TIMEOUT" "$HEALTH_URL" > /dev/null 2>&1; then
    echo "Health check passed"
    exit 0
else
    echo "Health check failed"
    exit 1
fi
EOF

RUN chmod +x /app/scripts/health-check.sh

# Copy readiness check script
COPY --chown=mcpuser:mcpuser <<'EOF' /app/scripts/readiness-check.sh
#!/bin/sh
set -e

# Readiness check endpoint
READINESS_URL="${READINESS_URL:-http://localhost:8080/ready}"
TIMEOUT="${TIMEOUT:-10}"

# Check if server is ready to handle requests
if curl -f -s --max-time "$TIMEOUT" "$READINESS_URL" | jq -e '.status == "ready"' > /dev/null 2>&1; then
    echo "Readiness check passed"
    exit 0
else
    echo "Readiness check failed"
    exit 1
fi
EOF

RUN chmod +x /app/scripts/readiness-check.sh

# Copy startup script
COPY --chown=mcpuser:mcpuser <<'EOF' /app/scripts/startup.sh
#!/bin/sh
set -e

echo "Starting MCP ACMG/AMP Server..."

# Environment validation
if [ -z "$DATABASE_URL" ]; then
    echo "ERROR: DATABASE_URL environment variable is required"
    exit 1
fi

# Wait for database to be ready
echo "Waiting for database..."
until pg_isready -d "$DATABASE_URL" -t 30; do
    echo "Database is not ready yet, waiting..."
    sleep 2
done
echo "Database is ready"

# Wait for Redis if configured
if [ -n "$REDIS_URL" ]; then
    echo "Waiting for Redis..."
    REDIS_HOST=$(echo "$REDIS_URL" | cut -d'/' -f3 | cut -d':' -f1)
    REDIS_PORT=$(echo "$REDIS_URL" | cut -d'/' -f3 | cut -d':' -f2)
    until redis-cli -h "$REDIS_HOST" -p "${REDIS_PORT:-6379}" ping > /dev/null 2>&1; do
        echo "Redis is not ready yet, waiting..."
        sleep 2
    done
    echo "Redis is ready"
fi

# Run database migrations if needed
if [ "$RUN_MIGRATIONS" = "true" ]; then
    echo "Running database migrations..."
    /app/mcp-server --migrate-only
fi

# Start the server
echo "Starting MCP server..."
exec /app/mcp-server "$@"
EOF

RUN chmod +x /app/scripts/startup.sh

# Switch to non-root user
USER mcpuser

# Set working directory
WORKDIR /app

# Expose ports
EXPOSE 8080 8443 9090

# Environment variables
ENV MCP_CONFIG_PATH=/app/config/config.yaml
ENV MCP_LOG_LEVEL=info
ENV MCP_LOG_FORMAT=json
ENV GIN_MODE=release
ENV GOMAXPROCS=0

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/app/scripts/health-check.sh"]

# Default command
ENTRYPOINT ["/app/scripts/startup.sh"]
CMD ["--config", "/app/config/config.yaml"]

# Metadata
LABEL maintainer="ACMG/AMP MCP Server Team" \
      version="1.0.0" \
      description="MCP ACMG/AMP Variant Classification Server" \
      org.opencontainers.image.source="https://github.com/yi-john-huang/acmg-amp-classifier-mcp" \
      org.opencontainers.image.documentation="https://github.com/yi-john-huang/acmg-amp-classifier-mcp/blob/main/README.md" \
      org.opencontainers.image.licenses="MIT"