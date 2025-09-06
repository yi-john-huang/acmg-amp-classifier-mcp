#!/bin/bash

# MCP ACMG/AMP Server Production Readiness Check Script
# This script validates that the deployment is ready for production use

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNING=0
TOTAL_CHECKS=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
    ((CHECKS_PASSED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $*"
    ((CHECKS_WARNING++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*"
    ((CHECKS_FAILED++))
}

log_check() {
    echo -e "${BLUE}[CHECK]${NC} $*"
    ((TOTAL_CHECKS++))
}

# Usage function
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

MCP ACMG/AMP Server Production Readiness Check

This script performs comprehensive checks to validate that your MCP server
deployment is ready for production use.

OPTIONS:
    -e, --env FILE          Environment file to check (default: .env)
    -t, --type TYPE         Deployment type (docker|kubernetes)
    -u, --url URL           Server URL to test (default: http://localhost:8080)
    -n, --namespace NAME    Kubernetes namespace (default: mcp-acmg-amp)
    -s, --skip-external     Skip external API checks
    -v, --verbose           Verbose output
    -h, --help              Show this help message

CHECKS PERFORMED:
    - Environment configuration validation
    - Security configuration review
    - Database connectivity and configuration
    - Redis connectivity and configuration
    - External API connectivity
    - SSL/TLS certificate validation
    - Resource limits and performance settings
    - Health and monitoring endpoints
    - Logging configuration
    - Backup and disaster recovery readiness
    - HIPAA compliance (if enabled)

EXIT CODES:
    0   All checks passed
    1   Critical checks failed
    2   Warnings present but no critical failures
EOF
}

# Check environment configuration
check_environment() {
    log_check "Checking environment configuration..."
    
    local env_file="${ENV_FILE:-$PROJECT_ROOT/.env}"
    
    if [[ ! -f "$env_file" ]]; then
        log_error "Environment file not found: $env_file"
        return
    fi
    
    # Source environment file
    set -a
    source "$env_file" 2>/dev/null || {
        log_error "Failed to source environment file"
        return
    }
    set +a
    
    # Check required variables
    local required_vars=(
        "POSTGRES_PASSWORD"
        "REDIS_PASSWORD"
    )
    
    local missing_vars=()
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            missing_vars+=("$var")
        fi
    done
    
    if [[ ${#missing_vars[@]} -eq 0 ]]; then
        log_success "All required environment variables are set"
    else
        log_error "Missing required environment variables: ${missing_vars[*]}"
        return
    fi
    
    # Check password strength
    if [[ ${#POSTGRES_PASSWORD} -lt 12 ]]; then
        log_warning "PostgreSQL password is less than 12 characters"
    else
        log_success "PostgreSQL password meets minimum length requirement"
    fi
    
    if [[ ${#REDIS_PASSWORD} -lt 12 ]]; then
        log_warning "Redis password is less than 12 characters"
    else
        log_success "Redis password meets minimum length requirement"
    fi
    
    # Check external API keys
    local api_keys=("CLINVAR_API_KEY" "GNOMAD_API_KEY" "COSMIC_API_KEY")
    local missing_keys=()
    
    for key in "${api_keys[@]}"; do
        if [[ -z "${!key:-}" ]]; then
            missing_keys+=("$key")
        fi
    done
    
    if [[ ${#missing_keys[@]} -eq 0 ]]; then
        log_success "All external API keys are configured"
    else
        log_warning "Missing optional API keys: ${missing_keys[*]}"
    fi
}

# Check security configuration
check_security() {
    log_check "Checking security configuration..."
    
    # Check TLS configuration
    if [[ "${MCP_TLS_ENABLED:-false}" == "true" ]]; then
        local cert_path="${MCP_TLS_CERT_PATH:-}"
        local key_path="${MCP_TLS_KEY_PATH:-}"
        
        if [[ -n "$cert_path" && -n "$key_path" ]]; then
            if [[ -f "$cert_path" && -f "$key_path" ]]; then
                # Check certificate validity
                if openssl x509 -in "$cert_path" -noout -checkend 2592000 2>/dev/null; then
                    log_success "TLS certificate is valid and not expiring within 30 days"
                else
                    log_warning "TLS certificate is expiring within 30 days or invalid"
                fi
                
                # Check certificate permissions
                local cert_perms=$(stat -f "%Op" "$cert_path" 2>/dev/null || stat -c "%a" "$cert_path" 2>/dev/null)
                local key_perms=$(stat -f "%Op" "$key_path" 2>/dev/null || stat -c "%a" "$key_path" 2>/dev/null)
                
                if [[ "$cert_perms" == *644 ]] && [[ "$key_perms" == *600 ]]; then
                    log_success "TLS certificate file permissions are correct"
                else
                    log_warning "TLS certificate file permissions may be insecure"
                fi
            else
                log_error "TLS certificate or key file not found"
            fi
        else
            log_error "TLS enabled but certificate paths not specified"
        fi
    else
        log_warning "TLS is disabled - not recommended for production"
    fi
    
    # Check JWT configuration
    if [[ "${AUTH_ENABLED:-false}" == "true" ]]; then
        if [[ -n "${JWT_SECRET:-}" ]]; then
            if [[ ${#JWT_SECRET} -ge 32 ]]; then
                log_success "JWT secret meets minimum length requirement"
            else
                log_error "JWT secret is less than 32 characters"
            fi
        else
            log_error "Authentication enabled but JWT secret not set"
        fi
    else
        log_info "Authentication is disabled"
    fi
    
    # Check HIPAA compliance settings
    if [[ "${HIPAA_COMPLIANCE_MODE:-false}" == "true" ]]; then
        local compliance_checks=(
            "AUDIT_LOG_ENABLED:true"
            "ANONYMIZE_PATIENT_DATA:true"
        )
        
        local compliance_issues=()
        for check in "${compliance_checks[@]}"; do
            local var_name="${check%:*}"
            local expected_value="${check#*:}"
            local actual_value="${!var_name:-}"
            
            if [[ "$actual_value" != "$expected_value" ]]; then
                compliance_issues+=("$var_name should be $expected_value")
            fi
        done
        
        if [[ ${#compliance_issues[@]} -eq 0 ]]; then
            log_success "HIPAA compliance settings are correct"
        else
            log_error "HIPAA compliance issues: ${compliance_issues[*]}"
        fi
    else
        log_info "HIPAA compliance mode is disabled"
    fi
}

# Check database connectivity
check_database() {
    log_check "Checking database connectivity..."
    
    local db_url="${DATABASE_URL:-}"
    if [[ -z "$db_url" ]]; then
        log_error "DATABASE_URL not set"
        return
    fi
    
    # Extract connection details
    local db_host db_port db_name db_user
    if [[ "$db_url" =~ postgresql://([^:]+):([^@]+)@([^:]+):([0-9]+)/([^?]+) ]]; then
        db_user="${BASH_REMATCH[1]}"
        db_host="${BASH_REMATCH[3]}"
        db_port="${BASH_REMATCH[4]}"
        db_name="${BASH_REMATCH[5]}"
    else
        log_error "Invalid DATABASE_URL format"
        return
    fi
    
    # Check database connectivity
    if command -v pg_isready &> /dev/null; then
        if pg_isready -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" &> /dev/null; then
            log_success "Database is accessible"
        else
            log_error "Cannot connect to database"
            return
        fi
    else
        log_warning "pg_isready not available, skipping database connectivity check"
    fi
    
    # Check database configuration (if psql is available)
    if command -v psql &> /dev/null; then
        local max_connections=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" -t -c "SHOW max_connections;" 2>/dev/null | xargs)
        
        if [[ -n "$max_connections" && "$max_connections" -ge 100 ]]; then
            log_success "Database max_connections ($max_connections) is adequate"
        elif [[ -n "$max_connections" ]]; then
            log_warning "Database max_connections ($max_connections) may be low for production"
        else
            log_warning "Could not check database max_connections setting"
        fi
        
        # Check shared_preload_libraries for performance extensions
        local shared_preload=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" -t -c "SHOW shared_preload_libraries;" 2>/dev/null | xargs)
        
        if [[ "$shared_preload" == *"pg_stat_statements"* ]]; then
            log_success "pg_stat_statements extension is loaded"
        else
            log_warning "pg_stat_statements extension not loaded (recommended for production)"
        fi
    else
        log_warning "psql not available, skipping detailed database checks"
    fi
}

# Check Redis connectivity
check_redis() {
    log_check "Checking Redis connectivity..."
    
    local redis_url="${REDIS_URL:-}"
    if [[ -z "$redis_url" ]]; then
        log_error "REDIS_URL not set"
        return
    fi
    
    # Extract connection details
    local redis_host redis_port
    if [[ "$redis_url" =~ redis://(:([^@]+)@)?([^:]+):([0-9]+) ]]; then
        redis_host="${BASH_REMATCH[3]}"
        redis_port="${BASH_REMATCH[4]}"
    else
        log_error "Invalid REDIS_URL format"
        return
    fi
    
    # Check Redis connectivity
    if command -v redis-cli &> /dev/null; then
        if redis-cli -h "$redis_host" -p "$redis_port" ping &> /dev/null; then
            log_success "Redis is accessible"
        else
            log_error "Cannot connect to Redis"
            return
        fi
        
        # Check Redis memory configuration
        local maxmemory=$(redis-cli -h "$redis_host" -p "$redis_port" CONFIG GET maxmemory 2>/dev/null | tail -1)
        
        if [[ -n "$maxmemory" && "$maxmemory" != "0" ]]; then
            log_success "Redis maxmemory is configured"
        else
            log_warning "Redis maxmemory not set (recommended for production)"
        fi
        
        # Check Redis persistence
        local save_config=$(redis-cli -h "$redis_host" -p "$redis_port" CONFIG GET save 2>/dev/null | tail -1)
        local aof_enabled=$(redis-cli -h "$redis_host" -p "$redis_port" CONFIG GET appendonly 2>/dev/null | tail -1)
        
        if [[ "$save_config" != '""' ]] || [[ "$aof_enabled" == "yes" ]]; then
            log_success "Redis persistence is configured"
        else
            log_warning "Redis persistence not configured (data may be lost on restart)"
        fi
    else
        log_warning "redis-cli not available, skipping Redis checks"
    fi
}

# Check external API connectivity
check_external_apis() {
    log_check "Checking external API connectivity..."
    
    if [[ "$SKIP_EXTERNAL" == "true" ]]; then
        log_info "Skipping external API checks (--skip-external flag)"
        return
    fi
    
    # Check ClinVar API
    if [[ -n "${CLINVAR_API_KEY:-}" ]]; then
        local clinvar_url="https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi?db=clinvar&id=1&retmode=json"
        if curl -f -s --max-time 10 "$clinvar_url" > /dev/null 2>&1; then
            log_success "ClinVar API is accessible"
        else
            log_warning "ClinVar API is not accessible"
        fi
    else
        log_warning "ClinVar API key not configured"
    fi
    
    # Check gnomAD API
    if [[ -n "${GNOMAD_API_KEY:-}" ]]; then
        local gnomad_url="https://gnomad.broadinstitute.org/api"
        if curl -f -s --max-time 10 "$gnomad_url" > /dev/null 2>&1; then
            log_success "gnomAD API is accessible"
        else
            log_warning "gnomAD API is not accessible"
        fi
    else
        log_warning "gnomAD API key not configured"
    fi
    
    # Check COSMIC API
    if [[ -n "${COSMIC_API_KEY:-}" ]]; then
        # COSMIC API check would go here
        log_info "COSMIC API check not implemented yet"
    else
        log_warning "COSMIC API key not configured"
    fi
}

# Check server health endpoints
check_server_health() {
    log_check "Checking server health endpoints..."
    
    local server_url="${SERVER_URL}"
    
    # Health endpoint
    if curl -f -s --max-time 10 "$server_url/health" > /dev/null 2>&1; then
        log_success "Health endpoint is accessible"
    else
        log_error "Health endpoint is not accessible"
    fi
    
    # Readiness endpoint
    if curl -f -s --max-time 10 "$server_url/ready" > /dev/null 2>&1; then
        log_success "Readiness endpoint is accessible"
    else
        log_warning "Readiness endpoint is not accessible"
    fi
    
    # Metrics endpoint
    if curl -f -s --max-time 10 "$server_url/metrics" > /dev/null 2>&1; then
        log_success "Metrics endpoint is accessible"
    else
        log_warning "Metrics endpoint is not accessible"
    fi
}

# Check resource limits
check_resources() {
    log_check "Checking resource limits and performance settings..."
    
    # Check memory limits
    local memory_limit="${MCP_SERVER_MEMORY_LIMIT:-}"
    if [[ -n "$memory_limit" ]]; then
        log_success "Memory limit is configured: $memory_limit"
    else
        log_warning "Memory limit not configured"
    fi
    
    # Check CPU limits
    local cpu_limit="${MCP_SERVER_CPU_LIMIT:-}"
    if [[ -n "$cpu_limit" ]]; then
        log_success "CPU limit is configured: $cpu_limit"
    else
        log_warning "CPU limit not configured"
    fi
    
    # Check connection limits
    local max_connections="${MCP_MAX_CONNECTIONS:-}"
    if [[ -n "$max_connections" && "$max_connections" -ge 100 ]]; then
        log_success "Maximum connections configured: $max_connections"
    else
        log_warning "Maximum connections not configured or too low"
    fi
    
    # Check cache configuration
    if [[ "${MCP_CACHE_ENABLED:-}" == "true" ]]; then
        log_success "Caching is enabled"
    else
        log_warning "Caching is disabled (may impact performance)"
    fi
    
    # Check compression
    if [[ "${MCP_COMPRESSION_ENABLED:-}" == "true" ]]; then
        log_success "Compression is enabled"
    else
        log_warning "Compression is disabled (may impact bandwidth usage)"
    fi
}

# Check logging configuration
check_logging() {
    log_check "Checking logging configuration..."
    
    # Check log level
    local log_level="${MCP_LOG_LEVEL:-info}"
    if [[ "$log_level" == "debug" ]]; then
        log_warning "Log level is set to debug (not recommended for production)"
    else
        log_success "Log level is appropriate: $log_level"
    fi
    
    # Check log format
    local log_format="${MCP_LOG_FORMAT:-}"
    if [[ "$log_format" == "json" ]]; then
        log_success "Log format is JSON (structured logging)"
    else
        log_warning "Log format is not JSON (structured logging recommended)"
    fi
    
    # Check audit logging
    if [[ "${AUDIT_LOG_ENABLED:-false}" == "true" ]]; then
        log_success "Audit logging is enabled"
    else
        log_warning "Audit logging is disabled"
    fi
}

# Check backup readiness
check_backup() {
    log_check "Checking backup and disaster recovery readiness..."
    
    # Check if backup is enabled
    if [[ "${BACKUP_ENABLED:-false}" == "true" ]]; then
        log_success "Backup is enabled"
        
        local backup_schedule="${BACKUP_SCHEDULE:-}"
        if [[ -n "$backup_schedule" ]]; then
            log_success "Backup schedule is configured: $backup_schedule"
        else
            log_warning "Backup schedule not configured"
        fi
    else
        log_error "Backup is not enabled (critical for production)"
    fi
    
    # Check backup retention
    local retention_days="${BACKUP_RETENTION_DAYS:-}"
    if [[ -n "$retention_days" && "$retention_days" -ge 30 ]]; then
        log_success "Backup retention is adequate: $retention_days days"
    else
        log_warning "Backup retention not configured or too short"
    fi
}

# Check deployment type specific items
check_deployment_type() {
    log_check "Checking deployment type specific configuration..."
    
    case "$DEPLOYMENT_TYPE" in
        docker)
            check_docker_deployment
            ;;
        kubernetes)
            check_kubernetes_deployment
            ;;
        *)
            log_info "Deployment type not specified, skipping specific checks"
            ;;
    esac
}

# Check Docker deployment
check_docker_deployment() {
    log_info "Checking Docker deployment..."
    
    # Check if docker-compose.yml exists
    if [[ -f "$PROJECT_ROOT/docker-compose.yml" ]]; then
        log_success "Docker Compose file exists"
    else
        log_error "Docker Compose file not found"
    fi
    
    # Check if services are running
    if command -v docker-compose &> /dev/null || docker compose version &> /dev/null; then
        local compose_cmd
        if command -v docker-compose &> /dev/null; then
            compose_cmd="docker-compose"
        else
            compose_cmd="docker compose"
        fi
        
        cd "$PROJECT_ROOT"
        local running_services=$($compose_cmd ps --services --filter "status=running" 2>/dev/null | wc -l)
        
        if [[ "$running_services" -gt 0 ]]; then
            log_success "$running_services Docker services are running"
        else
            log_warning "No Docker services are currently running"
        fi
    else
        log_warning "Docker Compose not available, skipping service checks"
    fi
}

# Check Kubernetes deployment
check_kubernetes_deployment() {
    log_info "Checking Kubernetes deployment..."
    
    local namespace="${KUBERNETES_NAMESPACE}"
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not available"
        return
    fi
    
    # Check if namespace exists
    if kubectl get namespace "$namespace" &> /dev/null; then
        log_success "Kubernetes namespace '$namespace' exists"
    else
        log_error "Kubernetes namespace '$namespace' does not exist"
        return
    fi
    
    # Check pod status
    local running_pods=$(kubectl get pods -n "$namespace" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
    local total_pods=$(kubectl get pods -n "$namespace" --no-headers 2>/dev/null | wc -l)
    
    if [[ "$running_pods" -eq "$total_pods" && "$total_pods" -gt 0 ]]; then
        log_success "All $total_pods pods are running"
    elif [[ "$total_pods" -gt 0 ]]; then
        log_warning "$running_pods/$total_pods pods are running"
    else
        log_error "No pods found in namespace $namespace"
    fi
    
    # Check service status
    local services=$(kubectl get services -n "$namespace" --no-headers 2>/dev/null | wc -l)
    if [[ "$services" -gt 0 ]]; then
        log_success "$services Kubernetes services are configured"
    else
        log_warning "No Kubernetes services found"
    fi
}

# Generate summary report
generate_summary() {
    echo
    echo "=================================================="
    echo "         PRODUCTION READINESS SUMMARY"
    echo "=================================================="
    echo
    echo -e "Total Checks: ${BLUE}$TOTAL_CHECKS${NC}"
    echo -e "Passed: ${GREEN}$CHECKS_PASSED${NC}"
    echo -e "Warnings: ${YELLOW}$CHECKS_WARNING${NC}"
    echo -e "Failed: ${RED}$CHECKS_FAILED${NC}"
    echo
    
    local pass_rate
    if [[ $TOTAL_CHECKS -gt 0 ]]; then
        pass_rate=$((CHECKS_PASSED * 100 / TOTAL_CHECKS))
    else
        pass_rate=0
    fi
    
    echo "Pass Rate: $pass_rate%"
    echo
    
    if [[ $CHECKS_FAILED -eq 0 ]]; then
        if [[ $CHECKS_WARNING -eq 0 ]]; then
            echo -e "${GREEN}✓ PRODUCTION READY${NC}"
            echo "All checks passed. Your MCP server is ready for production deployment."
        else
            echo -e "${YELLOW}⚠ PRODUCTION READY WITH WARNINGS${NC}"
            echo "All critical checks passed, but there are some warnings to address."
        fi
    else
        echo -e "${RED}✗ NOT PRODUCTION READY${NC}"
        echo "Critical issues found. Please address all failed checks before production deployment."
    fi
    
    echo
    echo "For detailed information on addressing issues, consult the deployment documentation."
}

# Parse command line arguments
ENV_FILE=""
DEPLOYMENT_TYPE=""
SERVER_URL="http://localhost:8080"
KUBERNETES_NAMESPACE="mcp-acmg-amp"
SKIP_EXTERNAL="false"
VERBOSE="false"

while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env)
            ENV_FILE="$2"
            shift 2
            ;;
        -t|--type)
            DEPLOYMENT_TYPE="$2"
            shift 2
            ;;
        -u|--url)
            SERVER_URL="$2"
            shift 2
            ;;
        -n|--namespace)
            KUBERNETES_NAMESPACE="$2"
            shift 2
            ;;
        -s|--skip-external)
            SKIP_EXTERNAL="true"
            shift
            ;;
        -v|--verbose)
            VERBOSE="true"
            set -x
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    echo "=============================================="
    echo "  MCP ACMG/AMP Server Production Readiness Check"
    echo "=============================================="
    echo
    
    # Run all checks
    check_environment
    check_security
    check_database
    check_redis
    check_external_apis
    check_server_health
    check_resources
    check_logging
    check_backup
    check_deployment_type
    
    # Generate summary
    generate_summary
    
    # Set exit code
    if [[ $CHECKS_FAILED -gt 0 ]]; then
        exit 1
    elif [[ $CHECKS_WARNING -gt 0 ]]; then
        exit 2
    else
        exit 0
    fi
}

# Run main function
main