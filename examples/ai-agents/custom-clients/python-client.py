#!/usr/bin/env python3
"""
ACMG/AMP MCP Server - Python Client Example

This example demonstrates how to build a custom MCP client in Python
for interacting with the ACMG/AMP classification server.
"""

import asyncio
import json
import subprocess
import sys
import logging
from typing import Dict, List, Any, Optional
from dataclasses import dataclass
from datetime import datetime

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@dataclass
class MCPMessage:
    """MCP JSON-RPC 2.0 message structure"""
    jsonrpc: str = "2.0"
    id: Optional[int] = None
    method: Optional[str] = None
    params: Optional[Dict[str, Any]] = None
    result: Optional[Any] = None
    error: Optional[Dict[str, Any]] = None

class MCPClient:
    """Custom MCP client for ACMG/AMP server"""
    
    def __init__(self, server_command: str, server_args: List[str] = None):
        self.server_command = server_command
        self.server_args = server_args or []
        self.process = None
        self.message_id = 0
        self.capabilities = {}
        self.tools = []
        self.resources = []
        self.prompts = []

    async def connect(self):
        """Connect to MCP server via stdio"""
        logger.info(f"Starting MCP server: {self.server_command}")
        
        self.process = await asyncio.create_subprocess_exec(
            self.server_command,
            *self.server_args,
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE
        )
        
        # Initialize MCP session
        await self.initialize()
        
    async def disconnect(self):
        """Disconnect from MCP server"""
        if self.process:
            self.process.terminate()
            await self.process.wait()
            logger.info("MCP server terminated")

    async def send_message(self, message: MCPMessage) -> MCPMessage:
        """Send JSON-RPC message to server"""
        if not self.process:
            raise RuntimeError("Not connected to MCP server")
        
        # Serialize message
        message_json = json.dumps({
            "jsonrpc": message.jsonrpc,
            "id": message.id,
            "method": message.method,
            "params": message.params
        }) + "\n"
        
        logger.debug(f"Sending: {message_json.strip()}")
        
        # Send message
        self.process.stdin.write(message_json.encode())
        await self.process.stdin.drain()
        
        # Read response
        response_line = await self.process.stdout.readline()
        if not response_line:
            raise RuntimeError("Server connection closed")
        
        response_data = json.loads(response_line.decode())
        logger.debug(f"Received: {response_data}")
        
        return MCPMessage(
            jsonrpc=response_data.get("jsonrpc", "2.0"),
            id=response_data.get("id"),
            result=response_data.get("result"),
            error=response_data.get("error")
        )

    async def initialize(self):
        """Initialize MCP session"""
        self.message_id += 1
        
        init_message = MCPMessage(
            id=self.message_id,
            method="initialize",
            params={
                "protocolVersion": "2024-11-05",
                "capabilities": {
                    "roots": {"listChanged": True},
                    "sampling": {}
                },
                "clientInfo": {
                    "name": "acmg-amp-python-client",
                    "version": "1.0.0"
                }
            }
        )
        
        response = await self.send_message(init_message)
        
        if response.error:
            raise RuntimeError(f"Initialization failed: {response.error}")
        
        self.capabilities = response.result.get("capabilities", {})
        logger.info(f"MCP session initialized. Server capabilities: {list(self.capabilities.keys())}")

    async def list_tools(self) -> List[Dict[str, Any]]:
        """List available MCP tools"""
        self.message_id += 1
        
        message = MCPMessage(
            id=self.message_id,
            method="tools/list",
            params={}
        )
        
        response = await self.send_message(message)
        
        if response.error:
            raise RuntimeError(f"Failed to list tools: {response.error}")
        
        self.tools = response.result.get("tools", [])
        return self.tools

    async def call_tool(self, name: str, arguments: Dict[str, Any]) -> Any:
        """Call MCP tool"""
        self.message_id += 1
        
        message = MCPMessage(
            id=self.message_id,
            method="tools/call",
            params={
                "name": name,
                "arguments": arguments
            }
        )
        
        response = await self.send_message(message)
        
        if response.error:
            raise RuntimeError(f"Tool call failed: {response.error}")
        
        return response.result

    async def list_resources(self) -> List[Dict[str, Any]]:
        """List available MCP resources"""
        self.message_id += 1
        
        message = MCPMessage(
            id=self.message_id,
            method="resources/list",
            params={}
        )
        
        response = await self.send_message(message)
        
        if response.error:
            raise RuntimeError(f"Failed to list resources: {response.error}")
        
        self.resources = response.result.get("resources", [])
        return self.resources

    async def read_resource(self, uri: str) -> Any:
        """Read MCP resource"""
        self.message_id += 1
        
        message = MCPMessage(
            id=self.message_id,
            method="resources/read",
            params={"uri": uri}
        )
        
        response = await self.send_message(message)
        
        if response.error:
            raise RuntimeError(f"Failed to read resource: {response.error}")
        
        return response.result

    async def list_prompts(self) -> List[Dict[str, Any]]:
        """List available MCP prompts"""
        self.message_id += 1
        
        message = MCPMessage(
            id=self.message_id,
            method="prompts/list",
            params={}
        )
        
        response = await self.send_message(message)
        
        if response.error:
            raise RuntimeError(f"Failed to list prompts: {response.error}")
        
        self.prompts = response.result.get("prompts", [])
        return self.prompts

    async def get_prompt(self, name: str, arguments: Dict[str, Any] = None) -> Any:
        """Get MCP prompt"""
        self.message_id += 1
        
        message = MCPMessage(
            id=self.message_id,
            method="prompts/get",
            params={
                "name": name,
                "arguments": arguments or {}
            }
        )
        
        response = await self.send_message(message)
        
        if response.error:
            raise RuntimeError(f"Failed to get prompt: {response.error}")
        
        return response.result

class ACMGAMPClient:
    """High-level client for ACMG/AMP specific operations"""
    
    def __init__(self, mcp_client: MCPClient):
        self.mcp = mcp_client
    
    async def classify_variant(self, variant_data: Dict[str, Any], options: Dict[str, Any] = None) -> Dict[str, Any]:
        """Classify genetic variant using ACMG/AMP guidelines"""
        arguments = {
            "variant_data": variant_data,
            "options": options or {}
        }
        
        result = await self.mcp.call_tool("classify_variant", arguments)
        return result

    async def validate_hgvs(self, hgvs: str) -> Dict[str, Any]:
        """Validate HGVS notation"""
        result = await self.mcp.call_tool("validate_hgvs", {"hgvs": hgvs})
        return result

    async def query_evidence(self, variant: str, databases: List[str] = None) -> Dict[str, Any]:
        """Query evidence from external databases"""
        arguments = {
            "variant": variant,
            "databases": databases or ["all"]
        }
        
        result = await self.mcp.call_tool("query_evidence", arguments)
        return result

    async def generate_report(self, classification_result: Dict[str, Any], format: str = "clinical") -> Dict[str, Any]:
        """Generate classification report"""
        arguments = {
            "classification_data": classification_result,
            "format": format
        }
        
        result = await self.mcp.call_tool("generate_report", arguments)
        return result

    async def get_variant_info(self, variant_id: str) -> Dict[str, Any]:
        """Get variant information from resources"""
        resource_uri = f"variant/{variant_id}"
        result = await self.mcp.read_resource(resource_uri)
        return result

    async def get_acmg_rules(self) -> Dict[str, Any]:
        """Get ACMG/AMP classification rules"""
        result = await self.mcp.read_resource("acmg/rules")
        return result

async def example_workflow():
    """Example clinical workflow using the MCP client"""
    
    # Initialize client
    client = MCPClient("./bin/mcp-server", ["--config", "config/development.yaml"])
    acmg_client = ACMGAMPClient(client)
    
    try:
        await client.connect()
        
        print("=== ACMG/AMP MCP Client Example ===\n")
        
        # List available capabilities
        tools = await client.list_tools()
        print(f"Available tools: {[tool['name'] for tool in tools]}")
        
        resources = await client.list_resources()
        print(f"Available resources: {[res['uri'] for res in resources]}")
        
        prompts = await client.list_prompts()
        print(f"Available prompts: {[prompt['name'] for prompt in prompts]}\n")
        
        # Example variant classification
        print("=== Variant Classification Example ===")
        variant_data = {
            "hgvs": "NM_000492.3:c.1521_1523delCTT",
            "gene": "CFTR",
            "chromosome": "7",
            "position": 117199644,
            "ref": "CTT",
            "alt": "-"
        }
        
        print(f"Classifying variant: {variant_data['hgvs']}")
        
        # Validate HGVS first
        validation = await acmg_client.validate_hgvs(variant_data["hgvs"])
        print(f"HGVS validation: {validation.get('valid', False)}")
        
        if validation.get("valid"):
            # Gather evidence
            print("Gathering evidence...")
            evidence = await acmg_client.query_evidence(variant_data["hgvs"])
            print(f"Evidence sources: {evidence.get('sources', [])}")
            
            # Classify variant
            print("Performing classification...")
            classification = await acmg_client.classify_variant(variant_data, {
                "include_evidence": True,
                "confidence_threshold": 0.8
            })
            
            print(f"Classification: {classification.get('classification')}")
            print(f"Confidence: {classification.get('confidence', 0):.2f}")
            print(f"Applied criteria: {classification.get('applied_criteria', [])}")
            
            # Generate report
            print("\nGenerating clinical report...")
            report = await acmg_client.generate_report(classification, "clinical")
            print("Report generated successfully")
            
            # Save results
            results = {
                "timestamp": datetime.now().isoformat(),
                "variant": variant_data,
                "validation": validation,
                "evidence": evidence,
                "classification": classification,
                "report": report
            }
            
            with open("example_results.json", "w") as f:
                json.dump(results, f, indent=2)
            
            print("Results saved to example_results.json")
        
        # Example prompt usage
        print("\n=== Clinical Interpretation Prompt ===")
        prompt_result = await client.get_prompt("clinical_interpretation", {
            "variant": variant_data["hgvs"],
            "classification": classification.get("classification", "Unknown")
        })
        
        print("Clinical interpretation prompt:")
        for message in prompt_result.get("messages", []):
            print(f"- {message.get('role', 'system')}: {message.get('content', '')[:100]}...")
            
    except Exception as e:
        logger.error(f"Error in workflow: {e}")
        raise
        
    finally:
        await client.disconnect()

async def interactive_session():
    """Interactive session for testing MCP tools"""
    
    client = MCPClient("./bin/mcp-server", ["--config", "config/development.yaml"])
    acmg_client = ACMGAMPClient(client)
    
    try:
        await client.connect()
        print("Connected to ACMG/AMP MCP Server")
        print("Type 'help' for commands, 'quit' to exit\n")
        
        while True:
            try:
                command = input("mcp> ").strip()
                
                if command == "quit":
                    break
                elif command == "help":
                    print("Commands:")
                    print("  tools - List available tools")
                    print("  resources - List available resources")
                    print("  prompts - List available prompts")
                    print("  classify <hgvs> - Classify variant")
                    print("  evidence <variant> - Query evidence")
                    print("  validate <hgvs> - Validate HGVS")
                    print("  quit - Exit")
                elif command == "tools":
                    tools = await client.list_tools()
                    for tool in tools:
                        print(f"  {tool['name']}: {tool.get('description', 'No description')}")
                elif command == "resources":
                    resources = await client.list_resources()
                    for resource in resources:
                        print(f"  {resource['uri']}: {resource.get('name', 'No name')}")
                elif command == "prompts":
                    prompts = await client.list_prompts()
                    for prompt in prompts:
                        print(f"  {prompt['name']}: {prompt.get('description', 'No description')}")
                elif command.startswith("classify "):
                    hgvs = command[9:].strip()
                    result = await acmg_client.classify_variant({"hgvs": hgvs})
                    print(f"Classification: {result.get('classification')}")
                    print(f"Confidence: {result.get('confidence', 0):.2f}")
                elif command.startswith("evidence "):
                    variant = command[9:].strip()
                    result = await acmg_client.query_evidence(variant)
                    print(f"Evidence gathered from: {result.get('sources', [])}")
                elif command.startswith("validate "):
                    hgvs = command[9:].strip()
                    result = await acmg_client.validate_hgvs(hgvs)
                    print(f"Valid: {result.get('valid', False)}")
                    if result.get('normalized'):
                        print(f"Normalized: {result['normalized']}")
                else:
                    print("Unknown command. Type 'help' for available commands.")
                    
            except KeyboardInterrupt:
                break
            except Exception as e:
                print(f"Error: {e}")
                
    finally:
        await client.disconnect()

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "interactive":
        asyncio.run(interactive_session())
    else:
        asyncio.run(example_workflow())