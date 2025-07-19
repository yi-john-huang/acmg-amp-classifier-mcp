#!/bin/bash

# Setup Docker Secrets for Production Deployment
# This script creates Docker secrets for secure credential management

set -e

echo "Setting up Docker secrets for ACMG-AMP MCP Server..."

# Function to create a secret if it doesn't exist
create_secret() {
    local secret_name=$1
    local secret_value=$2
    
    if ! docker secret ls --format "{{.Name}}" | grep -q "^${secret_name}$"; then
        echo "Creating secret: ${secret_name}"
        echo "${secret_value}" | docker secret create "${secret_name}" -
    else
        echo "Secret ${secret_name} already exists, skipping..."
    fi
}

# Generate secure random passwords and keys
DB_PASSWORD=$(openssl rand -base64 32)
POSTGRES_PASSWORD=$(openssl rand -base64 32)
REDIS_PASSWORD=$(openssl rand -base64 32)
JWT_SECRET=$(openssl rand -base64 64)
ENCRYPTION_KEY=$(openssl rand -base64 32)

# Prompt for API keys
echo "Please enter your COSMIC API key (or press Enter to skip):"
read -r COSMIC_API_KEY

# Create secrets
create_secret "acmg_amp_db_password" "${DB_PASSWORD}"
create_secret "acmg_amp_postgres_password" "${POSTGRES_PASSWORD}"
create_secret "acmg_amp_redis_password" "${REDIS_PASSWORD}"
create_secret "acmg_amp_jwt_secret" "${JWT_SECRET}"
create_secret "acmg_amp_encryption_key" "${ENCRYPTION_KEY}"

if [ -n "${COSMIC_API_KEY}" ]; then
    create_secret "acmg_amp_cosmic_api_key" "${COSMIC_API_KEY}"
fi

echo ""
echo "✅ Docker secrets created successfully!"
echo ""
echo "📝 Save these credentials securely:"
echo "Database Password: ${DB_PASSWORD}"
echo "PostgreSQL Password: ${POSTGRES_PASSWORD}"
echo "Redis Password: ${REDIS_PASSWORD}"
echo "JWT Secret: ${JWT_SECRET}"
echo "Encryption Key: ${ENCRYPTION_KEY}"
echo ""
echo "🚀 You can now deploy using: docker-compose -f docker-compose.prod.yml up -d"
echo ""
echo "⚠️  IMPORTANT: Store these credentials in a secure password manager!"
echo "⚠️  These passwords will not be displayed again!"