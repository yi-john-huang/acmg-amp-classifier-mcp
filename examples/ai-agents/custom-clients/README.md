# Custom MCP Clients for ACMG/AMP Server

This directory contains custom MCP client implementations for testing and development with the ACMG/AMP classification server.

## Available Clients

### Python Client (`python-client.py`)

A comprehensive Python implementation demonstrating:
- MCP protocol communication via stdio transport
- High-level ACMG/AMP specific operations
- Interactive testing session
- Complete workflow examples

**Features:**
- Asynchronous I/O for optimal performance
- Type hints and dataclass structures
- Comprehensive error handling
- JSON result output for further processing

**Usage:**
```bash
# Run example workflow
./python-client.py

# Interactive session
./python-client.py interactive

# Or using Python directly
python3 python-client.py
python3 python-client.py interactive
```

**Requirements:**
```bash
# Python 3.7+ with asyncio support
pip install asyncio-subprocess  # If needed
```

### JavaScript Client (`javascript-client.js`)

A Node.js implementation providing:
- Event-driven MCP communication
- Promise-based API interface
- Interactive command-line interface
- Comprehensive examples

**Features:**
- ES2017+ async/await syntax
- Event emitter pattern for extensibility
- Readline interface for interactive use
- JSON output compatibility

**Usage:**
```bash
# Run example workflow
./javascript-client.js

# Interactive session
./javascript-client.js interactive

# Or using Node directly
node javascript-client.js
node javascript-client.js interactive
```

**Requirements:**
```bash
# Node.js 12+ required
npm install  # If you have a package.json with dependencies
```

## Client Architecture

### MCP Protocol Layer

Both clients implement the core MCP protocol:

```
┌─────────────────┐    JSON-RPC 2.0    ┌─────────────────┐
│   MCP Client    │ ←──────────────────→ │   MCP Server    │
│                 │     over stdio      │                 │
│ ┌─────────────┐ │                     │ ┌─────────────┐ │
│ │   Tools     │ │                     │ │ ACMG/AMP    │ │
│ │ Resources   │ │                     │ │ Tools &     │ │
│ │  Prompts    │ │                     │ │ Resources   │ │
│ └─────────────┘ │                     │ └─────────────┘ │
└─────────────────┘                     └─────────────────┘
```

### High-Level API

Both implementations provide an `ACMGAMPClient` wrapper that simplifies common operations:

- `classifyVariant()` - Complete variant classification workflow
- `validateHgvs()` - HGVS notation validation and normalization
- `queryEvidence()` - Multi-database evidence gathering
- `generateReport()` - Clinical report generation
- `getVariantInfo()` - Resource-based variant information access
- `getAcmgRules()` - ACMG/AMP criteria reference

## Example Workflows

### Basic Variant Classification

```python
# Python example
client = MCPClient("./bin/mcp-server", ["--config", "config/development.yaml"])
acmg_client = ACMGAMPClient(client)

await client.connect()

# Classify variant
result = await acmg_client.classify_variant({
    "hgvs": "NM_000492.3:c.1521_1523delCTT",
    "gene": "CFTR"
})

print(f"Classification: {result['classification']}")
print(f"Confidence: {result['confidence']}")
```

```javascript
// JavaScript example
const client = new MCPClient('./bin/mcp-server', ['--config', 'config/development.yaml']);
const acmgClient = new ACMGAMPClient(client);

await client.connect();

// Classify variant
const result = await acmgClient.classifyVariant({
    hgvs: 'NM_000492.3:c.1521_1523delCTT',
    gene: 'CFTR'
});

console.log(`Classification: ${result.classification}`);
console.log(`Confidence: ${result.confidence}`);
```

### Complete Clinical Workflow

Both clients include comprehensive workflows that:

1. **Initialize** MCP connection and list available capabilities
2. **Validate** HGVS notation for the input variant
3. **Gather evidence** from multiple external databases
4. **Classify** the variant using ACMG/AMP guidelines
5. **Generate** a clinical report in the requested format
6. **Save results** to JSON file for further analysis

### Interactive Testing

Both clients provide interactive modes for:
- Testing individual MCP tools
- Exploring available resources
- Debugging classification workflows
- Learning the MCP protocol

## Extending the Clients

### Adding New Tools

To support new MCP tools, add methods to the `ACMGAMPClient` class:

```python
# Python
async def my_custom_tool(self, parameters):
    return await self.mcp.call_tool("my_custom_tool", parameters)
```

```javascript
// JavaScript
async myCustomTool(parameters) {
    return await this.mcp.callTool('my_custom_tool', parameters);
}
```

### Custom Error Handling

Both clients can be extended with custom error handling:

```python
# Python
class CustomACMGClient(ACMGAMPClient):
    async def classify_variant(self, variant_data, options=None):
        try:
            return await super().classify_variant(variant_data, options)
        except Exception as e:
            # Custom error handling logic
            logger.error(f"Classification failed: {e}")
            return {"error": str(e), "classification": "Unknown"}
```

### Protocol Extensions

For advanced use cases, both clients can be extended to support:
- WebSocket transport (for remote servers)
- HTTP transport with Server-Sent Events
- Custom authentication mechanisms
- Message compression and optimization
- Connection pooling and load balancing

## Development and Testing

### Running Tests

```bash
# Test Python client
python3 -m pytest test_python_client.py

# Test JavaScript client  
npm test
```

### Debugging

Enable debug logging by setting environment variables:

```bash
# Python
export PYTHONPATH=.
export LOG_LEVEL=DEBUG
./python-client.py

# JavaScript
export NODE_DEBUG=mcp
./javascript-client.js
```

### Performance Testing

Both clients support performance testing:

```python
# Python - measure tool execution time
import time
start = time.time()
result = await acmg_client.classify_variant(variant_data)
duration = time.time() - start
print(f"Classification took {duration:.2f} seconds")
```

## Integration Examples

### CI/CD Pipeline

```yaml
# .github/workflows/variant-testing.yml
name: Variant Classification Tests
on: [push, pull_request]

jobs:
  test-classification:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Start MCP Server
        run: ./bin/mcp-server --config config/ci.yaml &
      - name: Run Python Client Tests
        run: python3 examples/ai-agents/custom-clients/python-client.py
      - name: Run JavaScript Client Tests
        run: node examples/ai-agents/custom-clients/javascript-client.js
```

### Docker Integration

```dockerfile
FROM python:3.9-slim
COPY examples/ai-agents/custom-clients/python-client.py /app/
COPY bin/mcp-server /app/
RUN chmod +x /app/python-client.py /app/mcp-server
CMD ["python3", "/app/python-client.py"]
```

## Next Steps

- Review `../workflows/` for clinical workflow examples
- Check `../troubleshooting.md` for common issues
- See the main README for server configuration options
- Explore the MCP protocol specification for advanced features