# ChatGPT MCP Client Integration

This guide demonstrates how to integrate the ACMG/AMP MCP Server with ChatGPT and other OpenAI-compatible systems using custom MCP client implementations.

## Overview

Since ChatGPT doesn't natively support MCP (Model Context Protocol), we provide several integration approaches:

1. **HTTP Bridge Client** - Expose MCP tools as REST API endpoints
2. **Custom GPT Integration** - Use the HTTP bridge with custom GPT actions
3. **OpenAI Assistant Integration** - Integrate via OpenAI Assistants API
4. **Proxy Service** - MCP-to-OpenAI function calling bridge

## Method 1: HTTP Bridge Client

### Setup

1. **Start the MCP HTTP Bridge**:
```bash
# Using the provided HTTP bridge script
./scripts/mcp-http-bridge.js --port 8080 --mcp-server ./bin/mcp-server

# Or using Docker
docker run -p 8080:8080 -e MCP_TRANSPORT=http mcp-acmg-amp-server:latest
```

2. **Configure Custom GPT**:
```yaml
# custom-gpt-schema.yaml
openapi: 3.0.1
info:
  title: ACMG/AMP Variant Classification
  description: Clinical variant classification using ACMG/AMP guidelines
  version: 1.0.0
servers:
  - url: http://localhost:8080
paths:
  /tools/classify_variant:
    post:
      operationId: classifyVariant
      summary: Classify genetic variant using ACMG/AMP guidelines
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                variant_data:
                  type: object
                  properties:
                    hgvs: 
                      type: string
                      description: HGVS notation of the variant
                    gene:
                      type: string
                      description: Gene symbol
                    chromosome:
                      type: string
                      description: Chromosome number
                    position:
                      type: integer
                      description: Genomic position
                    ref:
                      type: string
                      description: Reference allele
                    alt:
                      type: string
                      description: Alternative allele
                options:
                  type: object
                  properties:
                    include_evidence:
                      type: boolean
                      default: true
                    report_format:
                      type: string
                      enum: ["json", "text", "clinical"]
                      default: "clinical"
              required: ["variant_data"]
      responses:
        '200':
          description: Classification result
          content:
            application/json:
              schema:
                type: object
                properties:
                  classification:
                    type: string
                    enum: ["Pathogenic", "Likely Pathogenic", "Uncertain Significance", "Likely Benign", "Benign"]
                  confidence:
                    type: number
                  evidence_summary:
                    type: object
                  recommendation:
                    type: string
  /tools/query_evidence:
    post:
      operationId: queryEvidence
      summary: Gather evidence for variant from multiple databases
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                variant:
                  type: string
                  description: Variant identifier (HGVS, dbSNP, etc.)
                databases:
                  type: array
                  items:
                    type: string
                    enum: ["clinvar", "gnomad", "cosmic", "all"]
                  default: ["all"]
      responses:
        '200':
          description: Evidence data
          content:
            application/json:
              schema:
                type: object
                properties:
                  evidence:
                    type: object
                  sources:
                    type: array
                    items:
                      type: string
```

### Usage Examples

**Custom GPT Prompt**:
```
You are a clinical geneticist assistant with access to ACMG/AMP variant classification tools. 

When a user provides a genetic variant, you should:
1. First validate the variant notation using the available tools
2. Gather evidence from relevant databases
3. Apply ACMG/AMP classification criteria
4. Provide a comprehensive interpretation with clinical recommendations

Always explain your reasoning and cite the evidence sources used.
```

**Example Interaction**:
```
User: "Please classify the BRCA1 variant c.185delA"

GPT Response: "I'll help you classify this BRCA1 variant using ACMG/AMP guidelines. Let me gather the necessary information..."

[Calls classifyVariant tool with variant data]

"Based on the analysis:
- Classification: Pathogenic
- Confidence: High (0.95)
- Evidence: This variant causes a frameshift leading to premature termination
- Clinical Significance: Associated with increased breast and ovarian cancer risk
- Recommendation: Genetic counseling and enhanced screening recommended"
```

## Method 2: OpenAI Assistant Integration

### Setup Assistant with Function Calling

```python
# create_assistant.py
import openai
import json

client = openai.OpenAI()

# Define function schemas that mirror MCP tools
function_schemas = [
    {
        "type": "function",
        "function": {
            "name": "classify_variant",
            "description": "Classify a genetic variant using ACMG/AMP guidelines",
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
                    "include_evidence": {
                        "type": "boolean",
                        "description": "Include detailed evidence in response",
                        "default": True
                    }
                },
                "required": ["hgvs"]
            }
        }
    },
    {
        "type": "function", 
        "function": {
            "name": "query_evidence",
            "description": "Query evidence databases for variant information",
            "parameters": {
                "type": "object",
                "properties": {
                    "variant": {
                        "type": "string",
                        "description": "Variant identifier"
                    },
                    "databases": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "enum": ["clinvar", "gnomad", "cosmic"]
                        },
                        "description": "Databases to query"
                    }
                },
                "required": ["variant"]
            }
        }
    }
]

# Create assistant
assistant = client.beta.assistants.create(
    name="ACMG/AMP Variant Classifier",
    instructions="""
    You are a clinical geneticist assistant specializing in variant interpretation 
    using ACMG/AMP guidelines. You have access to tools for variant classification 
    and evidence gathering from clinical databases.
    
    When classifying variants:
    1. Always gather evidence from multiple sources
    2. Apply ACMG/AMP criteria systematically  
    3. Provide clear clinical recommendations
    4. Explain your reasoning and cite sources
    5. Highlight any limitations or uncertainties
    """,
    model="gpt-4-turbo",
    tools=function_schemas
)

print(f"Assistant created with ID: {assistant.id}")
```

### Function Handler Bridge

```python
# function_handler.py
import requests
import json
import asyncio
from typing import Dict, Any

class MCPBridge:
    def __init__(self, mcp_server_url: str = "http://localhost:8080"):
        self.mcp_server_url = mcp_server_url
    
    async def classify_variant(self, **kwargs) -> Dict[str, Any]:
        """Bridge function for variant classification"""
        response = requests.post(
            f"{self.mcp_server_url}/tools/classify_variant",
            json={"variant_data": kwargs}
        )
        return response.json()
    
    async def query_evidence(self, **kwargs) -> Dict[str, Any]:
        """Bridge function for evidence querying"""
        response = requests.post(
            f"{self.mcp_server_url}/tools/query_evidence", 
            json=kwargs
        )
        return response.json()

# Function dispatcher
async def handle_function_call(function_name: str, arguments: Dict[str, Any]) -> str:
    bridge = MCPBridge()
    
    if function_name == "classify_variant":
        result = await bridge.classify_variant(**arguments)
    elif function_name == "query_evidence":
        result = await bridge.query_evidence(**arguments)
    else:
        return f"Unknown function: {function_name}"
    
    return json.dumps(result, indent=2)
```

## Method 3: MCP-to-OpenAI Proxy Service

### Proxy Server Implementation

```javascript
// mcp-openai-proxy.js
const express = require('express');
const { MCPClient } = require('@mcp/client');
const OpenAI = require('openai');

const app = express();
app.use(express.json());

class MCPOpenAIProxy {
    constructor() {
        this.mcpClient = new MCPClient({
            transport: 'stdio',
            command: './bin/mcp-server',
            args: ['--config', 'config/development.yaml']
        });
        
        this.openai = new OpenAI({
            apiKey: process.env.OPENAI_API_KEY
        });
    }

    async initialize() {
        await this.mcpClient.connect();
        
        // Get available tools from MCP server
        const tools = await this.mcpClient.listTools();
        this.mcpTools = tools.tools;
        
        // Convert MCP tools to OpenAI function format
        this.openaiTools = this.mcpTools.map(tool => ({
            type: 'function',
            function: {
                name: tool.name,
                description: tool.description,
                parameters: tool.inputSchema
            }
        }));
    }

    async chat(messages, options = {}) {
        const response = await this.openai.chat.completions.create({
            model: 'gpt-4-turbo',
            messages,
            tools: this.openaiTools,
            tool_choice: 'auto',
            ...options
        });

        // Handle tool calls
        if (response.choices[0].message.tool_calls) {
            for (const toolCall of response.choices[0].message.tool_calls) {
                const result = await this.mcpClient.callTool(
                    toolCall.function.name,
                    JSON.parse(toolCall.function.arguments)
                );
                
                // Add tool result to conversation
                messages.push({
                    role: 'tool',
                    tool_call_id: toolCall.id,
                    content: JSON.stringify(result)
                });
            }
            
            // Get final response
            return await this.chat(messages, options);
        }

        return response;
    }
}

// Initialize proxy
const proxy = new MCPOpenAIProxy();

app.post('/chat/completions', async (req, res) => {
    try {
        const { messages, ...options } = req.body;
        const response = await proxy.chat(messages, options);
        res.json(response);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

app.listen(3000, async () => {
    await proxy.initialize();
    console.log('MCP-OpenAI Proxy running on port 3000');
});
```

## Usage Examples

### Basic Variant Classification
```javascript
const response = await fetch('http://localhost:3000/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        messages: [
            {
                role: 'user',
                content: 'Please classify the variant NM_000492.3:c.1521_1523delCTT in the CFTR gene'
            }
        ],
        model: 'gpt-4-turbo'
    })
});
```

### Comprehensive Clinical Assessment
```javascript
const response = await fetch('http://localhost:3000/chat/completions', {
    method: 'POST', 
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        messages: [
            {
                role: 'system',
                content: 'You are a clinical geneticist. Provide comprehensive variant assessments including classification, evidence summary, and clinical recommendations.'
            },
            {
                role: 'user', 
                content: 'A 35-year-old woman with family history of breast cancer has been found to carry the BRCA1 variant c.185delA. Please provide a complete clinical assessment.'
            }
        ],
        model: 'gpt-4-turbo'
    })
});
```

## Deployment Considerations

### Security
- Use API authentication for production deployments
- Implement rate limiting to prevent abuse
- Ensure HIPAA compliance for clinical data
- Use encrypted connections (HTTPS/TLS)

### Scalability  
- Deploy proxy service with load balancing
- Use connection pooling for MCP server connections
- Implement caching for frequently accessed data
- Monitor performance and resource usage

### Error Handling
- Graceful degradation when MCP server is unavailable
- Comprehensive error logging and monitoring
- User-friendly error messages
- Automatic retry logic for transient failures

## Next Steps

- See `workflows/chatgpt-examples.md` for detailed usage scenarios
- Review `custom-clients/` for building your own integrations  
- Check `troubleshooting.md` for common issues and solutions