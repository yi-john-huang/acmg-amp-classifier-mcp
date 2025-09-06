#!/usr/bin/env python3
"""
MCP Protocol Compliance Validation Suite

This validation suite ensures the ACMG/AMP MCP server fully complies with the
Model Context Protocol specification (2024-11-05).

Test Categories:
- JSON-RPC 2.0 Message Format Compliance
- MCP Protocol Version Negotiation
- Tool Discovery and Invocation
- Resource Access and Streaming
- Prompt Template Support
- Error Handling and Response Codes
- Transport Layer Compliance
"""

import asyncio
import json
import logging
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass, asdict
from typing import Dict, List, Any, Optional, Union
from datetime import datetime, timezone

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

@dataclass
class ValidationResult:
    """Result of a validation test"""
    test_name: str
    passed: bool
    message: str
    details: Optional[Dict[str, Any]] = None
    duration: Optional[float] = None

@dataclass
class ComplianceReport:
    """Overall compliance validation report"""
    timestamp: str
    server_version: str
    protocol_version: str
    total_tests: int
    passed_tests: int
    failed_tests: int
    compliance_score: float
    results: List[ValidationResult]
    summary: Dict[str, Any]

class MCPProtocolValidator:
    """MCP Protocol Compliance Validator"""
    
    def __init__(self, server_command: str, server_args: List[str] = None):
        self.server_command = server_command
        self.server_args = server_args or []
        self.process = None
        self.message_id = 0
        self.results = []
        
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
        
    async def disconnect(self):
        """Disconnect from MCP server"""
        if self.process:
            self.process.terminate()
            await self.process.wait()
            
    async def send_message(self, method: str, params: Dict[str, Any] = None, 
                          expect_error: bool = False) -> Dict[str, Any]:
        """Send JSON-RPC message to server"""
        self.message_id += 1
        
        message = {
            "jsonrpc": "2.0",
            "id": self.message_id,
            "method": method
        }
        
        if params is not None:
            message["params"] = params
            
        message_json = json.dumps(message) + "\n"
        logger.debug(f"Sending: {message_json.strip()}")
        
        # Send message
        self.process.stdin.write(message_json.encode())
        await self.process.stdin.drain()
        
        # Read response with timeout
        try:
            response_line = await asyncio.wait_for(
                self.process.stdout.readline(), 
                timeout=30.0
            )
        except asyncio.TimeoutError:
            raise RuntimeError("Response timeout")
            
        if not response_line:
            raise RuntimeError("Server connection closed")
            
        response = json.loads(response_line.decode())
        logger.debug(f"Received: {response}")
        
        return response
    
    def add_result(self, test_name: str, passed: bool, message: str, 
                   details: Dict[str, Any] = None, duration: float = None):
        """Add validation result"""
        result = ValidationResult(
            test_name=test_name,
            passed=passed,
            message=message,
            details=details,
            duration=duration
        )
        self.results.append(result)
        
        status = "✅ PASS" if passed else "❌ FAIL"
        logger.info(f"{status}: {test_name} - {message}")
        
    async def run_test(self, test_name: str, test_func):
        """Run individual validation test"""
        start_time = time.time()
        
        try:
            await test_func()
            duration = time.time() - start_time
            self.add_result(test_name, True, "Test passed", duration=duration)
            
        except Exception as e:
            duration = time.time() - start_time
            self.add_result(
                test_name, 
                False, 
                f"Test failed: {str(e)}", 
                details={"exception": str(e)},
                duration=duration
            )
    
    async def validate_jsonrpc_format(self):
        """Validate JSON-RPC 2.0 message format compliance"""
        
        # Test 1: Valid initialize message
        response = await self.send_message("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "mcp-validator",
                "version": "1.0.0"
            }
        })
        
        # Validate response format
        if "jsonrpc" not in response:
            raise ValueError("Response missing 'jsonrpc' field")
        if response["jsonrpc"] != "2.0":
            raise ValueError(f"Invalid jsonrpc version: {response['jsonrpc']}")
        if "id" not in response:
            raise ValueError("Response missing 'id' field")
        if response["id"] != self.message_id:
            raise ValueError(f"Response ID mismatch: {response['id']} != {self.message_id}")
        if "result" not in response and "error" not in response:
            raise ValueError("Response missing both 'result' and 'error' fields")
            
        # Test 2: Invalid message (missing required fields)
        try:
            await self.send_message("invalid_method", expect_error=True)
        except:
            pass  # Expected to fail
            
        # Test 3: Malformed JSON handling would be tested at transport layer
    
    async def validate_protocol_negotiation(self):
        """Validate MCP protocol version negotiation"""
        
        # Test supported protocol version
        response = await self.send_message("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "mcp-validator",
                "version": "1.0.0"
            }
        })
        
        if "error" in response:
            raise ValueError(f"Initialize failed: {response['error']}")
            
        result = response.get("result", {})
        if "protocolVersion" not in result:
            raise ValueError("Server response missing protocolVersion")
        if "capabilities" not in result:
            raise ValueError("Server response missing capabilities")
        if "serverInfo" not in result:
            raise ValueError("Server response missing serverInfo")
            
        # Validate server capabilities structure
        capabilities = result["capabilities"]
        expected_capabilities = ["tools", "resources", "prompts", "logging"]
        
        for cap in expected_capabilities:
            if cap not in capabilities:
                logger.warning(f"Server missing capability: {cap}")
    
    async def validate_tool_discovery(self):
        """Validate tool discovery and metadata"""
        
        # List available tools
        response = await self.send_message("tools/list")
        
        if "error" in response:
            raise ValueError(f"tools/list failed: {response['error']}")
            
        result = response.get("result", {})
        if "tools" not in result:
            raise ValueError("tools/list response missing 'tools' field")
            
        tools = result["tools"]
        if not isinstance(tools, list):
            raise ValueError("tools field must be an array")
            
        # Validate tool metadata
        expected_tools = [
            "classify_variant",
            "validate_hgvs", 
            "query_evidence",
            "generate_report"
        ]
        
        found_tools = [tool.get("name") for tool in tools]
        
        for expected_tool in expected_tools:
            if expected_tool not in found_tools:
                raise ValueError(f"Missing expected tool: {expected_tool}")
                
        # Validate tool schema
        for tool in tools:
            required_fields = ["name", "description"]
            for field in required_fields:
                if field not in tool:
                    raise ValueError(f"Tool missing required field '{field}': {tool}")
                    
            # Validate input schema if present
            if "inputSchema" in tool:
                schema = tool["inputSchema"]
                if not isinstance(schema, dict):
                    raise ValueError(f"Tool inputSchema must be object: {tool['name']}")
    
    async def validate_tool_invocation(self):
        """Validate tool invocation and response format"""
        
        # Test valid tool call
        response = await self.send_message("tools/call", {
            "name": "validate_hgvs",
            "arguments": {
                "hgvs": "NM_000492.3:c.1521_1523delCTT"
            }
        })
        
        if "error" in response:
            raise ValueError(f"Tool call failed: {response['error']}")
            
        result = response.get("result")
        if result is None:
            raise ValueError("Tool call response missing result")
            
        # Validate tool response structure
        if not isinstance(result, dict):
            raise ValueError("Tool result must be object")
            
        # Test invalid tool call
        response = await self.send_message("tools/call", {
            "name": "nonexistent_tool",
            "arguments": {}
        })
        
        if "error" not in response:
            raise ValueError("Invalid tool call should return error")
            
        error = response["error"]
        if "code" not in error or "message" not in error:
            raise ValueError("Error response missing required fields")
    
    async def validate_resource_discovery(self):
        """Validate resource discovery and access"""
        
        # List available resources
        response = await self.send_message("resources/list")
        
        if "error" in response:
            raise ValueError(f"resources/list failed: {response['error']}")
            
        result = response.get("result", {})
        if "resources" not in result:
            raise ValueError("resources/list response missing 'resources' field")
            
        resources = result["resources"]
        if not isinstance(resources, list):
            raise ValueError("resources field must be an array")
            
        # Validate resource metadata
        expected_resources = [
            "acmg/rules"
        ]
        
        found_resources = []
        for resource in resources:
            if "uri" in resource:
                found_resources.append(resource["uri"])
            elif "uriTemplate" in resource:
                found_resources.append(resource["uriTemplate"])
                
        for expected_resource in expected_resources:
            found = any(expected_resource in uri for uri in found_resources)
            if not found:
                logger.warning(f"Expected resource not found: {expected_resource}")
                
        # Validate resource schema
        for resource in resources:
            if "uri" not in resource and "uriTemplate" not in resource:
                raise ValueError(f"Resource missing URI: {resource}")
            if "name" not in resource:
                raise ValueError(f"Resource missing name: {resource}")
    
    async def validate_resource_access(self):
        """Validate resource reading and content"""
        
        # Test static resource access
        response = await self.send_message("resources/read", {
            "uri": "acmg/rules"
        })
        
        if "error" in response:
            raise ValueError(f"Resource read failed: {response['error']}")
            
        result = response.get("result")
        if result is None:
            raise ValueError("Resource read response missing result")
            
        # Validate resource content structure
        if "contents" not in result:
            raise ValueError("Resource response missing contents")
            
        contents = result["contents"]
        if not isinstance(contents, list):
            raise ValueError("Resource contents must be array")
            
        if len(contents) == 0:
            raise ValueError("Resource contents empty")
            
        # Validate content item structure
        for content in contents:
            if "uri" not in content:
                raise ValueError("Content item missing URI")
            if "mimeType" not in content:
                raise ValueError("Content item missing mimeType")
            if "text" not in content and "blob" not in content:
                raise ValueError("Content item missing text or blob")
    
    async def validate_prompt_support(self):
        """Validate prompt template support"""
        
        # List available prompts
        response = await self.send_message("prompts/list")
        
        if "error" in response:
            raise ValueError(f"prompts/list failed: {response['error']}")
            
        result = response.get("result", {})
        if "prompts" not in result:
            raise ValueError("prompts/list response missing 'prompts' field")
            
        prompts = result["prompts"]
        if not isinstance(prompts, list):
            raise ValueError("prompts field must be an array")
            
        expected_prompts = [
            "clinical_interpretation",
            "evidence_review"
        ]
        
        found_prompts = [prompt.get("name") for prompt in prompts]
        
        for expected_prompt in expected_prompts:
            if expected_prompt not in found_prompts:
                logger.warning(f"Expected prompt not found: {expected_prompt}")
                
        # Test prompt retrieval
        if found_prompts:
            first_prompt = found_prompts[0]
            response = await self.send_message("prompts/get", {
                "name": first_prompt,
                "arguments": {}
            })
            
            if "error" in response:
                raise ValueError(f"Prompt get failed: {response['error']}")
                
            result = response.get("result")
            if "messages" not in result:
                raise ValueError("Prompt response missing messages")
    
    async def validate_error_handling(self):
        """Validate error handling and response codes"""
        
        # Test invalid method
        response = await self.send_message("invalid/method")
        
        if "error" not in response:
            raise ValueError("Invalid method should return error")
            
        error = response["error"]
        if error.get("code") != -32601:  # Method not found
            logger.warning(f"Unexpected error code for invalid method: {error.get('code')}")
            
        # Test invalid parameters
        response = await self.send_message("tools/call", {
            "name": "validate_hgvs"
            # Missing required arguments
        })
        
        if "error" not in response:
            raise ValueError("Invalid parameters should return error")
            
        error = response["error"]
        if error.get("code") != -32602:  # Invalid params
            logger.warning(f"Unexpected error code for invalid params: {error.get('code')}")
            
        # Test malformed request structure
        # This would be tested at the transport layer level
    
    async def validate_logging_support(self):
        """Validate logging capability support"""
        
        # Test logging/setLevel (if supported)
        try:
            response = await self.send_message("logging/setLevel", {
                "level": "info"
            })
            
            # Logging support is optional, so don't fail if not supported
            if "error" in response:
                error_code = response["error"].get("code")
                if error_code == -32601:  # Method not found
                    logger.info("Logging capability not supported (optional)")
                else:
                    logger.warning(f"Logging/setLevel error: {response['error']}")
        except Exception as e:
            logger.info(f"Logging capability test failed (optional): {e}")
    
    async def run_all_validations(self) -> ComplianceReport:
        """Run all protocol compliance validations"""
        
        logger.info("Starting MCP Protocol Compliance Validation")
        
        try:
            await self.connect()
            
            # Define validation tests
            tests = [
                ("JSON-RPC 2.0 Format Compliance", self.validate_jsonrpc_format),
                ("Protocol Version Negotiation", self.validate_protocol_negotiation),
                ("Tool Discovery", self.validate_tool_discovery),
                ("Tool Invocation", self.validate_tool_invocation),
                ("Resource Discovery", self.validate_resource_discovery),
                ("Resource Access", self.validate_resource_access),
                ("Prompt Support", self.validate_prompt_support),
                ("Error Handling", self.validate_error_handling),
                ("Logging Support", self.validate_logging_support)
            ]
            
            # Run each validation test
            for test_name, test_func in tests:
                await self.run_test(test_name, test_func)
                
        finally:
            await self.disconnect()
        
        # Generate compliance report
        total_tests = len(self.results)
        passed_tests = sum(1 for r in self.results if r.passed)
        failed_tests = total_tests - passed_tests
        compliance_score = (passed_tests / total_tests) * 100 if total_tests > 0 else 0
        
        report = ComplianceReport(
            timestamp=datetime.now(timezone.utc).isoformat(),
            server_version="1.0.0",  # Would be detected from server
            protocol_version="2024-11-05",
            total_tests=total_tests,
            passed_tests=passed_tests,
            failed_tests=failed_tests,
            compliance_score=compliance_score,
            results=self.results,
            summary={
                "overall_status": "COMPLIANT" if failed_tests == 0 else "NON_COMPLIANT",
                "critical_failures": [
                    r.test_name for r in self.results 
                    if not r.passed and "JSON-RPC" in r.test_name
                ],
                "optional_failures": [
                    r.test_name for r in self.results 
                    if not r.passed and "Logging" in r.test_name
                ],
                "performance_summary": {
                    "average_response_time": sum(
                        r.duration for r in self.results 
                        if r.duration is not None
                    ) / len([r for r in self.results if r.duration is not None]),
                    "slowest_test": max(
                        self.results, 
                        key=lambda r: r.duration or 0
                    ).test_name if self.results else None
                }
            }
        )
        
        return report

def format_compliance_report(report: ComplianceReport) -> str:
    """Format compliance report for display"""
    
    status_emoji = "✅" if report.compliance_score == 100 else "⚠️" if report.compliance_score >= 80 else "❌"
    
    output = f"""
{status_emoji} MCP PROTOCOL COMPLIANCE VALIDATION REPORT {status_emoji}

📅 Validation Date: {report.timestamp}
🔌 Protocol Version: {report.protocol_version}
🏆 Compliance Score: {report.compliance_score:.1f}% ({report.passed_tests}/{report.total_tests} tests passed)

{'='*80}
DETAILED RESULTS
{'='*80}
"""
    
    for result in report.results:
        status = "✅ PASS" if result.passed else "❌ FAIL"
        duration_str = f" ({result.duration:.3f}s)" if result.duration else ""
        
        output += f"\n{status} {result.test_name}{duration_str}\n"
        output += f"    {result.message}\n"
        
        if result.details:
            for key, value in result.details.items():
                output += f"    {key}: {value}\n"
    
    output += f"\n{'='*80}\nSUMMARY\n{'='*80}\n"
    output += f"Overall Status: {report.summary['overall_status']}\n"
    
    if report.summary['critical_failures']:
        output += f"Critical Failures: {', '.join(report.summary['critical_failures'])}\n"
    
    if report.summary['optional_failures']:
        output += f"Optional Failures: {', '.join(report.summary['optional_failures'])}\n"
    
    perf = report.summary['performance_summary']
    if perf['average_response_time']:
        output += f"Average Response Time: {perf['average_response_time']:.3f}s\n"
    if perf['slowest_test']:
        output += f"Slowest Test: {perf['slowest_test']}\n"
    
    output += f"\n{'='*80}\nRECOMMENDATIONS\n{'='*80}\n"
    
    if report.compliance_score == 100:
        output += "🎉 Excellent! Server is fully MCP protocol compliant.\n"
    elif report.compliance_score >= 90:
        output += "✅ Good compliance. Address any remaining issues for full compliance.\n"
    elif report.compliance_score >= 70:
        output += "⚠️ Moderate compliance. Several issues need attention.\n"
    else:
        output += "❌ Poor compliance. Significant protocol violations detected.\n"
    
    return output

async def main():
    """Main validation function"""
    
    if len(sys.argv) < 2:
        print("Usage: python3 mcp-protocol-compliance.py <server_command> [args...]")
        print("Example: python3 mcp-protocol-compliance.py ./bin/mcp-server --config config/test.yaml")
        sys.exit(1)
    
    server_command = sys.argv[1]
    server_args = sys.argv[2:] if len(sys.argv) > 2 else []
    
    validator = MCPProtocolValidator(server_command, server_args)
    
    try:
        report = await validator.run_all_validations()
        
        # Display report
        print(format_compliance_report(report))
        
        # Save detailed report
        report_file = f"mcp_compliance_report_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
        with open(report_file, 'w') as f:
            # Convert dataclasses to dict for JSON serialization
            report_dict = asdict(report)
            json.dump(report_dict, f, indent=2, default=str)
        
        print(f"\nDetailed report saved to: {report_file}")
        
        # Exit with appropriate code
        sys.exit(0 if report.failed_tests == 0 else 1)
        
    except Exception as e:
        logger.error(f"Validation failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    asyncio.run(main())