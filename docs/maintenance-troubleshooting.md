# Maintenance & Troubleshooting Guide

Comprehensive operations and troubleshooting guide for system administrators managing the ACMG/AMP MCP Server.

## Table of Contents

1. [System Overview](#system-overview)
2. [Daily Operations](#daily-operations)
3. [Monitoring & Alerting](#monitoring--alerting)
4. [Troubleshooting](#troubleshooting)
5. [Performance Optimization](#performance-optimization)
6. [Backup & Recovery](#backup--recovery)
7. [Updates & Patches](#updates--patches)
8. [Emergency Procedures](#emergency-procedures)

---

## System Overview

### Architecture Components

```
┌─────────────────────────────────────────────────────────┐
│                Load Balancer                           │
│               (nginx/HAProxy)                          │
└─────────────────────┬───────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────┐
│              MCP Server Cluster                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │ MCP Server  │  │ MCP Server  │  │ MCP Server  │    │
│  │   Node 1    │  │   Node 2    │  │   Node 3    │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
└─────────────────────┬───────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────┐
│                Data Layer                              │
│  ┌─────────────────┐        ┌─────────────────┐       │
│  │   PostgreSQL    │        │     Redis       │       │
│  │   (Primary/     │        │    (Cache)      │       │
│  │    Replica)     │        │                 │       │
│  └─────────────────┘        └─────────────────┘       │
└─────────────────────────────────────────────────────────┘
```

### Service Dependencies

| Component | Dependencies | Critical? | Failover |
|-----------|--------------|-----------|----------|
| **MCP Server** | PostgreSQL, Redis | Yes | Auto-restart, clustering |
| **PostgreSQL** | Storage, Network | Yes | Primary/replica failover |
| **Redis** | Memory, Network | No | Degraded performance only |
| **Load Balancer** | Network | Yes | Secondary LB available |
| **External APIs** | Internet, API keys | No | Cached data available |

### Key Configuration Files

```bash
# Main server configuration
/opt/acmg-amp-mcp/config/production.yaml

# Service definitions  
/etc/systemd/system/mcp-server.service
/etc/systemd/system/mcp-server-worker.service

# Database configuration
/etc/postgresql/14/main/postgresql.conf
/etc/postgresql/14/main/pg_hba.conf

# Cache configuration
/etc/redis/redis.conf

# Web server configuration
/etc/nginx/sites-available/mcp-server
/etc/haproxy/haproxy.cfg

# Log configuration
/etc/rsyslog.d/mcp-server.conf
/etc/logrotate.d/mcp-server
```

---

## Daily Operations

### Morning Health Check

#### Automated Daily Report
```bash
#!/bin/bash
# Daily health check script
# File: /opt/acmg-amp-mcp/scripts/daily-health-check.sh

echo "=== ACMG/AMP MCP Server Daily Health Report ==="
echo "Date: $(date)"
echo

# Service status
echo "### Service Status ###"
systemctl is-active mcp-server && echo "✅ MCP Server: Running" || echo "❌ MCP Server: Stopped"
systemctl is-active postgresql && echo "✅ PostgreSQL: Running" || echo "❌ PostgreSQL: Stopped" 
systemctl is-active redis && echo "✅ Redis: Running" || echo "❌ Redis: Stopped"
systemctl is-active nginx && echo "✅ Nginx: Running" || echo "❌ Nginx: Stopped"

# Database health
echo -e "\n### Database Health ###"
POSTGRES_STATUS=$(sudo -u postgres psql -c "SELECT 1;" 2>/dev/null && echo "✅ Connected" || echo "❌ Connection failed")
echo "PostgreSQL: $POSTGRES_STATUS"

DB_SIZE=$(sudo -u postgres psql -d acmg_amp_mcp -c "SELECT pg_size_pretty(pg_database_size('acmg_amp_mcp'));" -t 2>/dev/null | xargs)
echo "Database size: $DB_SIZE"

# Redis health  
echo -e "\n### Cache Health ###"
REDIS_STATUS=$(redis-cli ping 2>/dev/null | grep PONG >/dev/null && echo "✅ Connected" || echo "❌ Connection failed")
echo "Redis: $REDIS_STATUS"

REDIS_MEMORY=$(redis-cli info memory | grep used_memory_human | cut -d: -f2 | tr -d '\r')
echo "Redis memory usage: $REDIS_MEMORY"

# Disk space
echo -e "\n### Disk Space ###"
df -h | grep -E '(Filesystem|/opt|/var|/tmp)' | awk '{print $1 ": " $5 " used (" $4 " available)"}'

# API connectivity
echo -e "\n### External APIs ###"
curl -s -o /dev/null -w "ClinVar: %{http_code}\n" "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/einfo.fcgi?db=clinvar" --max-time 10
curl -s -o /dev/null -w "gnomAD: %{http_code}\n" "https://gnomad.broadinstitute.org/api/" --max-time 10

# Recent errors
echo -e "\n### Recent Errors (Last 24h) ###"
ERROR_COUNT=$(journalctl -u mcp-server --since "24 hours ago" | grep -c ERROR || echo "0")
WARNING_COUNT=$(journalctl -u mcp-server --since "24 hours ago" | grep -c WARNING || echo "0")
echo "Errors: $ERROR_COUNT, Warnings: $WARNING_COUNT"

if [ $ERROR_COUNT -gt 0 ]; then
    echo "Recent errors:"
    journalctl -u mcp-server --since "24 hours ago" | grep ERROR | tail -5
fi

echo -e "\n=== End of Report ==="
```

#### Schedule Daily Check
```bash
# Add to crontab
0 8 * * * /opt/acmg-amp-mcp/scripts/daily-health-check.sh | mail -s "ACMG/AMP MCP Daily Report" admin@hospital.org
```

### Weekly Maintenance Tasks

#### Database Maintenance
```bash
#!/bin/bash
# Weekly database maintenance
# File: /opt/acmg-amp-mcp/scripts/weekly-db-maintenance.sh

echo "Starting weekly database maintenance..."

# Update database statistics
sudo -u postgres psql -d acmg_amp_mcp -c "ANALYZE;"

# Vacuum database
sudo -u postgres psql -d acmg_amp_mcp -c "VACUUM ANALYZE;"

# Check for dead tuples
DEAD_TUPLES=$(sudo -u postgres psql -d acmg_amp_mcp -c "SELECT schemaname,tablename,n_dead_tup FROM pg_stat_user_tables WHERE n_dead_tup > 1000;" -t | wc -l)
if [ $DEAD_TUPLES -gt 0 ]; then
    echo "Tables with high dead tuple counts detected - consider VACUUM FULL during maintenance window"
fi

# Update external database caches
curl -X POST http://localhost:8080/admin/refresh-cache \
     -H "Authorization: Bearer $ADMIN_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"databases": ["clinvar", "gnomad"]}'

# Rotate logs
logrotate -f /etc/logrotate.d/mcp-server

echo "Weekly maintenance completed"
```

### Monthly Tasks

#### Performance Review
```bash
#!/bin/bash
# Monthly performance analysis
# File: /opt/acmg-amp-mcp/scripts/monthly-performance-review.sh

# Query performance analysis
sudo -u postgres psql -d acmg_amp_mcp -c "
SELECT query, calls, total_time, mean_time, stddev_time, rows, hit_percent
FROM pg_stat_statements 
ORDER BY total_time DESC 
LIMIT 10;
"

# Index usage analysis  
sudo -u postgres psql -d acmg_amp_mcp -c "
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes 
WHERE idx_scan = 0 AND schemaname = 'public';
"

# Cache hit ratios
sudo -u postgres psql -d acmg_amp_mcp -c "
SELECT 
    'index hit rate' as name,
    (sum(idx_blks_hit)) / (sum(idx_blks_hit) + sum(idx_blks_read)) as ratio
FROM pg_statio_user_indexes 
UNION ALL
SELECT 
    'table hit rate' as name,
    sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as ratio
FROM pg_statio_user_tables;
"
```

---

## Monitoring & Alerting

### Key Metrics to Monitor

#### System Health Metrics
```yaml
monitoring_metrics:
  system:
    - cpu_usage_percent
    - memory_usage_percent  
    - disk_usage_percent
    - network_io_bytes
    - load_average
  
  application:
    - request_rate_per_second
    - response_time_p95
    - error_rate_percent
    - active_connections
    - queue_depth
  
  database:
    - connection_count
    - query_duration_p95
    - cache_hit_ratio
    - deadlock_count
    - slow_query_count
```

#### Alert Thresholds
```yaml
alert_thresholds:
  critical:
    cpu_usage: 90%
    memory_usage: 95%
    disk_usage: 90%
    error_rate: 5%
    response_time_p95: 10s
  
  warning:
    cpu_usage: 80%
    memory_usage: 85%
    disk_usage: 80%
    error_rate: 2%
    response_time_p95: 5s
```

### Grafana Dashboard Configuration

#### System Dashboard Panels
```json
{
  "dashboard": {
    "title": "ACMG/AMP MCP Server Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(mcp_requests_total[5m])",
            "legendFormat": "{{method}}"
          }
        ]
      },
      {
        "title": "Response Times",
        "type": "graph", 
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(mcp_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      },
      {
        "title": "Error Rates",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(mcp_errors_total[5m])",
            "legendFormat": "{{error_type}}"
          }
        ]
      }
    ]
  }
}
```

### Alerting Rules

#### Prometheus Alerting Rules
```yaml
# alerts.yml
groups:
  - name: mcp_server_alerts
    rules:
      - alert: MCPServerDown
        expr: up{job="mcp-server"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "MCP Server is down"
          description: "MCP Server has been down for more than 1 minute"
      
      - alert: HighErrorRate
        expr: rate(mcp_errors_total[5m]) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"
      
      - alert: SlowResponses
        expr: histogram_quantile(0.95, rate(mcp_request_duration_seconds_bucket[5m])) > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Slow response times"
          description: "95th percentile response time is {{ $value }}s"
      
      - alert: DatabaseConnectionHigh
        expr: pg_stat_activity_count > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High database connection count"
          description: "Database has {{ $value }} active connections"
```

---

## Troubleshooting

### Common Issues and Solutions

#### Issue: MCP Server Won't Start

**Symptoms**:
- Service fails to start
- Error messages in systemctl status
- Application logs show startup errors

**Diagnostic Steps**:
```bash
# Check service status
systemctl status mcp-server

# Check application logs
journalctl -u mcp-server -n 50

# Check configuration syntax
/opt/acmg-amp-mcp/bin/mcp-server --config /opt/acmg-amp-mcp/config/production.yaml --validate

# Check port availability
netstat -tulpn | grep :8080

# Check dependencies
systemctl status postgresql redis nginx
```

**Common Causes & Solutions**:

1. **Configuration Error**:
   ```bash
   # Fix configuration file
   vim /opt/acmg-amp-mcp/config/production.yaml
   # Validate syntax
   yamllint /opt/acmg-amp-mcp/config/production.yaml
   ```

2. **Database Connection Failed**:
   ```bash
   # Test database connection
   sudo -u postgres psql -d acmg_amp_mcp -c "SELECT 1;"
   # Check connection parameters in config
   # Restart PostgreSQL if needed
   systemctl restart postgresql
   ```

3. **Port Already in Use**:
   ```bash
   # Find process using port
   lsof -i :8080
   # Kill process or change port in config
   ```

#### Issue: High Memory Usage

**Symptoms**:
- System running out of memory
- OOM killer terminating processes
- Slow performance

**Diagnostic Steps**:
```bash
# Check memory usage
free -h
ps aux --sort=-%mem | head -20

# Check MCP server memory usage
systemctl status mcp-server
cat /proc/$(pgrep mcp-server)/status | grep -E "(VmSize|VmRSS)"

# Check for memory leaks
valgrind --tool=massif --time-unit=B /opt/acmg-amp-mcp/bin/mcp-server --config config/test.yaml
```

**Solutions**:

1. **Increase System Memory**:
   ```bash
   # Add swap if needed (temporary solution)
   fallocate -l 2G /swapfile
   chmod 600 /swapfile
   mkswap /swapfile
   swapon /swapfile
   ```

2. **Optimize Application Settings**:
   ```yaml
   # config/production.yaml
   performance:
     max_connections: 100  # Reduce if high memory usage
     cache_size: 512MB     # Adjust based on available memory
     worker_pool_size: 4   # Match CPU cores
   ```

3. **Enable Memory Monitoring**:
   ```bash
   # Add memory alerts
   echo "mcp-server memory usage: $(ps -o pid,vsz,rss,comm -p $(pgrep mcp-server))" | mail -s "Memory Alert" admin@hospital.org
   ```

#### Issue: Database Performance Problems

**Symptoms**:
- Slow query responses
- High CPU usage on database server
- Connection timeouts

**Diagnostic Steps**:
```bash
# Check database performance
sudo -u postgres psql -d acmg_amp_mcp -c "SELECT * FROM pg_stat_activity WHERE state = 'active';"

# Check for long-running queries
sudo -u postgres psql -d acmg_amp_mcp -c "SELECT pid, now() - pg_stat_activity.query_start AS duration, query FROM pg_stat_activity WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';"

# Check database locks
sudo -u postgres psql -d acmg_amp_mcp -c "SELECT * FROM pg_locks l JOIN pg_stat_activity a ON l.pid = a.pid WHERE NOT l.granted;"

# Analyze table statistics
sudo -u postgres psql -d acmg_amp_mcp -c "SELECT schemaname, tablename, seq_scan, seq_tup_read, idx_scan, idx_tup_fetch FROM pg_stat_user_tables ORDER BY seq_tup_read DESC;"
```

**Solutions**:

1. **Query Optimization**:
   ```sql
   -- Add missing indexes
   CREATE INDEX CONCURRENTLY idx_variants_hgvs ON variants(hgvs);
   CREATE INDEX CONCURRENTLY idx_evidence_variant_id ON evidence(variant_id);
   
   -- Update table statistics
   ANALYZE variants;
   ANALYZE evidence;
   ```

2. **Database Tuning**:
   ```ini
   # /etc/postgresql/14/main/postgresql.conf
   shared_buffers = 256MB
   effective_cache_size = 1GB  
   work_mem = 4MB
   maintenance_work_mem = 64MB
   checkpoint_completion_target = 0.9
   wal_buffers = 16MB
   default_statistics_target = 100
   ```

3. **Connection Pooling**:
   ```yaml
   # config/production.yaml
   database:
     pool_size: 20
     max_overflow: 30
     pool_timeout: 30
     pool_recycle: 3600
   ```

#### Issue: External API Failures

**Symptoms**:
- Classification errors
- "External service unavailable" messages
- Degraded functionality

**Diagnostic Steps**:
```bash
# Test API connectivity
curl -v "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/einfo.fcgi?db=clinvar"
curl -v "https://gnomad.broadinstitute.org/api/"

# Check API rate limits
grep -i "rate.limit" /var/log/mcp-server/application.log

# Check DNS resolution
nslookup eutils.ncbi.nlm.nih.gov
nslookup gnomad.broadinstitute.org

# Check firewall/network
telnet eutils.ncbi.nlm.nih.gov 443
```

**Solutions**:

1. **Enable Circuit Breaker**:
   ```yaml
   # config/production.yaml
   external_apis:
     circuit_breaker:
       failure_threshold: 5
       timeout: 30s
       reset_timeout: 60s
   ```

2. **Implement Retry Logic**:
   ```yaml
   # config/production.yaml
   external_apis:
     retry:
       max_attempts: 3
       backoff: exponential
       base_delay: 1s
   ```

3. **Use Cached Data**:
   ```bash
   # Enable fallback to cached data
   redis-cli config set save "900 1 300 10 60 10000"
   ```

### Performance Issues

#### CPU Usage Optimization

**High CPU Usage Investigation**:
```bash
# Check CPU usage by process
top -p $(pgrep mcp-server)

# Profile application
perf record -p $(pgrep mcp-server) -g -- sleep 30
perf report

# Check system load
uptime
cat /proc/loadavg
```

**Solutions**:
```yaml
# config/production.yaml
performance:
  worker_processes: auto  # Match CPU cores
  worker_connections: 1024
  async_processing: true
  
optimization:
  enable_compression: true
  static_file_caching: true
  database_query_cache: true
```

#### Disk I/O Optimization

**Disk I/O Investigation**:
```bash
# Check disk usage
iostat -x 1 10
iotop -o

# Check database I/O
sudo -u postgres psql -d acmg_amp_mcp -c "SELECT * FROM pg_statio_user_tables;"
```

**Solutions**:
```bash
# Move database to SSD if on HDD
# Optimize database configuration
# Enable write-ahead logging optimization
```

---

## Performance Optimization

### Database Performance Tuning

#### PostgreSQL Optimization
```sql
-- Regular maintenance queries
VACUUM ANALYZE;
REINDEX DATABASE acmg_amp_mcp;

-- Optimize configuration based on system resources
ALTER SYSTEM SET shared_buffers = '25% of system RAM';
ALTER SYSTEM SET effective_cache_size = '75% of system RAM';
ALTER SYSTEM SET maintenance_work_mem = '1GB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;
SELECT pg_reload_conf();
```

#### Index Optimization
```sql
-- Identify missing indexes
SELECT schemaname, tablename, attname, n_distinct, correlation 
FROM pg_stats 
WHERE schemaname = 'public' 
  AND n_distinct > 100 
  AND correlation < 0.1;

-- Create performance indexes
CREATE INDEX CONCURRENTLY idx_variants_gene_hgvs ON variants(gene, hgvs);
CREATE INDEX CONCURRENTLY idx_evidence_created_at ON evidence(created_at) WHERE created_at > '2024-01-01';
CREATE INDEX CONCURRENTLY idx_classifications_confidence ON classifications(confidence) WHERE confidence > 0.8;
```

### Application Performance Tuning

#### Caching Strategy
```yaml
# config/production.yaml
caching:
  levels:
    - memory      # L1: In-process cache
    - redis       # L2: Distributed cache
    - database    # L3: Query result cache
  
  policies:
    variant_classifications:
      ttl: 86400    # 24 hours
      max_size: 10000
    
    evidence_data:
      ttl: 3600     # 1 hour
      max_size: 50000
    
    external_api_responses:
      ttl: 1800     # 30 minutes
      max_size: 100000
```

#### Connection Pool Tuning
```yaml
# config/production.yaml
database:
  pool_size: 20           # Base connections
  max_overflow: 30        # Additional connections
  pool_timeout: 30        # Connection wait timeout
  pool_recycle: 3600      # Connection lifetime
  pool_pre_ping: true     # Validate connections

redis:
  connection_pool:
    max_connections: 50
    retry_on_timeout: true
    socket_keepalive: true
    socket_keepalive_options:
      TCP_KEEPIDLE: 1
      TCP_KEEPINTVL: 3
      TCP_KEEPCNT: 5
```

### System-Level Optimization

#### Kernel Parameters
```bash
# /etc/sysctl.d/mcp-server.conf
# Network optimization
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# File descriptor limits
fs.file-max = 65536

# Virtual memory
vm.swappiness = 1
vm.dirty_ratio = 15
vm.dirty_background_ratio = 5
```

#### Service Limits
```ini
# /etc/systemd/system/mcp-server.service.d/limits.conf
[Service]
LimitNOFILE=65536
LimitNPROC=4096
LimitMEMLOCK=infinity
```

---

## Backup & Recovery

### Backup Strategy

#### Database Backups
```bash
#!/bin/bash
# Database backup script
# File: /opt/acmg-amp-mcp/scripts/backup-database.sh

BACKUP_DIR="/var/backups/mcp-server"
DATE=$(date +%Y%m%d_%H%M%S)
DB_NAME="acmg_amp_mcp"

# Create backup directory
mkdir -p "$BACKUP_DIR/database"

# Full database backup
sudo -u postgres pg_dump "$DB_NAME" | gzip > "$BACKUP_DIR/database/db_backup_$DATE.sql.gz"

# Schema-only backup
sudo -u postgres pg_dump --schema-only "$DB_NAME" > "$BACKUP_DIR/database/schema_backup_$DATE.sql"

# Configuration backup
tar -czf "$BACKUP_DIR/config_backup_$DATE.tar.gz" /opt/acmg-amp-mcp/config/

# Cleanup old backups (keep 30 days)
find "$BACKUP_DIR" -name "*.gz" -mtime +30 -delete
find "$BACKUP_DIR" -name "*.sql" -mtime +30 -delete

# Upload to cloud storage (if configured)
if command -v aws &> /dev/null; then
    aws s3 cp "$BACKUP_DIR/database/db_backup_$DATE.sql.gz" s3://your-backup-bucket/database/
fi

echo "Backup completed: $DATE"
```

#### System State Backup
```bash
#!/bin/bash
# System configuration backup
# File: /opt/acmg-amp-mcp/scripts/backup-system.sh

BACKUP_DIR="/var/backups/mcp-server/system"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

# System configuration
tar -czf "$BACKUP_DIR/system_config_$DATE.tar.gz" \
    /etc/systemd/system/mcp-server* \
    /etc/nginx/sites-available/mcp-server \
    /etc/postgresql/*/main/ \
    /etc/redis/ \
    --exclude="*.log"

# Application state
tar -czf "$BACKUP_DIR/app_state_$DATE.tar.gz" \
    /opt/acmg-amp-mcp/config/ \
    /opt/acmg-amp-mcp/data/ \
    /var/log/mcp-server/

# Package list
dpkg --get-selections > "$BACKUP_DIR/packages_$DATE.txt"

echo "System backup completed: $DATE"
```

### Recovery Procedures

#### Database Recovery
```bash
#!/bin/bash
# Database recovery script
# File: /opt/acmg-amp-mcp/scripts/recover-database.sh

BACKUP_FILE="$1"
DB_NAME="acmg_amp_mcp"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    exit 1
fi

echo "WARNING: This will overwrite the existing database!"
read -p "Continue? (y/N): " -n 1 -r
echo

if [[ $REPLY =~ ^[Yy]$ ]]; then
    # Stop MCP server
    systemctl stop mcp-server
    
    # Drop and recreate database
    sudo -u postgres dropdb "$DB_NAME"
    sudo -u postgres createdb "$DB_NAME"
    
    # Restore from backup
    if [[ $BACKUP_FILE == *.gz ]]; then
        zcat "$BACKUP_FILE" | sudo -u postgres psql "$DB_NAME"
    else
        sudo -u postgres psql "$DB_NAME" < "$BACKUP_FILE"
    fi
    
    # Restart services
    systemctl start mcp-server
    
    echo "Database recovery completed"
else
    echo "Recovery cancelled"
fi
```

#### Full System Recovery
```bash
#!/bin/bash
# Full system recovery procedure
# File: /opt/acmg-amp-mcp/scripts/disaster-recovery.sh

echo "=== ACMG/AMP MCP Server Disaster Recovery ==="

# 1. Restore system packages
if [ -f "packages.txt" ]; then
    sudo dpkg --set-selections < packages.txt
    sudo apt-get dselect-upgrade -y
fi

# 2. Restore configuration files
if [ -f "system_config.tar.gz" ]; then
    sudo tar -xzf system_config.tar.gz -C /
fi

# 3. Restore application
if [ -f "app_state.tar.gz" ]; then
    sudo tar -xzf app_state.tar.gz -C /
fi

# 4. Restore database (prompt for backup file)
read -p "Database backup file path: " DB_BACKUP
if [ -f "$DB_BACKUP" ]; then
    ./recover-database.sh "$DB_BACKUP"
fi

# 5. Restart all services
sudo systemctl daemon-reload
sudo systemctl restart postgresql redis nginx mcp-server

echo "Disaster recovery completed"
echo "Please verify system functionality"
```

---

## Updates & Patches

### Update Procedure

#### Application Updates
```bash
#!/bin/bash
# Application update script
# File: /opt/acmg-amp-mcp/scripts/update-application.sh

VERSION="$1"
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

echo "Updating ACMG/AMP MCP Server to version $VERSION"

# 1. Backup current installation
./backup-system.sh

# 2. Download new version
wget "https://releases.acmg-amp-mcp.org/v$VERSION/mcp-server-$VERSION.tar.gz"
tar -xzf "mcp-server-$VERSION.tar.gz"

# 3. Stop services
systemctl stop mcp-server

# 4. Install new version
cp -r "mcp-server-$VERSION"/* /opt/acmg-amp-mcp/
chown -R mcp-server:mcp-server /opt/acmg-amp-mcp/

# 5. Run database migrations
sudo -u mcp-server /opt/acmg-amp-mcp/bin/migrate --config /opt/acmg-amp-mcp/config/production.yaml

# 6. Update configuration if needed
if [ -f "mcp-server-$VERSION/config/migration-notes.txt" ]; then
    echo "Please review configuration changes:"
    cat "mcp-server-$VERSION/config/migration-notes.txt"
    read -p "Press enter to continue..."
fi

# 7. Start services
systemctl start mcp-server

# 8. Verify functionality
sleep 10
curl -f http://localhost:8080/health || echo "Health check failed!"

echo "Update completed"
```

#### System Updates
```bash
#!/bin/bash
# System maintenance updates
# File: /opt/acmg-amp-mcp/scripts/system-updates.sh

echo "Starting system updates..."

# Update package lists
apt update

# Security updates only (for production)
apt list --upgradable | grep -i security
apt upgrade -s | grep -i security

# Apply security updates
unattended-upgrade --dry-run
unattended-upgrade

# Update MCP server dependencies
pip install --upgrade -r /opt/acmg-amp-mcp/requirements.txt

# Restart services if needed
systemctl daemon-reload
if systemctl is-active --quiet mcp-server; then
    systemctl restart mcp-server
fi

echo "System updates completed"
```

### Rollback Procedures

#### Application Rollback
```bash
#!/bin/bash
# Rollback to previous version
# File: /opt/acmg-amp-mcp/scripts/rollback.sh

BACKUP_DATE="$1"
if [ -z "$BACKUP_DATE" ]; then
    echo "Available backups:"
    ls -la /var/backups/mcp-server/
    echo "Usage: $0 <backup_date>"
    exit 1
fi

echo "Rolling back to backup from $BACKUP_DATE"

# Stop services
systemctl stop mcp-server

# Restore application
tar -xzf "/var/backups/mcp-server/app_state_$BACKUP_DATE.tar.gz" -C /

# Restore database if needed
read -p "Restore database too? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./recover-database.sh "/var/backups/mcp-server/database/db_backup_$BACKUP_DATE.sql.gz"
fi

# Restart services
systemctl start mcp-server

echo "Rollback completed"
```

---

## Emergency Procedures

### Critical Service Outage

#### Immediate Response Checklist
```bash
# 1. Check service status
systemctl status mcp-server postgresql redis nginx

# 2. Check system resources
df -h
free -h
top -n 1

# 3. Check network connectivity
ping -c 4 8.8.8.8
curl -I http://localhost:8080/health

# 4. Check recent logs
journalctl -u mcp-server --since "1 hour ago" | tail -50
tail -50 /var/log/mcp-server/error.log

# 5. Attempt service restart
systemctl restart mcp-server

# 6. If still failing, escalate to on-call team
```

#### Failover to Backup System
```bash
#!/bin/bash
# Failover to backup system
# File: /opt/acmg-amp-mcp/scripts/failover.sh

BACKUP_SERVER="backup.acmg-amp-mcp.internal"

echo "Initiating failover to $BACKUP_SERVER"

# 1. Update load balancer to point to backup
curl -X POST http://load-balancer:8080/api/failover \
     -H "Content-Type: application/json" \
     -d '{"primary": "false", "backup": "true"}'

# 2. Update DNS (if using DNS-based failover)
nsupdate -k /etc/bind/keys/update.key <<EOF
server dns-server
zone acmg-amp-mcp.internal
update delete mcp-server.acmg-amp-mcp.internal A
update add mcp-server.acmg-amp-mcp.internal 300 A backup-server-ip
send
EOF

# 3. Notify operations team
echo "Failover completed at $(date)" | mail -s "CRITICAL: MCP Server Failover" ops-team@hospital.org

# 4. Update status page
curl -X POST https://status.acmg-amp-mcp.org/api/incident \
     -H "Authorization: Bearer $STATUS_API_KEY" \
     -d '{"title": "Primary server failover", "status": "investigating"}'
```

### Security Incident Response

#### Suspected Breach
```bash
#!/bin/bash
# Security incident response
# File: /opt/acmg-amp-mcp/scripts/security-incident.sh

echo "=== SECURITY INCIDENT RESPONSE ==="
echo "Incident time: $(date)"

# 1. Immediate containment
echo "1. Containing incident..."
iptables -A INPUT -j DROP  # Block all incoming traffic
systemctl stop mcp-server  # Stop application

# 2. Preserve evidence
echo "2. Preserving evidence..."
EVIDENCE_DIR="/var/security-evidence/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$EVIDENCE_DIR"

# Copy logs
cp -r /var/log/mcp-server/ "$EVIDENCE_DIR/logs/"
cp /var/log/auth.log "$EVIDENCE_DIR/"
cp /var/log/syslog "$EVIDENCE_DIR/"

# Network connections
netstat -tulpn > "$EVIDENCE_DIR/network-connections.txt"
ss -tulpn > "$EVIDENCE_DIR/socket-connections.txt"

# Process list
ps auxf > "$EVIDENCE_DIR/process-list.txt"

# File system changes
find /opt/acmg-amp-mcp/ -mtime -1 > "$EVIDENCE_DIR/recent-changes.txt"

# 3. Notify security team
echo "3. Notifying security team..."
echo "SECURITY INCIDENT DETECTED at $(date)" | mail -s "CRITICAL: Security Incident" security-team@hospital.org

# 4. Create incident ticket
curl -X POST https://ticketing-system/api/incident \
     -H "Content-Type: application/json" \
     -d '{
       "title": "MCP Server Security Incident",
       "priority": "critical",
       "category": "security",
       "description": "Automated security incident response triggered"
     }'

echo "Security incident response completed"
echo "Evidence stored in: $EVIDENCE_DIR"
```

### Data Corruption Recovery

#### Database Corruption
```bash
#!/bin/bash
# Database corruption recovery
# File: /opt/acmg-amp-mcp/scripts/db-corruption-recovery.sh

echo "=== DATABASE CORRUPTION RECOVERY ==="

# 1. Stop application
systemctl stop mcp-server

# 2. Check database integrity
sudo -u postgres pg_dump acmg_amp_mcp > /dev/null
if [ $? -ne 0 ]; then
    echo "Database corruption confirmed"
    
    # 3. Attempt repair
    sudo -u postgres reindexdb acmg_amp_mcp
    sudo -u postgres vacuumdb --full --analyze acmg_amp_mcp
    
    # 4. If repair fails, restore from backup
    if ! sudo -u postgres pg_dump acmg_amp_mcp > /dev/null; then
        echo "Repair failed, restoring from backup..."
        LATEST_BACKUP=$(ls -t /var/backups/mcp-server/database/db_backup_*.sql.gz | head -1)
        ./recover-database.sh "$LATEST_BACKUP"
    fi
fi

# 5. Restart application
systemctl start mcp-server

# 6. Verify functionality
sleep 10
curl -f http://localhost:8080/health
if [ $? -eq 0 ]; then
    echo "Recovery successful"
else
    echo "Recovery failed - manual intervention required"
fi
```

---

## Contact Information

### Emergency Contacts

| Role | Contact | Phone | Email |
|------|---------|-------|--------|
| **On-Call Engineer** | Primary | +1-XXX-XXX-XXXX | oncall@hospital.org |
| **Database Administrator** | Secondary | +1-XXX-XXX-XXXX | dba@hospital.org |
| **Security Team** | 24/7 SOC | +1-XXX-XXX-XXXX | security@hospital.org |
| **Network Operations** | NOC | +1-XXX-XXX-XXXX | noc@hospital.org |

### Escalation Procedures

1. **Level 1**: Operations Team (Response: 15 minutes)
2. **Level 2**: Engineering Manager (Response: 30 minutes)
3. **Level 3**: IT Director (Response: 1 hour)
4. **Level 4**: CIO/Executive Team (Response: 2 hours)

### Vendor Support

| Component | Vendor | Support Contact | SLA |
|-----------|---------|----------------|-----|
| **Cloud Infrastructure** | AWS/Azure/GCP | Enterprise Support | 4 hours |
| **Database** | PostgreSQL Community | Community Forums | Best effort |
| **Load Balancer** | F5/HAProxy | Vendor Support | 8 hours |
| **Monitoring** | Prometheus/Grafana | Community/Enterprise | Varies |

---

**Document Version**: 1.0  
**Last Updated**: January 15, 2024  
**Next Review**: April 15, 2024  
**Approved By**: IT Operations Manager

For additional support documentation and runbooks, refer to the [Operations Wiki] and [Incident Response Procedures].