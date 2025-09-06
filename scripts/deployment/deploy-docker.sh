#!/bin/bash

# MCP ACMG/AMP Server Docker Deployment Script
# This script handles deployment of the MCP server using Docker Compose

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
DOCKER_COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.yml"

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

MCP ACMG/AMP Server Docker Deployment Script

COMMANDS:
    deploy              Deploy the MCP server stack
    start               Start the MCP server stack
    stop                Stop the MCP server stack
    restart             Restart the MCP server stack
    status              Show status of all services
    logs                Show logs from all services
    clean               Stop and remove all containers and volumes
    update              Pull latest images and restart services
    backup              Create backup of database and volumes
    restore             Restore from backup

OPTIONS:
    -e, --env FILE      Environment file (default: .env)
    -f, --file FILE     Docker compose file (default: docker-compose.yml)
    -p, --profile NAME  Docker compose profile (monitoring, proxy, tools)
    -d, --detach        Run in detached mode
    -v, --verbose       Verbose output
    -h, --help          Show this help message

EXAMPLES:
    $0 deploy                           # Deploy with default settings
    $0 deploy --profile monitoring     # Deploy with monitoring stack
    $0 start --detach                   # Start in background
    $0 logs mcp-server                  # Show logs for specific service
    $0 backup                           # Create backup
EOF
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed"
        exit 1
    fi
    
    # Check environment file
    if [[ ! -f "$ENV_FILE" ]]; then
        log_warning "Environment file $ENV_FILE not found"
        log_info "Creating default environment file from template..."
        if [[ -f "${PROJECT_ROOT}/.env.example" ]]; then
            cp "${PROJECT_ROOT}/.env.example" "$ENV_FILE"
            log_warning "Please edit $ENV_FILE with your configuration"
        else
            log_error "No environment template found"
            exit 1
        fi
    fi
    
    # Check Docker Compose file
    if [[ ! -f "$DOCKER_COMPOSE_FILE" ]]; then
        log_error "Docker Compose file $DOCKER_COMPOSE_FILE not found"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Validate configuration
validate_config() {
    log_info "Validating configuration..."
    
    # Source environment file
    set -a
    source "$ENV_FILE"
    set +a
    
    # Check required variables
    local required_vars=(
        "POSTGRES_PASSWORD"
        "REDIS_PASSWORD"
    )
    
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            log_error "Required environment variable $var is not set"
            exit 1
        fi
    done
    
    # Validate passwords strength
    if [[ ${#POSTGRES_PASSWORD} -lt 12 ]]; then
        log_warning "PostgreSQL password is less than 12 characters"
    fi
    
    if [[ ${#REDIS_PASSWORD} -lt 12 ]]; then
        log_warning "Redis password is less than 12 characters"
    fi
    
    log_success "Configuration validation passed"
}

# Get Docker Compose command
get_docker_compose_cmd() {
    if command -v docker-compose &> /dev/null; then
        echo "docker-compose"
    else
        echo "docker compose"
    fi
}

# Build services
build_services() {
    log_info "Building MCP server image..."
    
    local compose_cmd=$(get_docker_compose_cmd)
    
    cd "$PROJECT_ROOT"
    $compose_cmd build --no-cache mcp-server
    
    log_success "Build completed"
}

# Deploy services
deploy_services() {
    local profiles="${PROFILES:-}"
    local detach_flag=""
    
    if [[ "$DETACH" == "true" ]]; then
        detach_flag="--detach"
    fi
    
    log_info "Deploying MCP ACMG/AMP Server stack..."
    
    local compose_cmd=$(get_docker_compose_cmd)
    local profile_args=""
    
    if [[ -n "$profiles" ]]; then
        IFS=',' read -ra PROFILE_ARRAY <<< "$profiles"
        for profile in "${PROFILE_ARRAY[@]}"; do
            profile_args="$profile_args --profile $profile"
        done
    fi
    
    cd "$PROJECT_ROOT"
    
    # Pull images
    log_info "Pulling base images..."
    $compose_cmd $profile_args pull postgres redis nginx prometheus grafana
    
    # Build custom image
    build_services
    
    # Deploy
    log_info "Starting services..."
    $compose_cmd $profile_args up $detach_flag
    
    log_success "Deployment completed"
}

# Start services
start_services() {
    local profiles="${PROFILES:-}"
    local detach_flag=""
    
    if [[ "$DETACH" == "true" ]]; then
        detach_flag="--detach"
    fi
    
    log_info "Starting MCP server stack..."
    
    local compose_cmd=$(get_docker_compose_cmd)
    local profile_args=""
    
    if [[ -n "$profiles" ]]; then
        IFS=',' read -ra PROFILE_ARRAY <<< "$profiles"
        for profile in "${PROFILE_ARRAY[@]}"; do
            profile_args="$profile_args --profile $profile"
        done
    fi
    
    cd "$PROJECT_ROOT"
    $compose_cmd $profile_args up $detach_flag
    
    log_success "Services started"
}

# Stop services
stop_services() {
    log_info "Stopping MCP server stack..."
    
    local compose_cmd=$(get_docker_compose_cmd)
    
    cd "$PROJECT_ROOT"
    $compose_cmd down
    
    log_success "Services stopped"
}

# Restart services
restart_services() {
    log_info "Restarting MCP server stack..."
    stop_services
    start_services
}

# Show service status
show_status() {
    log_info "Service status:"
    
    local compose_cmd=$(get_docker_compose_cmd)
    
    cd "$PROJECT_ROOT"
    $compose_cmd ps
}

# Show logs
show_logs() {
    local service="${1:-}"
    local compose_cmd=$(get_docker_compose_cmd)
    
    cd "$PROJECT_ROOT"
    
    if [[ -n "$service" ]]; then
        log_info "Showing logs for service: $service"
        $compose_cmd logs --follow "$service"
    else
        log_info "Showing logs for all services:"
        $compose_cmd logs --follow
    fi
}

# Clean deployment
clean_deployment() {
    log_warning "This will stop and remove all containers, networks, and volumes!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Cleaning up deployment..."
        
        local compose_cmd=$(get_docker_compose_cmd)
        
        cd "$PROJECT_ROOT"
        $compose_cmd down --volumes --rmi local --remove-orphans
        
        log_success "Cleanup completed"
    else
        log_info "Cleanup cancelled"
    fi
}

# Update services
update_services() {
    log_info "Updating services..."
    
    local compose_cmd=$(get_docker_compose_cmd)
    
    cd "$PROJECT_ROOT"
    
    # Pull latest images
    $compose_cmd pull
    
    # Rebuild custom image
    build_services
    
    # Restart services
    $compose_cmd up --detach
    
    log_success "Update completed"
}

# Create backup
create_backup() {
    local backup_dir="${PROJECT_ROOT}/backups"
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    local backup_file="${backup_dir}/mcp_backup_${timestamp}.tar.gz"
    
    log_info "Creating backup..."
    
    mkdir -p "$backup_dir"
    
    # Create database dump
    log_info "Backing up PostgreSQL database..."
    docker exec mcp-postgres pg_dump -U mcpuser acmg_amp_mcp > "${backup_dir}/postgres_${timestamp}.sql"
    
    # Create volume backup
    log_info "Backing up volumes..."
    docker run --rm \
        -v mcp-acmg-amp-classifier-mcp_postgres_data:/data/postgres:ro \
        -v mcp-acmg-amp-classifier-mcp_redis_data:/data/redis:ro \
        -v "${backup_dir}":/backup \
        alpine:latest \
        tar czf "/backup/volumes_${timestamp}.tar.gz" -C /data .
    
    # Create complete backup
    tar czf "$backup_file" -C "${backup_dir}" "postgres_${timestamp}.sql" "volumes_${timestamp}.tar.gz"
    
    # Cleanup temporary files
    rm -f "${backup_dir}/postgres_${timestamp}.sql" "${backup_dir}/volumes_${timestamp}.tar.gz"
    
    log_success "Backup created: $backup_file"
}

# Restore from backup
restore_backup() {
    local backup_file="${1:-}"
    
    if [[ -z "$backup_file" ]]; then
        log_error "Please specify backup file"
        exit 1
    fi
    
    if [[ ! -f "$backup_file" ]]; then
        log_error "Backup file not found: $backup_file"
        exit 1
    fi
    
    log_warning "This will restore from backup and overwrite existing data!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Restoring from backup: $backup_file"
        
        # Stop services
        stop_services
        
        # Extract backup
        local temp_dir=$(mktemp -d)
        tar xzf "$backup_file" -C "$temp_dir"
        
        # Restore database
        log_info "Restoring PostgreSQL database..."
        start_services
        sleep 30 # Wait for database to be ready
        docker exec -i mcp-postgres psql -U mcpuser acmg_amp_mcp < "${temp_dir}/postgres_"*.sql
        
        # Restore volumes (would need to stop services and restore volumes)
        log_info "Volume restoration requires manual intervention"
        
        # Cleanup
        rm -rf "$temp_dir"
        
        log_success "Restore completed"
    else
        log_info "Restore cancelled"
    fi
}

# Parse command line arguments
DETACH="false"
PROFILES=""
VERBOSE="false"

while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env)
            ENV_FILE="$2"
            shift 2
            ;;
        -f|--file)
            DOCKER_COMPOSE_FILE="$2"
            shift 2
            ;;
        -p|--profile)
            PROFILES="$2"
            shift 2
            ;;
        -d|--detach)
            DETACH="true"
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
        deploy|start|stop|restart|status|logs|clean|update|backup|restore)
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

# Main execution
main() {
    check_prerequisites
    validate_config
    
    case "$COMMAND" in
        deploy)
            deploy_services
            ;;
        start)
            start_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            restart_services
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "${1:-}"
            ;;
        clean)
            clean_deployment
            ;;
        update)
            update_services
            ;;
        backup)
            create_backup
            ;;
        restore)
            restore_backup "${1:-}"
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