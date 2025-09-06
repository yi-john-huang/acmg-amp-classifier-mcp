#!/bin/bash

# MCP ACMG/AMP Server Transport Configuration Script
# This script configures the MCP server for different transport modes

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG_DIR="${PROJECT_ROOT}/config"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# Usage function
usage() {
    cat << EOF
Usage: $0 [OPTIONS] COMMAND

MCP ACMG/AMP Server Transport Configuration Script

COMMANDS:
    configure           Configure transport settings
    stdio               Configure for stdio transport (Claude Desktop)
    http                Configure for HTTP transport
    websocket           Configure for WebSocket transport
    generate-certs      Generate TLS certificates
    validate            Validate transport configuration
    test                Test transport connectivity

TRANSPORT MODES:
    stdio               Standard input/output (for Claude Desktop integration)
    http                HTTP with JSON-RPC 2.0 (for web clients)
    websocket           WebSocket (for real-time applications)
    sse                 Server-Sent Events (for streaming)

OPTIONS:
    -t, --transport TYPE    Transport type (stdio|http|websocket|sse)
    -p, --port PORT         Port number for network transports
    -h, --host HOST         Host address (default: 0.0.0.0)
    -s, --ssl               Enable SSL/TLS
    -c, --cert-path PATH    Certificate file path
    -k, --key-path PATH     Private key file path
    -o, --output DIR        Output directory for config files
    -v, --verbose           Verbose output
    --help                  Show this help message

EXAMPLES:
    $0 stdio                                    # Configure for Claude Desktop
    $0 http --port 8080                        # Configure HTTP on port 8080
    $0 websocket --port 8081 --ssl             # Configure WebSocket with SSL
    $0 generate-certs                          # Generate self-signed certificates
    $0 test --transport http --port 8080       # Test HTTP transport
EOF
}

# Create config directories
create_config_dirs() {
    log_info "Creating configuration directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$CONFIG_DIR/transports"
    mkdir -p "$CONFIG_DIR/certs"
    mkdir -p "$CONFIG_DIR/nginx"
    mkdir -p "$CONFIG_DIR/nginx/conf.d"
    
    log_success "Configuration directories created"
}

# Configure stdio transport
configure_stdio() {
    log_info "Configuring stdio transport for Claude Desktop integration..."
    
    cat > "$CONFIG_DIR/transports/stdio.yaml" << 'EOF'
# MCP Server Configuration for stdio transport
# Used for Claude Desktop and other local AI agent integrations

server:
  name: "MCP ACMG/AMP Variant Classification Server"
  version: "1.0.0"

transport:
  type: "stdio"
  settings:
    # Standard input/output configuration
    input_buffer_size: 8192
    output_buffer_size: 8192
    line_buffered: true
    
    # Message handling
    max_message_size: 10485760  # 10MB
    message_timeout: 30s
    
    # Logging (be careful with stdio - logs should go to file)
    log_to_file: true
    log_file: "/app/logs/mcp-server.log"
    log_level: "info"

# MCP Protocol Configuration  
mcp:
  protocol_version: "2024-11-05"
  
  # Server capabilities
  capabilities:
    tools: true
    resources: true
    prompts: true
    logging: true
    
  # Tool configuration
  tools:
    enabled: true
    timeout: 60s
    max_concurrent: 10
    
  # Resource configuration
  resources:
    enabled: true
    cache_ttl: 300s
    max_size: 1048576  # 1MB
    
  # Prompt configuration
  prompts:
    enabled: true
    templates_dir: "/app/templates"

# Application settings
app:
  database_url: "${DATABASE_URL}"
  redis_url: "${REDIS_URL}"
  
  # External APIs
  external_apis:
    clinvar:
      api_key: "${CLINVAR_API_KEY}"
      base_url: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
      timeout: 30s
      rate_limit: 10
    
    gnomad:
      api_key: "${GNOMAD_API_KEY}"
      base_url: "https://gnomad.broadinstitute.org/api"
      timeout: 30s
      rate_limit: 5
    
    cosmic:
      api_key: "${COSMIC_API_KEY}"
      base_url: "https://cancer.sanger.ac.uk/cosmic/api"
      timeout: 30s
      rate_limit: 5

# Security settings
security:
  enable_audit_logging: true
  audit_log_file: "/app/logs/audit.log"
  anonymize_patient_data: true
  
# Performance settings
performance:
  cache_enabled: true
  compression_enabled: false  # Not needed for stdio
  connection_pool_size: 25
EOF

    # Create Claude Desktop configuration example
    cat > "$CONFIG_DIR/claude-desktop-config.json" << EOF
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "/path/to/mcp-server",
      "args": ["--config", "/path/to/config/transports/stdio.yaml"],
      "env": {
        "DATABASE_URL": "postgresql://user:password@localhost:5432/acmg_amp_mcp",
        "REDIS_URL": "redis://localhost:6379",
        "CLINVAR_API_KEY": "your_api_key",
        "GNOMAD_API_KEY": "your_api_key", 
        "COSMIC_API_KEY": "your_api_key"
      }
    }
  }
}
EOF

    log_success "Stdio transport configured"
    log_info "Claude Desktop config example created: $CONFIG_DIR/claude-desktop-config.json"
}

# Configure HTTP transport
configure_http() {
    local port="${PORT:-8080}"
    local host="${HOST:-0.0.0.0}"
    local ssl_enabled="${SSL_ENABLED:-false}"
    
    log_info "Configuring HTTP transport on $host:$port (SSL: $ssl_enabled)..."
    
    cat > "$CONFIG_DIR/transports/http.yaml" << EOF
# MCP Server Configuration for HTTP transport
# Used for web clients and HTTP-based integrations

server:
  name: "MCP ACMG/AMP Variant Classification Server"
  version: "1.0.0"

transport:
  type: "http"
  settings:
    host: "$host"
    port: $port
    ssl_enabled: $ssl_enabled
    
    # TLS Configuration (if SSL enabled)
    tls:
      cert_file: "${CERT_PATH:-/app/certs/server.crt}"
      key_file: "${KEY_PATH:-/app/certs/server.key}"
      min_version: "1.2"
      cipher_suites:
        - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
        - "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
    
    # HTTP Settings
    read_timeout: 30s
    write_timeout: 30s
    idle_timeout: 120s
    max_header_size: 1048576  # 1MB
    
    # CORS Configuration
    cors:
      enabled: true
      allowed_origins: ["*"]
      allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      allowed_headers: ["*"]
      expose_headers: ["X-Request-ID"]
      allow_credentials: false
      max_age: 86400
    
    # Rate Limiting
    rate_limit:
      enabled: true
      requests_per_minute: 60
      burst: 10
      key_func: "ip"  # ip, user, api_key

# MCP Protocol Configuration
mcp:
  protocol_version: "2024-11-05"
  
  # Server capabilities
  capabilities:
    tools: true
    resources: true
    prompts: true
    logging: true
    
  # JSON-RPC Configuration
  jsonrpc:
    version: "2.0"
    max_batch_size: 10
    allow_notifications: true
    
  # Tool configuration
  tools:
    enabled: true
    timeout: 60s
    max_concurrent: 50
    
  # Resource configuration
  resources:
    enabled: true
    cache_ttl: 300s
    max_size: 10485760  # 10MB
    compression_enabled: true
    
  # Prompt configuration
  prompts:
    enabled: true
    templates_dir: "/app/templates"

# Application settings
app:
  database_url: "\${DATABASE_URL}"
  redis_url: "\${REDIS_URL}"
  
  # External APIs
  external_apis:
    clinvar:
      api_key: "\${CLINVAR_API_KEY}"
      base_url: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
      timeout: 30s
      rate_limit: 10
    
    gnomad:
      api_key: "\${GNOMAD_API_KEY}"
      base_url: "https://gnomad.broadinstitute.org/api"
      timeout: 30s
      rate_limit: 5
    
    cosmic:
      api_key: "\${COSMIC_API_KEY}"
      base_url: "https://cancer.sanger.ac.uk/cosmic/api"
      timeout: 30s
      rate_limit: 5

# Logging configuration
logging:
  level: "info"
  format: "json"
  output: "stdout"
  
  # Access logging
  access_log:
    enabled: true
    format: "combined"
    output: "/app/logs/access.log"

# Security settings
security:
  enable_audit_logging: true
  audit_log_file: "/app/logs/audit.log"
  anonymize_patient_data: true
  
  # Optional authentication
  authentication:
    enabled: false
    type: "jwt"  # jwt, api_key
    jwt_secret: "\${JWT_SECRET}"
  
# Performance settings
performance:
  cache_enabled: true
  compression_enabled: true
  compression_threshold: 1024
  connection_pool_size: 100
  
# Health checks
health:
  enabled: true
  endpoint: "/health"
  detailed_endpoint: "/health/detailed"
  
# Metrics
metrics:
  enabled: true
  endpoint: "/metrics"
  port: 9090
EOF

    # Create Nginx configuration for HTTP transport
    create_nginx_config_http "$port" "$ssl_enabled"
    
    log_success "HTTP transport configured on $host:$port"
}

# Configure WebSocket transport
configure_websocket() {
    local port="${PORT:-8081}"
    local host="${HOST:-0.0.0.0}"
    local ssl_enabled="${SSL_ENABLED:-false}"
    
    log_info "Configuring WebSocket transport on $host:$port (SSL: $ssl_enabled)..."
    
    cat > "$CONFIG_DIR/transports/websocket.yaml" << EOF
# MCP Server Configuration for WebSocket transport
# Used for real-time applications and bi-directional communication

server:
  name: "MCP ACMG/AMP Variant Classification Server"
  version: "1.0.0"

transport:
  type: "websocket"
  settings:
    host: "$host"
    port: $port
    ssl_enabled: $ssl_enabled
    
    # WebSocket Settings
    path: "/mcp"
    origins: ["*"]
    
    # Connection settings
    read_buffer_size: 1024
    write_buffer_size: 1024
    handshake_timeout: 10s
    
    # Message settings
    max_message_size: 10485760  # 10MB
    ping_interval: 30s
    pong_timeout: 10s
    
    # TLS Configuration (if SSL enabled)
    tls:
      cert_file: "${CERT_PATH:-/app/certs/server.crt}"
      key_file: "${KEY_PATH:-/app/certs/server.key}"
      min_version: "1.2"

# MCP Protocol Configuration
mcp:
  protocol_version: "2024-11-05"
  
  # Server capabilities
  capabilities:
    tools: true
    resources: true
    prompts: true
    logging: true
    streaming: true  # WebSocket supports streaming
    
  # Tool configuration
  tools:
    enabled: true
    timeout: 60s
    max_concurrent: 100
    
  # Resource configuration
  resources:
    enabled: true
    cache_ttl: 300s
    max_size: 10485760  # 10MB
    
  # Prompt configuration
  prompts:
    enabled: true
    templates_dir: "/app/templates"
    
  # Streaming configuration
  streaming:
    enabled: true
    chunk_size: 8192
    flush_interval: 100ms

# Application settings (same as HTTP)
app:
  database_url: "\${DATABASE_URL}"
  redis_url: "\${REDIS_URL}"
  
  external_apis:
    clinvar:
      api_key: "\${CLINVAR_API_KEY}"
      base_url: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
      timeout: 30s
      rate_limit: 10
    
    gnomad:
      api_key: "\${GNOMAD_API_KEY}"
      base_url: "https://gnomad.broadinstitute.org/api"
      timeout: 30s
      rate_limit: 5
    
    cosmic:
      api_key: "\${COSMIC_API_KEY}"
      base_url: "https://cancer.sanger.ac.uk/cosmic/api"
      timeout: 30s
      rate_limit: 5

# Logging configuration
logging:
  level: "info"
  format: "json"
  output: "/app/logs/websocket.log"

# Security settings
security:
  enable_audit_logging: true
  audit_log_file: "/app/logs/audit.log"
  anonymize_patient_data: true

# Performance settings
performance:
  cache_enabled: true
  compression_enabled: true
  connection_pool_size: 200
EOF

    log_success "WebSocket transport configured on $host:$port"
}

# Create Nginx configuration for HTTP transport
create_nginx_config_http() {
    local upstream_port="$1"
    local ssl_enabled="$2"
    
    cat > "$CONFIG_DIR/nginx/conf.d/mcp-http.conf" << EOF
# Nginx configuration for MCP HTTP transport

upstream mcp_backend {
    server mcp-server:$upstream_port;
    
    # Health checking
    keepalive 32;
    keepalive_requests 100;
    keepalive_timeout 60s;
}

server {
    listen 80;
    server_name mcp-acmg-amp.example.com;
    
    # Security headers
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    
EOF

    if [[ "$ssl_enabled" == "true" ]]; then
        cat >> "$CONFIG_DIR/nginx/conf.d/mcp-http.conf" << 'EOF'
    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name mcp-acmg-amp.example.com;
    
    # SSL Configuration
    ssl_certificate /etc/nginx/certs/server.crt;
    ssl_certificate_key /etc/nginx/certs/server.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    
EOF
    fi

    cat >> "$CONFIG_DIR/nginx/conf.d/mcp-http.conf" << 'EOF'
    # General settings
    client_max_body_size 10M;
    client_body_timeout 30s;
    client_header_timeout 30s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types
        application/json
        application/javascript
        text/plain
        text/css
        text/xml
        text/javascript;
    
    # Rate limiting
    limit_req_zone $binary_remote_addr zone=mcp_limit:10m rate=10r/s;
    limit_req zone=mcp_limit burst=20 nodelay;
    
    # MCP API endpoints
    location / {
        proxy_pass http://mcp_backend;
        proxy_http_version 1.1;
        
        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "upgrade";
        proxy_set_header Upgrade $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
        
        # Buffering
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
    }
    
    # Health check endpoint
    location /health {
        access_log off;
        proxy_pass http://mcp_backend/health;
        proxy_set_header Host $host;
    }
    
    # Metrics endpoint (restrict access)
    location /metrics {
        allow 127.0.0.1;
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
        
        proxy_pass http://mcp_backend/metrics;
        proxy_set_header Host $host;
    }
}
EOF
}

# Generate TLS certificates
generate_certificates() {
    local cert_dir="${CONFIG_DIR}/certs"
    local domain="${DOMAIN:-localhost}"
    
    log_info "Generating self-signed TLS certificates for domain: $domain"
    
    mkdir -p "$cert_dir"
    
    # Generate private key
    openssl genrsa -out "$cert_dir/server.key" 2048
    
    # Generate certificate signing request
    cat > "$cert_dir/server.conf" << EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = CA
L = San Francisco
O = MCP ACMG/AMP Server
CN = $domain

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $domain
DNS.2 = localhost
DNS.3 = *.mcp-acmg-amp.example.com
IP.1 = 127.0.0.1
EOF

    # Generate certificate
    openssl req -new -x509 -key "$cert_dir/server.key" -out "$cert_dir/server.crt" \
        -days 365 -config "$cert_dir/server.conf" -extensions v3_req
    
    # Set appropriate permissions
    chmod 600 "$cert_dir/server.key"
    chmod 644 "$cert_dir/server.crt"
    
    log_success "TLS certificates generated in $cert_dir"
    log_warning "These are self-signed certificates for development use only"
}

# Validate configuration
validate_config() {
    local config_file="${1:-}"
    
    if [[ -z "$config_file" ]]; then
        log_error "Please specify configuration file to validate"
        exit 1
    fi
    
    if [[ ! -f "$config_file" ]]; then
        log_error "Configuration file not found: $config_file"
        exit 1
    fi
    
    log_info "Validating configuration: $config_file"
    
    # Check YAML syntax
    if command -v yq &> /dev/null; then
        if yq eval '.' "$config_file" > /dev/null 2>&1; then
            log_success "YAML syntax is valid"
        else
            log_error "YAML syntax error in $config_file"
            exit 1
        fi
    else
        log_warning "yq not found, skipping YAML syntax check"
    fi
    
    # Validate required sections
    local required_sections=("server" "transport" "mcp")
    
    for section in "${required_sections[@]}"; do
        if grep -q "^${section}:" "$config_file"; then
            log_info "✓ Section '$section' found"
        else
            log_error "✗ Required section '$section' missing"
            exit 1
        fi
    done
    
    log_success "Configuration validation passed"
}

# Test transport connectivity
test_transport() {
    local transport_type="${TRANSPORT:-http}"
    local port="${PORT:-8080}"
    local host="${HOST:-localhost}"
    
    log_info "Testing $transport_type transport connectivity on $host:$port..."
    
    case "$transport_type" in
        http)
            test_http_transport "$host" "$port"
            ;;
        websocket)
            test_websocket_transport "$host" "$port"
            ;;
        stdio)
            log_info "Stdio transport testing requires running the server manually"
            ;;
        *)
            log_error "Unknown transport type: $transport_type"
            exit 1
            ;;
    esac
}

# Test HTTP transport
test_http_transport() {
    local host="$1"
    local port="$2"
    local url="http://$host:$port"
    
    # Test health endpoint
    if curl -f -s --max-time 10 "$url/health" > /dev/null 2>&1; then
        log_success "✓ Health endpoint accessible"
    else
        log_error "✗ Health endpoint not accessible"
        return 1
    fi
    
    # Test MCP endpoint with basic JSON-RPC request
    local test_request='{"jsonrpc": "2.0", "method": "tools/list", "id": "test"}'
    
    if curl -f -s --max-time 10 -H "Content-Type: application/json" \
        -d "$test_request" "$url" > /dev/null 2>&1; then
        log_success "✓ MCP JSON-RPC endpoint accessible"
    else
        log_warning "✗ MCP JSON-RPC endpoint not accessible (server may not be running)"
    fi
}

# Test WebSocket transport
test_websocket_transport() {
    local host="$1"
    local port="$2"
    
    # This would require a WebSocket client tool
    log_info "WebSocket testing requires specialized tools (wscat, etc.)"
    log_info "Test URL would be: ws://$host:$port/mcp"
}

# Parse command line arguments
TRANSPORT=""
PORT=""
HOST=""
SSL_ENABLED="false"
CERT_PATH=""
KEY_PATH=""
OUTPUT_DIR=""
DOMAIN=""
VERBOSE="false"

while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--transport)
            TRANSPORT="$2"
            shift 2
            ;;
        -p|--port)
            PORT="$2"
            shift 2
            ;;
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -s|--ssl)
            SSL_ENABLED="true"
            shift
            ;;
        -c|--cert-path)
            CERT_PATH="$2"
            shift 2
            ;;
        -k|--key-path)
            KEY_PATH="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -d|--domain)
            DOMAIN="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE="true"
            set -x
            shift
            ;;
        --help)
            usage
            exit 0
            ;;
        configure|stdio|http|websocket|generate-certs|validate|test)
            COMMAND="$1"
            shift
            break
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check if command is provided
if [[ -z "${COMMAND:-}" ]]; then
    log_error "No command provided"
    usage
    exit 1
fi

# Override config dir if output specified
if [[ -n "$OUTPUT_DIR" ]]; then
    CONFIG_DIR="$OUTPUT_DIR"
fi

# Main execution
main() {
    create_config_dirs
    
    case "$COMMAND" in
        configure)
            if [[ -n "$TRANSPORT" ]]; then
                case "$TRANSPORT" in
                    stdio) configure_stdio ;;
                    http) configure_http ;;
                    websocket) configure_websocket ;;
                    *) log_error "Unknown transport: $TRANSPORT"; exit 1 ;;
                esac
            else
                log_error "Please specify transport type with --transport"
                exit 1
            fi
            ;;
        stdio)
            configure_stdio
            ;;
        http)
            configure_http
            ;;
        websocket)
            configure_websocket
            ;;
        generate-certs)
            generate_certificates
            ;;
        validate)
            validate_config "${1:-$CONFIG_DIR/transports/http.yaml}"
            ;;
        test)
            test_transport
            ;;
        *)
            log_error "Unknown command: $COMMAND"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"