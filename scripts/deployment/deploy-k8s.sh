#!/bin/bash

# MCP ACMG/AMP Server Kubernetes Deployment Script
# This script handles deployment of the MCP server on Kubernetes

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
K8S_MANIFESTS_DIR="${PROJECT_ROOT}/deployments/kubernetes"
NAMESPACE="mcp-acmg-amp"

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

MCP ACMG/AMP Server Kubernetes Deployment Script

COMMANDS:
    deploy              Deploy the MCP server to Kubernetes
    delete              Delete the MCP server from Kubernetes
    status              Show deployment status
    logs                Show pod logs
    scale               Scale deployment replicas
    rollout             Manage deployment rollouts
    port-forward        Port forward services
    exec                Execute command in pod
    describe            Describe Kubernetes resources
    backup              Backup Kubernetes resources and data
    restore             Restore from backup

OPTIONS:
    -n, --namespace NAME    Kubernetes namespace (default: mcp-acmg-amp)
    -k, --kubeconfig FILE   Kubeconfig file path
    -c, --context NAME      Kubernetes context
    -i, --image TAG         Docker image tag (default: latest)
    -r, --replicas NUM      Number of replicas (default: 3)
    -w, --wait              Wait for deployment to complete
    -v, --verbose           Verbose output
    -h, --help              Show this help message

EXAMPLES:
    $0 deploy                                    # Deploy with default settings
    $0 deploy --replicas 5 --image v1.2.0      # Deploy with custom settings
    $0 status                                    # Show deployment status
    $0 logs mcp-acmg-amp-server                 # Show logs for specific pod
    $0 scale --replicas 5                       # Scale to 5 replicas
    $0 port-forward 8080:8080                   # Port forward service
EOF
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    # Check kustomize
    if ! command -v kustomize &> /dev/null; then
        log_warning "kustomize is not installed, using kubectl kustomize"
    fi
    
    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check manifests directory
    if [[ ! -d "$K8S_MANIFESTS_DIR" ]]; then
        log_error "Kubernetes manifests directory not found: $K8S_MANIFESTS_DIR"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Validate cluster
validate_cluster() {
    log_info "Validating cluster..."
    
    # Check cluster version
    local cluster_version=$(kubectl version --short --client=false 2>/dev/null | grep "Server Version" | awk '{print $3}' || echo "unknown")
    log_info "Cluster version: $cluster_version"
    
    # Check available resources
    local nodes=$(kubectl get nodes --no-headers 2>/dev/null | wc -l || echo "0")
    log_info "Available nodes: $nodes"
    
    if [[ "$nodes" -eq 0 ]]; then
        log_error "No nodes available in cluster"
        exit 1
    fi
    
    # Check storage classes
    local storage_classes=$(kubectl get storageclass --no-headers 2>/dev/null | wc -l || echo "0")
    log_info "Available storage classes: $storage_classes"
    
    if [[ "$storage_classes" -eq 0 ]]; then
        log_warning "No storage classes found - persistent volumes may not work"
    fi
    
    log_success "Cluster validation passed"
}

# Create namespace
create_namespace() {
    log_info "Creating namespace: $NAMESPACE"
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace $NAMESPACE already exists"
    else
        kubectl apply -f "$K8S_MANIFESTS_DIR/namespace.yaml"
        log_success "Namespace created"
    fi
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying secrets..."
    
    # Check if secrets exist
    if kubectl get secret mcp-acmg-amp-secrets -n "$NAMESPACE" &> /dev/null; then
        log_warning "Secrets already exist. Please update them manually if needed."
    else
        log_warning "Please update the secrets in $K8S_MANIFESTS_DIR/secrets.yaml with your actual values"
        read -p "Have you updated the secrets? (y/N): " -n 1 -r
        echo
        
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_error "Please update secrets before deploying"
            exit 1
        fi
        
        kubectl apply -f "$K8S_MANIFESTS_DIR/secrets.yaml"
        log_success "Secrets deployed"
    fi
}

# Deploy application
deploy_application() {
    local image_tag="${IMAGE_TAG:-latest}"
    local replicas="${REPLICAS:-3}"
    local wait_flag=""
    
    if [[ "$WAIT" == "true" ]]; then
        wait_flag="--wait"
    fi
    
    log_info "Deploying MCP ACMG/AMP Server..."
    log_info "Image tag: $image_tag"
    log_info "Replicas: $replicas"
    
    # Use kustomize if available, otherwise kubectl
    if command -v kustomize &> /dev/null; then
        log_info "Using kustomize for deployment"
        cd "$K8S_MANIFESTS_DIR"
        
        # Update image tag in kustomization
        kustomize edit set image "mcp-acmg-amp-server=mcp-acmg-amp-server:$image_tag"
        
        # Update replicas
        kustomize edit set replicas "mcp-acmg-amp-server=$replicas"
        
        # Apply
        kustomize build . | kubectl apply $wait_flag -f -
    else
        log_info "Using kubectl for deployment"
        
        # Apply manifests in order
        kubectl apply -f "$K8S_MANIFESTS_DIR/configmap.yaml" $wait_flag
        kubectl apply -f "$K8S_MANIFESTS_DIR/postgresql.yaml" $wait_flag
        kubectl apply -f "$K8S_MANIFESTS_DIR/redis.yaml" $wait_flag
        kubectl apply -f "$K8S_MANIFESTS_DIR/mcp-server.yaml" $wait_flag
        kubectl apply -f "$K8S_MANIFESTS_DIR/ingress.yaml" $wait_flag
        kubectl apply -f "$K8S_MANIFESTS_DIR/hpa.yaml" $wait_flag
        kubectl apply -f "$K8S_MANIFESTS_DIR/monitoring.yaml" $wait_flag
        
        # Update image
        kubectl set image deployment/mcp-acmg-amp-server mcp-server="mcp-acmg-amp-server:$image_tag" -n "$NAMESPACE"
        
        # Scale deployment
        kubectl scale deployment mcp-acmg-amp-server --replicas="$replicas" -n "$NAMESPACE"
    fi
    
    log_success "Deployment completed"
}

# Wait for deployment
wait_for_deployment() {
    log_info "Waiting for deployment to be ready..."
    
    # Wait for PostgreSQL
    log_info "Waiting for PostgreSQL..."
    kubectl wait --for=condition=available deployment/postgresql -n "$NAMESPACE" --timeout=300s
    
    # Wait for Redis
    log_info "Waiting for Redis..."
    kubectl wait --for=condition=available deployment/redis -n "$NAMESPACE" --timeout=300s
    
    # Wait for MCP Server
    log_info "Waiting for MCP Server..."
    kubectl wait --for=condition=available deployment/mcp-acmg-amp-server -n "$NAMESPACE" --timeout=600s
    
    # Check pod status
    kubectl get pods -n "$NAMESPACE"
    
    log_success "All deployments are ready"
}

# Delete deployment
delete_deployment() {
    log_warning "This will delete the entire MCP deployment!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Deleting MCP deployment..."
        
        # Delete in reverse order
        kubectl delete -f "$K8S_MANIFESTS_DIR/monitoring.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/hpa.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/ingress.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/mcp-server.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/redis.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/postgresql.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/configmap.yaml" --ignore-not-found
        kubectl delete -f "$K8S_MANIFESTS_DIR/secrets.yaml" --ignore-not-found
        
        # Optionally delete namespace
        read -p "Delete namespace $NAMESPACE? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kubectl delete namespace "$NAMESPACE" --ignore-not-found
        fi
        
        log_success "Deployment deleted"
    else
        log_info "Deletion cancelled"
    fi
}

# Show deployment status
show_status() {
    log_info "Deployment status for namespace: $NAMESPACE"
    
    echo
    echo "=== Namespaces ==="
    kubectl get namespace "$NAMESPACE" 2>/dev/null || echo "Namespace not found"
    
    echo
    echo "=== Pods ==="
    kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || echo "No pods found"
    
    echo
    echo "=== Services ==="
    kubectl get services -n "$NAMESPACE" 2>/dev/null || echo "No services found"
    
    echo
    echo "=== Deployments ==="
    kubectl get deployments -n "$NAMESPACE" 2>/dev/null || echo "No deployments found"
    
    echo
    echo "=== Persistent Volume Claims ==="
    kubectl get pvc -n "$NAMESPACE" 2>/dev/null || echo "No PVCs found"
    
    echo
    echo "=== Ingress ==="
    kubectl get ingress -n "$NAMESPACE" 2>/dev/null || echo "No ingress found"
    
    echo
    echo "=== HPA ==="
    kubectl get hpa -n "$NAMESPACE" 2>/dev/null || echo "No HPA found"
}

# Show pod logs
show_logs() {
    local pod_pattern="${1:-mcp-acmg-amp-server}"
    
    log_info "Showing logs for pods matching: $pod_pattern"
    
    local pods=$(kubectl get pods -n "$NAMESPACE" -o name | grep "$pod_pattern" | head -1)
    
    if [[ -z "$pods" ]]; then
        log_error "No pods found matching: $pod_pattern"
        exit 1
    fi
    
    kubectl logs -f -n "$NAMESPACE" "$pods"
}

# Scale deployment
scale_deployment() {
    local replicas="${REPLICAS:-3}"
    
    log_info "Scaling deployment to $replicas replicas..."
    
    kubectl scale deployment mcp-acmg-amp-server --replicas="$replicas" -n "$NAMESPACE"
    
    log_success "Deployment scaled to $replicas replicas"
}

# Port forward
port_forward() {
    local port_mapping="${1:-8080:8080}"
    
    log_info "Port forwarding: $port_mapping"
    
    local service="mcp-acmg-amp-server"
    kubectl port-forward service/"$service" -n "$NAMESPACE" "$port_mapping"
}

# Execute command in pod
exec_pod() {
    local command="${1:-sh}"
    local pod_pattern="${2:-mcp-acmg-amp-server}"
    
    log_info "Executing command in pod: $command"
    
    local pods=$(kubectl get pods -n "$NAMESPACE" -o name | grep "$pod_pattern" | head -1)
    
    if [[ -z "$pods" ]]; then
        log_error "No pods found matching: $pod_pattern"
        exit 1
    fi
    
    kubectl exec -it -n "$NAMESPACE" "$pods" -- "$command"
}

# Describe resources
describe_resource() {
    local resource_type="${1:-pods}"
    local resource_name="${2:-}"
    
    if [[ -n "$resource_name" ]]; then
        kubectl describe "$resource_type" "$resource_name" -n "$NAMESPACE"
    else
        kubectl describe "$resource_type" -n "$NAMESPACE"
    fi
}

# Create backup
create_backup() {
    local backup_dir="${PROJECT_ROOT}/backups/k8s"
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    
    log_info "Creating Kubernetes backup..."
    
    mkdir -p "$backup_dir"
    
    # Backup manifests
    log_info "Backing up Kubernetes manifests..."
    kubectl get all,pvc,secrets,configmaps,ingress,hpa,networkpolicies -n "$NAMESPACE" -o yaml > "${backup_dir}/manifests_${timestamp}.yaml"
    
    # Backup database (if possible)
    log_info "Backing up database..."
    local postgres_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=postgresql -o name | head -1)
    if [[ -n "$postgres_pod" ]]; then
        kubectl exec -n "$NAMESPACE" "$postgres_pod" -- pg_dump -U mcpuser acmg_amp_mcp > "${backup_dir}/postgres_${timestamp}.sql"
    fi
    
    log_success "Backup created in $backup_dir"
}

# Restore from backup
restore_backup() {
    local manifest_file="${1:-}"
    
    if [[ -z "$manifest_file" ]]; then
        log_error "Please specify manifest backup file"
        exit 1
    fi
    
    if [[ ! -f "$manifest_file" ]]; then
        log_error "Backup file not found: $manifest_file"
        exit 1
    fi
    
    log_warning "This will restore from backup and may overwrite existing resources!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Restoring from backup: $manifest_file"
        kubectl apply -f "$manifest_file"
        log_success "Restore completed"
    else
        log_info "Restore cancelled"
    fi
}

# Parse command line arguments
IMAGE_TAG="latest"
REPLICAS="3"
WAIT="false"
VERBOSE="false"

while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -k|--kubeconfig)
            export KUBECONFIG="$2"
            shift 2
            ;;
        -c|--context)
            kubectl config use-context "$2"
            shift 2
            ;;
        -i|--image)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -r|--replicas)
            REPLICAS="$2"
            shift 2
            ;;
        -w|--wait)
            WAIT="true"
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
        deploy|delete|status|logs|scale|rollout|port-forward|exec|describe|backup|restore)
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
    validate_cluster
    
    case "$COMMAND" in
        deploy)
            create_namespace
            deploy_secrets
            deploy_application
            if [[ "$WAIT" == "true" ]]; then
                wait_for_deployment
            fi
            ;;
        delete)
            delete_deployment
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "${1:-}"
            ;;
        scale)
            scale_deployment
            ;;
        rollout)
            kubectl rollout status deployment/mcp-acmg-amp-server -n "$NAMESPACE"
            ;;
        port-forward)
            port_forward "${1:-}"
            ;;
        exec)
            exec_pod "${1:-}" "${2:-}"
            ;;
        describe)
            describe_resource "${1:-}" "${2:-}"
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