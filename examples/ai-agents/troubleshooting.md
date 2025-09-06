# ACMG/AMP MCP Server - AI Agent Integration Troubleshooting Guide

This comprehensive troubleshooting guide helps resolve common issues when integrating AI agents with the ACMG/AMP MCP Server.

## Quick Diagnosis

### Symptom-Based Problem Identification

**Connection Issues** 🔌
- Server not starting
- Connection refused
- Timeout errors
- Transport failures

**Authentication/Authorization** 🔐
- API key errors
- Permission denied
- Unauthorized access
- Token expiration

**Data/Format Issues** 📝
- Invalid HGVS notation
- Malformed requests
- Response parsing errors
- Encoding problems

**Performance Issues** ⚡
- Slow response times
- Memory exhaustion
- Rate limiting
- Database timeouts

**Classification Issues** 🧬
- Unexpected results
- Missing evidence
- Criteria misapplication
- Confidence score anomalies

## Connection and Transport Issues

### Problem: MCP Server Won't Start

**Symptoms:**
```
Error: Cannot connect to MCP server
Process exit code: 1
Server process terminated unexpectedly
```

**Diagnostic Steps:**
```bash
# Check server binary
./bin/mcp-server --version
./bin/mcp-server --help

# Test configuration
./bin/mcp-server --config config/development.yaml --validate-config

# Check dependencies
ldd ./bin/mcp-server  # Linux
otool -L ./bin/mcp-server  # macOS
```

**Common Causes & Solutions:**

1. **Missing Dependencies**
   ```bash
   # Install required system libraries
   sudo apt-get install libpq-dev redis-tools  # Ubuntu/Debian
   brew install postgresql redis  # macOS
   ```

2. **Configuration File Issues**
   ```bash
   # Validate YAML syntax
   yamllint config/development.yaml
   
   # Check file permissions
   chmod 644 config/development.yaml
   ```

3. **Port Already in Use**
   ```bash
   # Check what's using the port
   netstat -tulpn | grep 8080
   lsof -i :8080
   
   # Kill conflicting process or change port
   kill -9 $(lsof -t -i:8080)
   ```

4. **Database Connection Issues**
   ```bash
   # Test PostgreSQL connection
   psql postgresql://mcpuser:mcppass@localhost:5432/acmg_amp_mcp
   
   # Test Redis connection
   redis-cli -h localhost -p 6379 ping
   ```

### Problem: Claude Desktop Can't Connect

**Symptoms:**
```json
{
  "error": "Failed to start MCP server",
  "details": "Command not found or permission denied"
}
```

**Diagnostic Steps:**

1. **Check Claude Desktop Configuration**
   ```bash
   # macOS
   cat ~/Library/Application\ Support/Claude/claude_desktop_config.json
   
   # Windows
   type %APPDATA%\Claude\claude_desktop_config.json
   ```

2. **Verify File Paths**
   ```bash
   # Test the exact command from config
   /full/path/to/mcp-server --config /full/path/to/config.yaml
   ```

3. **Check Permissions**
   ```bash
   chmod +x /path/to/mcp-server
   ls -la /path/to/mcp-server
   ```

**Solutions:**

1. **Use Absolute Paths**
   ```json
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "/Users/username/acmg-amp-classifier-mcp/bin/mcp-server",
         "args": ["--config", "/Users/username/acmg-amp-classifier-mcp/config/development.yaml"]
       }
     }
   }
   ```

2. **Environment Variable Issues**
   ```json
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "/path/to/mcp-server",
         "env": {
           "DATABASE_URL": "postgresql://mcpuser:mcppass@localhost:5432/acmg_amp_mcp",
           "PATH": "/usr/local/bin:/usr/bin:/bin"
         }
       }
     }
   }
   ```

### Problem: Docker Transport Issues

**Symptoms:**
```
Error: Cannot connect to Docker daemon
Container exits immediately
Network connectivity issues
```

**Diagnostic Steps:**
```bash
# Test Docker functionality
docker --version
docker info
docker ps

# Test container directly
docker run --rm -it mcp-acmg-amp-server:latest /bin/bash

# Check logs
docker logs container_name
```

**Solutions:**

1. **Docker Daemon Issues**
   ```bash
   # Start Docker daemon
   sudo systemctl start docker  # Linux
   open -a Docker  # macOS
   ```

2. **Network Configuration**
   ```json
   {
     "command": "docker",
     "args": [
       "run", "--rm", "-i",
       "--network", "host",
       "--env-file", "/path/to/.env",
       "mcp-acmg-amp-server:latest"
     ]
   }
   ```

3. **Volume Mounting Issues**
   ```bash
   # Fix permission issues
   chmod 755 /path/to/config
   chown -R $USER:$GROUP /path/to/config
   ```

## API and Authentication Issues

### Problem: External API Key Errors

**Symptoms:**
```json
{
  "error": "Authentication failed",
  "message": "Invalid API key for ClinVar",
  "tool": "query_clinvar"
}
```

**Diagnostic Steps:**
```bash
# Test API keys manually
curl -H "Authorization: Bearer $CLINVAR_API_KEY" \
     https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi

# Check environment variables
env | grep -E "(CLINVAR|GNOMAD|COSMIC)_API_KEY"

# Validate .env file
cat .env | grep -E "(CLINVAR|GNOMAD|COSMIC)"
```

**Solutions:**

1. **API Key Validation**
   ```bash
   # Test each API key
   curl -f "https://api.clinvar.com/test?key=$CLINVAR_API_KEY"
   curl -f "https://gnomad.broadinstitute.org/api/test?key=$GNOMAD_API_KEY"
   ```

2. **Environment Variable Loading**
   ```bash
   # Source .env file properly
   export $(grep -v '^#' .env | xargs)
   
   # Or use dotenv in scripts
   python3 -c "from dotenv import load_dotenv; load_dotenv(); import os; print(os.getenv('CLINVAR_API_KEY'))"
   ```

3. **Key Rotation**
   ```bash
   # Generate new API keys and update configuration
   # Update .env file
   # Restart MCP server
   ```

### Problem: Rate Limiting Issues

**Symptoms:**
```json
{
  "error": "Rate limit exceeded",
  "retry_after": 60,
  "api": "gnomad"
}
```

**Solutions:**

1. **Implement Exponential Backoff**
   ```python
   import asyncio
   import random
   
   async def retry_with_backoff(func, max_retries=3):
       for attempt in range(max_retries):
           try:
               return await func()
           except RateLimitError as e:
               if attempt == max_retries - 1:
                   raise
               delay = (2 ** attempt) + random.uniform(0, 1)
               await asyncio.sleep(delay)
   ```

2. **Configure Rate Limits**
   ```yaml
   # config/development.yaml
   external_apis:
     rate_limits:
       clinvar: 10  # requests per second
       gnomad: 5
       cosmic: 3
     batch_settings:
       max_concurrent: 3
       delay_between_batches: 1.0
   ```

3. **Use Caching**
   ```yaml
   caching:
     enabled: true
     ttl: 3600  # 1 hour
     redis_url: "redis://localhost:6379"
   ```

## Data Format and Validation Issues

### Problem: Invalid HGVS Notation

**Symptoms:**
```json
{
  "error": "Invalid HGVS notation",
  "input": "c.1521_1523delCTT",
  "message": "Missing transcript identifier"
}
```

**Diagnostic Steps:**
```bash
# Test HGVS validation
curl -X POST http://localhost:8080/tools/validate_hgvs \
  -H "Content-Type: application/json" \
  -d '{"hgvs": "NM_000492.3:c.1521_1523delCTT"}'
```

**Solutions:**

1. **Standard HGVS Format**
   ```
   Correct: NM_000492.3:c.1521_1523delCTT
   Incorrect: c.1521_1523delCTT
   
   Correct: NC_000007.14:g.117199644_117199646del
   Incorrect: chr7:117199644_117199646del
   ```

2. **Transcript Version Issues**
   ```python
   # Use current transcript versions
   valid_transcripts = {
       "CFTR": "NM_000492.4",  # Updated version
       "BRCA1": "NM_007294.4",
       "BRCA2": "NM_000059.4"
   }
   ```

3. **Batch Validation**
   ```python
   # Validate all variants before processing
   invalid_variants = []
   for variant in variant_list:
       validation = await client.validate_hgvs(variant['hgvs'])
       if not validation['valid']:
           invalid_variants.append(variant)
   ```

### Problem: Character Encoding Issues

**Symptoms:**
```
UnicodeDecodeError: 'utf-8' codec can't decode byte
Invalid JSON: unexpected character
```

**Solutions:**

1. **Ensure UTF-8 Encoding**
   ```python
   # Python
   with open('variants.json', 'r', encoding='utf-8') as f:
       data = json.load(f)
   ```

2. **Handle Special Characters**
   ```bash
   # Convert file encoding
   iconv -f ISO-8859-1 -t UTF-8 input.csv > output.csv
   ```

3. **Clean Input Data**
   ```python
   # Remove problematic characters
   import unicodedata
   
   def clean_text(text):
       # Normalize unicode
       text = unicodedata.normalize('NFKD', text)
       # Remove non-printable characters
       text = ''.join(char for char in text if ord(char) >= 32)
       return text
   ```

## Performance and Resource Issues

### Problem: Slow Response Times

**Symptoms:**
- Classification takes > 30 seconds
- Timeouts on batch processing
- High CPU/memory usage

**Diagnostic Steps:**
```bash
# Monitor resource usage
top -p $(pgrep mcp-server)
htop

# Check database performance
psql -c "SELECT * FROM pg_stat_activity;"

# Monitor network
netstat -i
iftop
```

**Solutions:**

1. **Database Optimization**
   ```sql
   -- Create indexes for common queries
   CREATE INDEX idx_variants_hgvs ON variants(hgvs);
   CREATE INDEX idx_evidence_variant_id ON evidence(variant_id);
   
   -- Analyze query performance
   EXPLAIN ANALYZE SELECT * FROM variants WHERE hgvs = 'NM_000492.3:c.1521_1523delCTT';
   ```

2. **Caching Strategy**
   ```yaml
   caching:
     levels:
       - memory  # L1 cache
       - redis   # L2 cache
     policies:
       evidence_ttl: 3600    # 1 hour
       classification_ttl: 7200  # 2 hours
   ```

3. **Connection Pooling**
   ```yaml
   database:
     pool_size: 20
     max_overflow: 30
     pool_timeout: 30
     pool_recycle: 3600
   ```

4. **Async Processing**
   ```python
   # Process variants concurrently
   import asyncio
   
   async def process_batch(variants):
       tasks = [classify_variant(v) for v in variants]
       results = await asyncio.gather(*tasks, return_exceptions=True)
       return results
   ```

### Problem: Memory Issues

**Symptoms:**
```
Out of memory error
Process killed by OOM killer
Swap usage high
```

**Solutions:**

1. **Memory Profiling**
   ```python
   import tracemalloc
   import psutil
   
   # Start memory tracing
   tracemalloc.start()
   
   # Monitor memory usage
   process = psutil.Process()
   print(f"Memory usage: {process.memory_info().rss / 1024 / 1024:.2f} MB")
   ```

2. **Batch Size Optimization**
   ```python
   # Process in smaller chunks
   def chunk_list(lst, chunk_size):
       for i in range(0, len(lst), chunk_size):
           yield lst[i:i + chunk_size]
   
   for chunk in chunk_list(variants, 10):
       results = await process_chunk(chunk)
   ```

3. **Memory Cleanup**
   ```python
   import gc
   
   # Force garbage collection
   gc.collect()
   
   # Clear caches periodically
   if len(cache) > 10000:
       cache.clear()
   ```

## Classification and Scientific Issues

### Problem: Unexpected Classification Results

**Symptoms:**
- Known pathogenic variant classified as benign
- High-frequency variant classified as pathogenic
- Conflicting results between runs

**Diagnostic Steps:**
```bash
# Check classification criteria
curl -X POST http://localhost:8080/tools/classify_variant \
  -d '{"variant_data": {...}, "options": {"debug": true}}'

# Compare with known databases
curl "https://www.ncbi.nlm.nih.gov/clinvar/variation/12345/"
```

**Solutions:**

1. **Evidence Review**
   ```python
   # Get detailed evidence breakdown
   evidence = await client.query_evidence(variant, databases=["all"])
   classification = await client.classify_variant(
       variant, 
       options={"include_evidence": True, "debug": True}
   )
   
   # Review each applied criterion
   for criterion in classification['applied_criteria']:
       print(f"{criterion}: {classification['evidence_summary'][criterion]}")
   ```

2. **Population Frequency Issues**
   ```python
   # Check population-specific frequencies
   gnomad_data = evidence['gnomad']
   
   # Consider patient ancestry
   relevant_population = get_patient_ancestry(patient_data)
   frequency = gnomad_data[f'{relevant_population}_frequency']
   ```

3. **Database Version Consistency**
   ```yaml
   # Pin database versions
   external_apis:
     clinvar:
       version: "2024-01"
       endpoint: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/"
     gnomad:
       version: "v4.0.0"
       endpoint: "https://gnomad.broadinstitute.org/api/"
   ```

### Problem: Missing Evidence

**Symptoms:**
```json
{
  "evidence": {
    "clinvar": null,
    "gnomad": null,
    "functional_studies": []
  }
}
```

**Solutions:**

1. **Alternative Identifiers**
   ```python
   # Try different variant representations
   identifiers = [
       "NM_000492.3:c.1521_1523delCTT",
       "NM_000492.4:c.1521_1523del",
       "rs113993960",
       "p.Phe508del"
   ]
   
   for identifier in identifiers:
       evidence = await client.query_evidence(identifier)
       if evidence['sources']:
           break
   ```

2. **Transcript Mapping**
   ```python
   # Map between transcript versions
   transcript_map = {
       "NM_000492.3": "NM_000492.4",
       "NM_007294.3": "NM_007294.4"
   }
   ```

3. **Synonymous Search**
   ```python
   # Search for synonymous representations
   async def find_evidence_comprehensive(variant):
       # Try exact match first
       evidence = await query_evidence(variant)
       if not evidence['sources']:
           # Try genomic coordinates
           genomic_variant = convert_to_genomic(variant)
           evidence = await query_evidence(genomic_variant)
       return evidence
   ```

## Integration-Specific Issues

### Problem: Claude Desktop Integration Issues

**Common Issues:**

1. **Configuration File Location**
   ```bash
   # Correct paths for different OS
   # macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
   # Windows: %APPDATA%\Claude\claude_desktop_config.json
   # Linux: ~/.config/claude/claude_desktop_config.json
   ```

2. **JSON Syntax Errors**
   ```json
   // ❌ Incorrect (trailing comma)
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "/path/to/server",
       }
     }
   }
   
   // ✅ Correct
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "/path/to/server"
       }
     }
   }
   ```

3. **Environment Variables**
   ```json
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "/path/to/server",
         "env": {
           "HOME": "/Users/username",
           "PATH": "/usr/local/bin:/usr/bin:/bin",
           "DATABASE_URL": "postgresql://..."
         }
       }
     }
   }
   ```

### Problem: Custom Client Development Issues

**Common Patterns:**

1. **Message ID Management**
   ```python
   class MCPClient:
       def __init__(self):
           self.message_id = 0
           self.pending_requests = {}
       
       async def send_message(self, method, params):
           msg_id = self.message_id + 1
           self.message_id = msg_id
           # ... rest of implementation
   ```

2. **Protocol Compliance**
   ```python
   # Ensure JSON-RPC 2.0 compliance
   message = {
       "jsonrpc": "2.0",  # Required
       "id": msg_id,      # Required for requests
       "method": method,  # Required for requests
       "params": params   # Optional
   }
   ```

3. **Error Handling**
   ```python
   if 'error' in response:
       error = response['error']
       if error['code'] == -32602:  # Invalid params
           raise InvalidParametersError(error['message'])
       elif error['code'] == -32603:  # Internal error
           raise InternalServerError(error['message'])
   ```

## Debugging Tools and Techniques

### Debug Mode Activation

```bash
# Enable debug logging
export LOG_LEVEL=DEBUG
./bin/mcp-server --config config/development.yaml

# Or in configuration
```

```yaml
# config/development.yaml
logging:
  level: DEBUG
  enable_request_logging: true
  enable_response_logging: true
```

### Message Tracing

```python
# Custom client with message tracing
class DebugMCPClient(MCPClient):
    async def send_message(self, method, params):
        print(f"SEND: {method} -> {json.dumps(params, indent=2)}")
        result = await super().send_message(method, params)
        print(f"RECV: {json.dumps(result, indent=2)}")
        return result
```

### Health Check Endpoint

```bash
# Test server health
curl http://localhost:8080/health

# Expected response
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "database": "connected",
  "redis": "connected",
  "external_apis": {
    "clinvar": "healthy",
    "gnomad": "healthy"
  }
}
```

### Performance Profiling

```bash
# Profile server performance
go tool pprof http://localhost:8080/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:8080/debug/pprof/heap

# CPU profiling
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

## Getting Help

### Before Seeking Help

1. **Check Logs**
   ```bash
   # Server logs
   tail -f logs/mcp-server.log
   
   # System logs
   journalctl -u mcp-server -f  # systemd
   tail -f /var/log/system.log  # macOS
   ```

2. **Reproduce with Minimal Example**
   ```python
   # Minimal reproduction case
   client = MCPClient("./bin/mcp-server")
   await client.connect()
   result = await client.call_tool("validate_hgvs", {"hgvs": "problem_variant"})
   ```

3. **Collect Environment Information**
   ```bash
   ./scripts/debug/collect-env-info.sh > debug_info.txt
   ```

### Support Channels

1. **GitHub Issues**: Bug reports and feature requests
2. **Documentation**: Comprehensive guides and API reference
3. **Community Forum**: User discussions and tips
4. **Professional Support**: For enterprise deployments

### Issue Report Template

```markdown
## Problem Description
Brief description of the issue

## Environment
- OS: 
- MCP Server Version:
- AI Agent: 
- Database: PostgreSQL version / Redis version

## Steps to Reproduce
1. 
2. 
3. 

## Expected Behavior
What should happen

## Actual Behavior  
What actually happens

## Logs/Error Messages
```
error log here
```

## Additional Context
Any other relevant information
```

## Prevention Best Practices

1. **Regular Updates**: Keep MCP server and dependencies updated
2. **Configuration Management**: Use version control for configuration files
3. **Monitoring**: Implement health checks and alerting
4. **Testing**: Regular integration tests with known variants
5. **Documentation**: Maintain internal documentation of customizations
6. **Backup**: Regular backups of configuration and data
7. **Security**: Regular security updates and access reviews