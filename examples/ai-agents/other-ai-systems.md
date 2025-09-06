# Integration Guide for Other MCP-Compatible AI Systems

This guide provides integration instructions for various AI systems and platforms that support the Model Context Protocol (MCP), enabling them to work with the ACMG/AMP classification server.

## Supported AI Systems Overview

### Current MCP Support Status

**Tier 1: Native MCP Support** ✅
- Claude (Anthropic) - Full native support
- Custom implementations using MCP SDKs

**Tier 2: Bridge/Proxy Support** 🔄
- ChatGPT/GPT-4 (OpenAI) - Via HTTP bridge
- Gemini (Google) - Via function calling bridge
- Llama-based systems - Via custom adapters

**Tier 3: Development/Experimental** 🧪
- Microsoft Copilot - Early development
- Bard/Gemini Advanced - Function calling integration
- Open source alternatives - Community implementations

## Native MCP Integrations

### Claude Desktop (Primary Reference)

See dedicated [Claude Desktop configuration examples](./claude-desktop-config.json) for complete setup.

**Key Features**:
- Direct stdio transport support
- Real-time tool invocation
- Context-aware conversations
- Resource streaming

### Custom MCP Clients

For building your own MCP-compatible clients:

#### Python Client Integration
```python
from mcp_client import MCPClient

# Initialize client
client = MCPClient(
    server_command="./bin/mcp-server",
    server_args=["--config", "config/production.yaml"]
)

# Connect and use
await client.connect()
tools = await client.list_tools()
result = await client.call_tool("classify_variant", {
    "variant_data": {"hgvs": "NM_000492.3:c.1521_1523delCTT"}
})
```

#### JavaScript/TypeScript Integration
```typescript
import { MCPClient } from '@modelcontextprotocol/sdk';

const client = new MCPClient({
    transport: {
        type: 'stdio',
        command: './bin/mcp-server',
        args: ['--config', 'config/production.yaml']
    }
});

await client.initialize();
const classification = await client.callTool('classify_variant', {
    variant_data: { hgvs: 'NM_000492.3:c.1521_1523delCTT' }
});
```

## Bridge Integrations

### OpenAI GPT Integration

#### Method 1: HTTP Bridge + Function Calling

**Step 1: Start HTTP Bridge**
```bash
# Start MCP-to-HTTP bridge
node scripts/mcp-http-bridge.js \
    --mcp-server ./bin/mcp-server \
    --port 8080 \
    --cors-origin "*"
```

**Step 2: Configure OpenAI Functions**
```python
import openai
import requests

# Define function schema matching MCP tools
classify_variant_schema = {
    "name": "classify_variant",
    "description": "Classify genetic variant using ACMG/AMP guidelines",
    "parameters": {
        "type": "object",
        "properties": {
            "hgvs": {
                "type": "string",
                "description": "HGVS notation of the variant"
            },
            "gene": {
                "type": "string", 
                "description": "Gene symbol"
            },
            "clinical_context": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Clinical conditions or phenotypes"
            }
        },
        "required": ["hgvs"]
    }
}

# Function handler
def handle_classify_variant(**kwargs):
    response = requests.post(
        "http://localhost:8080/tools/classify_variant",
        json={"variant_data": kwargs}
    )
    return response.json()

# OpenAI chat completion with functions
client = openai.OpenAI(api_key="your-api-key")

response = client.chat.completions.create(
    model="gpt-4-turbo",
    messages=[{
        "role": "user", 
        "content": "Please classify the CFTR variant NM_000492.3:c.1521_1523delCTT"
    }],
    functions=[classify_variant_schema],
    function_call="auto"
)

# Handle function call
if response.choices[0].message.function_call:
    function_name = response.choices[0].message.function_call.name
    arguments = json.loads(response.choices[0].message.function_call.arguments)
    result = handle_classify_variant(**arguments)
```

#### Method 2: Assistant API Integration

```python
# Create OpenAI Assistant with ACMG/AMP tools
assistant = client.beta.assistants.create(
    name="ACMG/AMP Classifier",
    instructions="""
    You are a clinical geneticist assistant specializing in variant interpretation.
    Use the available tools to classify genetic variants according to ACMG/AMP guidelines.
    Always gather evidence from multiple sources and provide comprehensive clinical recommendations.
    """,
    model="gpt-4-turbo",
    tools=[
        {
            "type": "function",
            "function": classify_variant_schema
        },
        {
            "type": "function", 
            "function": query_evidence_schema
        }
    ]
)

# Handle conversation with tool calls
def run_conversation(user_message):
    # Create thread and run
    thread = client.beta.threads.create()
    
    client.beta.threads.messages.create(
        thread_id=thread.id,
        role="user",
        content=user_message
    )
    
    run = client.beta.threads.runs.create(
        thread_id=thread.id,
        assistant_id=assistant.id
    )
    
    # Handle tool calls
    while run.status == "requires_action":
        tool_calls = run.required_action.submit_tool_outputs.tool_calls
        tool_outputs = []
        
        for tool_call in tool_calls:
            if tool_call.function.name == "classify_variant":
                result = handle_classify_variant(
                    **json.loads(tool_call.function.arguments)
                )
                tool_outputs.append({
                    "tool_call_id": tool_call.id,
                    "output": json.dumps(result)
                })
        
        run = client.beta.threads.runs.submit_tool_outputs(
            thread_id=thread.id,
            run_id=run.id,
            tool_outputs=tool_outputs
        )
    
    return run
```

### Google Gemini Integration

#### Function Calling Setup

```python
import google.generativeai as genai

# Configure Gemini
genai.configure(api_key="your-api-key")

# Define function schema
classify_variant_function = genai.protos.FunctionDeclaration(
    name="classify_variant",
    description="Classify genetic variant using ACMG/AMP guidelines",
    parameters=genai.protos.Schema(
        type=genai.protos.Type.OBJECT,
        properties={
            "hgvs": genai.protos.Schema(
                type=genai.protos.Type.STRING,
                description="HGVS notation of variant"
            ),
            "gene": genai.protos.Schema(
                type=genai.protos.Type.STRING,
                description="Gene symbol"
            )
        },
        required=["hgvs"]
    )
)

# Create tool
classify_tool = genai.protos.Tool(
    function_declarations=[classify_variant_function]
)

# Initialize model with tools
model = genai.GenerativeModel(
    model_name="gemini-1.5-pro",
    tools=[classify_tool]
)

# Function handler
def execute_function_call(function_call):
    if function_call.name == "classify_variant":
        args = {key: val for key, val in function_call.args.items()}
        response = requests.post(
            "http://localhost:8080/tools/classify_variant",
            json={"variant_data": args}
        )
        return response.json()

# Chat with function calling
chat = model.start_chat()
response = chat.send_message(
    "Please classify the variant NM_000492.3:c.1521_1523delCTT in CFTR gene"
)

# Handle function calls
if response.candidates[0].content.parts[0].function_call:
    function_call = response.candidates[0].content.parts[0].function_call
    result = execute_function_call(function_call)
    
    # Send function result back
    response = chat.send_message(
        genai.protos.Content(
            parts=[genai.protos.Part(
                function_response=genai.protos.FunctionResponse(
                    name=function_call.name,
                    response={"result": result}
                )
            )]
        )
    )
```

### Microsoft Copilot Integration

#### Plugin Development (Experimental)

```typescript
// copilot-plugin.ts
import { CopilotPlugin, PluginContext } from '@microsoft/copilot-sdk';

export class ACMGAMPPlugin implements CopilotPlugin {
    name = "acmg-amp-classifier";
    description = "Genetic variant classification using ACMG/AMP guidelines";
    
    private mcpClient: MCPClient;
    
    constructor() {
        this.mcpClient = new MCPClient({
            serverCommand: "./bin/mcp-server",
            serverArgs: ["--config", "config/copilot.yaml"]
        });
    }
    
    async initialize(): Promise<void> {
        await this.mcpClient.connect();
    }
    
    async execute(context: PluginContext): Promise<string> {
        const { message, parameters } = context;
        
        // Extract variant from user message
        const variant = this.extractVariant(message);
        
        if (variant) {
            const classification = await this.mcpClient.callTool(
                "classify_variant",
                { variant_data: { hgvs: variant } }
            );
            
            return this.formatClassificationResponse(classification);
        }
        
        return "Please provide a genetic variant in HGVS notation for classification.";
    }
    
    private extractVariant(message: string): string | null {
        // Regex to extract HGVS notation
        const hgvsPattern = /N[MR]_\d+\.\d+:[cgmnpr]\.[^\s]+/g;
        const matches = message.match(hgvsPattern);
        return matches ? matches[0] : null;
    }
    
    private formatClassificationResponse(result: any): string {
        return `
**Variant Classification**: ${result.classification}
**Confidence**: ${(result.confidence * 100).toFixed(1)}%
**Applied Criteria**: ${result.applied_criteria.join(', ')}
**Clinical Significance**: ${result.clinical_significance}
        `.trim();
    }
}
```

## Open Source AI Systems

### Ollama Integration

```python
# ollama-mcp-bridge.py
import ollama
import json
import requests
from typing import Dict, Any

class OllamaMCPBridge:
    def __init__(self, model: str = "llama3.1:8b"):
        self.client = ollama.Client()
        self.model = model
        self.mcp_endpoint = "http://localhost:8080"
    
    def classify_variant_tool(self, hgvs: str, gene: str = None) -> Dict[str, Any]:
        """Tool function for variant classification"""
        response = requests.post(
            f"{self.mcp_endpoint}/tools/classify_variant",
            json={
                "variant_data": {"hgvs": hgvs, "gene": gene}
            }
        )
        return response.json()
    
    def query_evidence_tool(self, variant: str, databases: list = None) -> Dict[str, Any]:
        """Tool function for evidence querying"""
        response = requests.post(
            f"{self.mcp_endpoint}/tools/query_evidence",
            json={
                "variant": variant,
                "databases": databases or ["all"]
            }
        )
        return response.json()
    
    def chat_with_tools(self, user_message: str) -> str:
        # System prompt with tool descriptions
        system_prompt = """
        You are a clinical geneticist assistant. You have access to genetic variant 
        classification tools. When users ask about genetic variants:
        
        1. Use classify_variant_tool(hgvs, gene) to classify variants
        2. Use query_evidence_tool(variant, databases) to gather evidence
        3. Provide comprehensive clinical interpretations
        
        Always explain your reasoning and cite evidence sources.
        """
        
        # Simple tool detection (in practice, use more sophisticated NLP)
        if "classify" in user_message.lower() and ("NM_" in user_message or "NR_" in user_message):
            # Extract variant information
            import re
            hgvs_match = re.search(r'N[MR]_\d+\.\d+:[cgmnpr]\.[^\s]+', user_message)
            if hgvs_match:
                hgvs = hgvs_match.group()
                result = self.classify_variant_tool(hgvs)
                
                # Format response
                response = f"""
I've classified the variant {hgvs}:

**Classification**: {result.get('classification', 'Unknown')}
**Confidence**: {result.get('confidence', 0) * 100:.1f}%
**Applied ACMG/AMP Criteria**: {', '.join(result.get('applied_criteria', []))}

**Clinical Interpretation**:
{result.get('clinical_significance', 'See detailed report for clinical significance.')}
                """.strip()
                
                return response
        
        # Default Ollama response for non-tool queries
        response = self.client.chat(
            model=self.model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_message}
            ]
        )
        
        return response['message']['content']

# Usage example
bridge = OllamaMCPBridge()
response = bridge.chat_with_tools(
    "Please classify the variant NM_000492.3:c.1521_1523delCTT"
)
print(response)
```

### LangChain Integration

```python
from langchain.tools import BaseTool
from langchain.agents import initialize_agent, AgentType
from langchain.llms import OpenAI
from pydantic import BaseModel, Field
import requests

class VariantClassificationTool(BaseTool):
    name = "classify_variant"
    description = """
    Classify genetic variants using ACMG/AMP guidelines.
    Input should be a JSON string with 'hgvs' notation and optional 'gene' symbol.
    Example: {"hgvs": "NM_000492.3:c.1521_1523delCTT", "gene": "CFTR"}
    """
    
    def _run(self, query: str) -> str:
        try:
            import json
            params = json.loads(query)
            
            response = requests.post(
                "http://localhost:8080/tools/classify_variant",
                json={"variant_data": params}
            )
            result = response.json()
            
            return json.dumps(result, indent=2)
        except Exception as e:
            return f"Error classifying variant: {str(e)}"

class EvidenceQueryTool(BaseTool):
    name = "query_evidence"
    description = """
    Query evidence from genetic databases for a variant.
    Input should be variant identifier (HGVS, dbSNP, etc.)
    """
    
    def _run(self, variant: str) -> str:
        try:
            response = requests.post(
                "http://localhost:8080/tools/query_evidence",
                json={"variant": variant, "databases": ["all"]}
            )
            result = response.json()
            
            return json.dumps(result, indent=2)
        except Exception as e:
            return f"Error querying evidence: {str(e)}"

# Initialize agent with tools
tools = [VariantClassificationTool(), EvidenceQueryTool()]

agent = initialize_agent(
    tools=tools,
    llm=OpenAI(temperature=0),
    agent=AgentType.ZERO_SHOT_REACT_DESCRIPTION,
    verbose=True
)

# Use agent
response = agent.run(
    "Please classify the CFTR variant NM_000492.3:c.1521_1523delCTT and provide clinical recommendations"
)
```

## Web-Based Integrations

### RESTful API Interface

For AI systems that only support HTTP/REST:

```python
# flask-mcp-api.py
from flask import Flask, request, jsonify
import asyncio
from mcp_client import MCPClient

app = Flask(__name__)

class MCPAPIBridge:
    def __init__(self):
        self.mcp_client = MCPClient(
            server_command="./bin/mcp-server",
            server_args=["--config", "config/api.yaml"]
        )
        asyncio.run(self.mcp_client.connect())
    
    async def classify_variant(self, variant_data):
        return await self.mcp_client.call_tool("classify_variant", {
            "variant_data": variant_data
        })
    
    async def query_evidence(self, variant, databases=None):
        return await self.mcp_client.call_tool("query_evidence", {
            "variant": variant,
            "databases": databases or ["all"]
        })

bridge = MCPAPIBridge()

@app.route('/api/classify', methods=['POST'])
def api_classify():
    try:
        data = request.json
        result = asyncio.run(bridge.classify_variant(data.get('variant_data', {})))
        return jsonify(result)
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/api/evidence', methods=['POST'])  
def api_evidence():
    try:
        data = request.json
        result = asyncio.run(bridge.query_evidence(
            data.get('variant'),
            data.get('databases')
        ))
        return jsonify(result)
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/api/tools', methods=['GET'])
def api_tools():
    """Return available tools in OpenAPI format"""
    return jsonify({
        "openapi": "3.0.1",
        "info": {
            "title": "ACMG/AMP Variant Classification API",
            "version": "1.0.0"
        },
        "paths": {
            "/api/classify": {
                "post": {
                    "operationId": "classifyVariant",
                    "summary": "Classify genetic variant",
                    "requestBody": {
                        "required": True,
                        "content": {
                            "application/json": {
                                "schema": {
                                    "type": "object",
                                    "properties": {
                                        "variant_data": {
                                            "type": "object",
                                            "properties": {
                                                "hgvs": {"type": "string"},
                                                "gene": {"type": "string"}
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    })

if __name__ == '__main__':
    app.run(port=8080)
```

### WebSocket Interface

For real-time AI system integration:

```python
# websocket-mcp-bridge.py
import asyncio
import websockets
import json
from mcp_client import MCPClient

class WebSocketMCPBridge:
    def __init__(self):
        self.mcp_client = None
        self.clients = set()
    
    async def initialize(self):
        self.mcp_client = MCPClient(
            server_command="./bin/mcp-server",
            server_args=["--config", "config/websocket.yaml"]
        )
        await self.mcp_client.connect()
    
    async def handle_client(self, websocket, path):
        self.clients.add(websocket)
        try:
            async for message in websocket:
                data = json.loads(message)
                response = await self.handle_message(data)
                await websocket.send(json.dumps(response))
        finally:
            self.clients.remove(websocket)
    
    async def handle_message(self, data):
        try:
            method = data.get('method')
            params = data.get('params', {})
            
            if method == 'classify_variant':
                result = await self.mcp_client.call_tool('classify_variant', params)
            elif method == 'query_evidence':
                result = await self.mcp_client.call_tool('query_evidence', params)
            else:
                result = {"error": f"Unknown method: {method}"}
            
            return {
                "id": data.get('id'),
                "result": result
            }
        except Exception as e:
            return {
                "id": data.get('id'),
                "error": str(e)
            }

async def main():
    bridge = WebSocketMCPBridge()
    await bridge.initialize()
    
    start_server = websockets.serve(bridge.handle_client, "localhost", 8765)
    
    print("WebSocket MCP bridge running on ws://localhost:8765")
    await start_server
    await asyncio.Future()  # Run forever

if __name__ == '__main__':
    asyncio.run(main())
```

## Integration Best Practices

### Error Handling

```python
class RobustMCPBridge:
    def __init__(self):
        self.mcp_client = None
        self.max_retries = 3
        self.retry_delay = 1.0
    
    async def call_with_retry(self, tool_name, params):
        for attempt in range(self.max_retries):
            try:
                if not self.mcp_client:
                    await self.reconnect()
                
                return await self.mcp_client.call_tool(tool_name, params)
                
            except ConnectionError:
                if attempt == self.max_retries - 1:
                    raise
                await asyncio.sleep(self.retry_delay * (2 ** attempt))
                await self.reconnect()
            
            except Exception as e:
                if attempt == self.max_retries - 1:
                    raise
                await asyncio.sleep(self.retry_delay)
    
    async def reconnect(self):
        try:
            if self.mcp_client:
                await self.mcp_client.disconnect()
        except:
            pass
        
        self.mcp_client = MCPClient(
            server_command="./bin/mcp-server",
            server_args=["--config", "config/production.yaml"]
        )
        await self.mcp_client.connect()
```

### Performance Optimization

```python
import asyncio
from concurrent.futures import ThreadPoolExecutor

class OptimizedMCPBridge:
    def __init__(self):
        self.mcp_client = None
        self.executor = ThreadPoolExecutor(max_workers=5)
        self.cache = {}
        self.cache_ttl = 300  # 5 minutes
    
    async def cached_classification(self, variant_data):
        cache_key = json.dumps(variant_data, sort_keys=True)
        
        # Check cache
        if cache_key in self.cache:
            cached_result, timestamp = self.cache[cache_key]
            if time.time() - timestamp < self.cache_ttl:
                return cached_result
        
        # Call MCP tool
        result = await self.mcp_client.call_tool("classify_variant", {
            "variant_data": variant_data
        })
        
        # Cache result
        self.cache[cache_key] = (result, time.time())
        return result
    
    async def batch_classification(self, variant_list):
        # Process variants concurrently
        tasks = [
            self.cached_classification(variant)
            for variant in variant_list
        ]
        
        results = await asyncio.gather(*tasks, return_exceptions=True)
        return results
```

### Security Considerations

```python
class SecureMCPBridge:
    def __init__(self, api_key_validator):
        self.mcp_client = None
        self.api_key_validator = api_key_validator
        self.rate_limiter = {}
    
    def validate_request(self, request):
        # API key validation
        api_key = request.headers.get('Authorization', '').replace('Bearer ', '')
        if not self.api_key_validator(api_key):
            raise ValueError("Invalid API key")
        
        # Rate limiting
        client_ip = request.remote_addr
        current_time = time.time()
        
        if client_ip in self.rate_limiter:
            last_request, count = self.rate_limiter[client_ip]
            if current_time - last_request < 60:  # 1 minute window
                if count >= 10:  # Max 10 requests per minute
                    raise ValueError("Rate limit exceeded")
                self.rate_limiter[client_ip] = (last_request, count + 1)
            else:
                self.rate_limiter[client_ip] = (current_time, 1)
        else:
            self.rate_limiter[client_ip] = (current_time, 1)
    
    def sanitize_input(self, data):
        # Input validation and sanitization
        if 'variant_data' in data:
            variant_data = data['variant_data']
            
            # HGVS validation
            if 'hgvs' in variant_data:
                hgvs = variant_data['hgvs']
                if not re.match(r'^N[MR]_\d+\.\d+:[cgmnpr]\.[A-Za-z0-9_>*-]+$', hgvs):
                    raise ValueError(f"Invalid HGVS notation: {hgvs}")
            
            # Gene symbol validation
            if 'gene' in variant_data:
                gene = variant_data['gene']
                if not re.match(r'^[A-Z0-9-]+$', gene):
                    raise ValueError(f"Invalid gene symbol: {gene}")
        
        return data
```

## Testing and Validation

### Integration Testing

```python
import pytest
import asyncio

class TestMCPIntegration:
    @pytest.fixture
    async def mcp_bridge(self):
        bridge = MCPBridge()
        await bridge.initialize()
        yield bridge
        await bridge.cleanup()
    
    @pytest.mark.asyncio
    async def test_variant_classification(self, mcp_bridge):
        result = await mcp_bridge.classify_variant({
            "hgvs": "NM_000492.3:c.1521_1523delCTT",
            "gene": "CFTR"
        })
        
        assert result['classification'] == 'Pathogenic'
        assert result['confidence'] > 0.9
        assert 'PVS1' in result['applied_criteria']
    
    @pytest.mark.asyncio
    async def test_evidence_query(self, mcp_bridge):
        result = await mcp_bridge.query_evidence(
            "NM_000492.3:c.1521_1523delCTT"
        )
        
        assert 'clinvar' in result['sources']
        assert 'gnomad' in result['sources']
        assert result['evidence']['clinvar']['clinical_significance'] == 'Pathogenic'
    
    @pytest.mark.asyncio
    async def test_error_handling(self, mcp_bridge):
        with pytest.raises(Exception):
            await mcp_bridge.classify_variant({
                "hgvs": "invalid_hgvs"
            })
```

### Performance Benchmarking

```python
import time
import statistics

async def benchmark_integration(bridge, num_requests=100):
    test_variant = {
        "hgvs": "NM_000492.3:c.1521_1523delCTT",
        "gene": "CFTR"
    }
    
    response_times = []
    
    for i in range(num_requests):
        start_time = time.time()
        await bridge.classify_variant(test_variant)
        end_time = time.time()
        
        response_times.append(end_time - start_time)
    
    return {
        "mean_response_time": statistics.mean(response_times),
        "median_response_time": statistics.median(response_times),
        "p95_response_time": statistics.quantiles(response_times, n=20)[18],
        "max_response_time": max(response_times),
        "total_requests": num_requests
    }
```

## Deployment Considerations

### Production Deployment

```yaml
# docker-compose-ai-bridge.yml
version: '3.8'

services:
  mcp-server:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://user:pass@postgres:5432/acmg_amp_mcp
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

  ai-bridge:
    build: ./bridges/
    ports:
      - "8081:8081"
    environment:
      - MCP_SERVER_URL=http://mcp-server:8080
      - API_KEY_SECRET=${API_KEY_SECRET}
    depends_on:
      - mcp-server

  postgres:
    image: postgres:13
    environment:
      POSTGRES_DB: acmg_amp_mcp
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass

  redis:
    image: redis:7-alpine
```

### Monitoring and Logging

```python
import logging
from prometheus_client import Counter, Histogram, start_http_server

# Metrics
REQUEST_COUNT = Counter('mcp_requests_total', 'Total requests', ['method', 'status'])
REQUEST_DURATION = Histogram('mcp_request_duration_seconds', 'Request duration')

class MonitoredMCPBridge:
    def __init__(self):
        self.logger = logging.getLogger(__name__)
        start_http_server(8082)  # Prometheus metrics endpoint
    
    async def call_tool_with_monitoring(self, tool_name, params):
        with REQUEST_DURATION.time():
            try:
                result = await self.mcp_client.call_tool(tool_name, params)
                REQUEST_COUNT.labels(method=tool_name, status='success').inc()
                
                self.logger.info(f"Successful {tool_name} call", extra={
                    "tool": tool_name,
                    "params": params,
                    "result_classification": result.get('classification')
                })
                
                return result
                
            except Exception as e:
                REQUEST_COUNT.labels(method=tool_name, status='error').inc()
                
                self.logger.error(f"Failed {tool_name} call", extra={
                    "tool": tool_name,
                    "params": params,
                    "error": str(e)
                })
                
                raise
```

This comprehensive integration guide enables various AI systems to leverage the ACMG/AMP MCP Server's capabilities, regardless of their native protocol support. The bridge patterns and examples provided ensure broad compatibility while maintaining the rich functionality of the MCP protocol.