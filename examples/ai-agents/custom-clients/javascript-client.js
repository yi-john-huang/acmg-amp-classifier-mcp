#!/usr/bin/env node
/**
 * ACMG/AMP MCP Server - JavaScript/Node.js Client Example
 * 
 * This example demonstrates how to build a custom MCP client in JavaScript
 * for interacting with the ACMG/AMP classification server.
 */

const { spawn } = require('child_process');
const { EventEmitter } = require('events');
const readline = require('readline');
const fs = require('fs').promises;

class MCPClient extends EventEmitter {
    constructor(serverCommand, serverArgs = []) {
        super();
        this.serverCommand = serverCommand;
        this.serverArgs = serverArgs;
        this.process = null;
        this.messageId = 0;
        this.pendingRequests = new Map();
        this.capabilities = {};
        this.tools = [];
        this.resources = [];
        this.prompts = [];
    }

    async connect() {
        console.log(`Starting MCP server: ${this.serverCommand}`);
        
        this.process = spawn(this.serverCommand, this.serverArgs, {
            stdio: ['pipe', 'pipe', 'pipe']
        });

        this.process.on('error', (error) => {
            this.emit('error', error);
        });

        this.process.on('exit', (code) => {
            console.log(`MCP server exited with code ${code}`);
            this.emit('disconnect');
        });

        // Set up message handling
        const rl = readline.createInterface({
            input: this.process.stdout,
            crlfDelay: Infinity
        });

        rl.on('line', (line) => {
            try {
                const message = JSON.parse(line);
                this.handleMessage(message);
            } catch (error) {
                console.error('Failed to parse message:', error);
            }
        });

        // Initialize MCP session
        await this.initialize();
    }

    async disconnect() {
        if (this.process) {
            this.process.kill();
            this.process = null;
        }
    }

    handleMessage(message) {
        const { id, result, error } = message;
        
        if (id && this.pendingRequests.has(id)) {
            const { resolve, reject } = this.pendingRequests.get(id);
            this.pendingRequests.delete(id);
            
            if (error) {
                reject(new Error(error.message || 'MCP error'));
            } else {
                resolve(result);
            }
        }
    }

    async sendMessage(method, params = {}) {
        return new Promise((resolve, reject) => {
            if (!this.process) {
                reject(new Error('Not connected to MCP server'));
                return;
            }

            const id = ++this.messageId;
            const message = {
                jsonrpc: '2.0',
                id,
                method,
                params
            };

            this.pendingRequests.set(id, { resolve, reject });

            const messageJson = JSON.stringify(message) + '\n';
            console.log(`Sending: ${messageJson.trim()}`);
            
            this.process.stdin.write(messageJson);

            // Set timeout
            setTimeout(() => {
                if (this.pendingRequests.has(id)) {
                    this.pendingRequests.delete(id);
                    reject(new Error('Request timeout'));
                }
            }, 30000);
        });
    }

    async initialize() {
        const result = await this.sendMessage('initialize', {
            protocolVersion: '2024-11-05',
            capabilities: {
                roots: { listChanged: true },
                sampling: {}
            },
            clientInfo: {
                name: 'acmg-amp-js-client',
                version: '1.0.0'
            }
        });

        this.capabilities = result.capabilities || {};
        console.log(`MCP session initialized. Server capabilities: ${Object.keys(this.capabilities)}`);
    }

    async listTools() {
        const result = await this.sendMessage('tools/list');
        this.tools = result.tools || [];
        return this.tools;
    }

    async callTool(name, arguments = {}) {
        return await this.sendMessage('tools/call', { name, arguments });
    }

    async listResources() {
        const result = await this.sendMessage('resources/list');
        this.resources = result.resources || [];
        return this.resources;
    }

    async readResource(uri) {
        return await this.sendMessage('resources/read', { uri });
    }

    async listPrompts() {
        const result = await this.sendMessage('prompts/list');
        this.prompts = result.prompts || [];
        return this.prompts;
    }

    async getPrompt(name, arguments = {}) {
        return await this.sendMessage('prompts/get', { name, arguments });
    }
}

class ACMGAMPClient {
    constructor(mcpClient) {
        this.mcp = mcpClient;
    }

    async classifyVariant(variantData, options = {}) {
        return await this.mcp.callTool('classify_variant', {
            variant_data: variantData,
            options
        });
    }

    async validateHgvs(hgvs) {
        return await this.mcp.callTool('validate_hgvs', { hgvs });
    }

    async queryEvidence(variant, databases = ['all']) {
        return await this.mcp.callTool('query_evidence', {
            variant,
            databases
        });
    }

    async generateReport(classificationData, format = 'clinical') {
        return await this.mcp.callTool('generate_report', {
            classification_data: classificationData,
            format
        });
    }

    async getVariantInfo(variantId) {
        return await this.mcp.readResource(`variant/${variantId}`);
    }

    async getAcmgRules() {
        return await this.mcp.readResource('acmg/rules');
    }
}

async function exampleWorkflow() {
    console.log('=== ACMG/AMP MCP Client Example ===\n');

    const client = new MCPClient('./bin/mcp-server', ['--config', 'config/development.yaml']);
    const acmgClient = new ACMGAMPClient(client);

    try {
        await client.connect();

        // List available capabilities
        const tools = await client.listTools();
        console.log(`Available tools: ${tools.map(t => t.name).join(', ')}`);

        const resources = await client.listResources();
        console.log(`Available resources: ${resources.map(r => r.uri).join(', ')}`);

        const prompts = await client.listPrompts();
        console.log(`Available prompts: ${prompts.map(p => p.name).join(', ')}\n`);

        // Example variant classification
        console.log('=== Variant Classification Example ===');
        const variantData = {
            hgvs: 'NM_000492.3:c.1521_1523delCTT',
            gene: 'CFTR',
            chromosome: '7',
            position: 117199644,
            ref: 'CTT',
            alt: '-'
        };

        console.log(`Classifying variant: ${variantData.hgvs}`);

        // Validate HGVS first
        console.log('Validating HGVS...');
        const validation = await acmgClient.validateHgvs(variantData.hgvs);
        console.log(`HGVS validation: ${validation.valid ? 'Valid' : 'Invalid'}`);

        if (validation.valid) {
            // Gather evidence
            console.log('Gathering evidence...');
            const evidence = await acmgClient.queryEvidence(variantData.hgvs);
            console.log(`Evidence sources: ${evidence.sources?.join(', ') || 'None'}`);

            // Classify variant
            console.log('Performing classification...');
            const classification = await acmgClient.classifyVariant(variantData, {
                include_evidence: true,
                confidence_threshold: 0.8
            });

            console.log(`Classification: ${classification.classification}`);
            console.log(`Confidence: ${(classification.confidence || 0).toFixed(2)}`);
            console.log(`Applied criteria: ${classification.applied_criteria?.join(', ') || 'None'}`);

            // Generate report
            console.log('\nGenerating clinical report...');
            const report = await acmgClient.generateReport(classification, 'clinical');
            console.log('Report generated successfully');

            // Save results
            const results = {
                timestamp: new Date().toISOString(),
                variant: variantData,
                validation,
                evidence,
                classification,
                report
            };

            await fs.writeFile('example_results.json', JSON.stringify(results, null, 2));
            console.log('Results saved to example_results.json');
        }

        // Example prompt usage
        console.log('\n=== Clinical Interpretation Prompt ===');
        const promptResult = await client.getPrompt('clinical_interpretation', {
            variant: variantData.hgvs,
            classification: classification?.classification || 'Unknown'
        });

        console.log('Clinical interpretation prompt:');
        promptResult.messages?.forEach(msg => {
            const preview = (msg.content || '').substring(0, 100);
            console.log(`- ${msg.role}: ${preview}${preview.length === 100 ? '...' : ''}`);
        });

    } catch (error) {
        console.error('Error in workflow:', error.message);
    } finally {
        await client.disconnect();
    }
}

async function interactiveSession() {
    const client = new MCPClient('./bin/mcp-server', ['--config', 'config/development.yaml']);
    const acmgClient = new ACMGAMPClient(client);

    try {
        await client.connect();
        console.log('Connected to ACMG/AMP MCP Server');
        console.log("Type 'help' for commands, 'quit' to exit\n");

        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout
        });

        const question = (prompt) => new Promise(resolve => rl.question(prompt, resolve));

        while (true) {
            try {
                const command = await question('mcp> ');

                if (command.trim() === 'quit') {
                    break;
                } else if (command.trim() === 'help') {
                    console.log('Commands:');
                    console.log('  tools - List available tools');
                    console.log('  resources - List available resources');
                    console.log('  prompts - List available prompts');
                    console.log('  classify <hgvs> - Classify variant');
                    console.log('  evidence <variant> - Query evidence');
                    console.log('  validate <hgvs> - Validate HGVS');
                    console.log('  quit - Exit');
                } else if (command.trim() === 'tools') {
                    const tools = await client.listTools();
                    tools.forEach(tool => {
                        console.log(`  ${tool.name}: ${tool.description || 'No description'}`);
                    });
                } else if (command.trim() === 'resources') {
                    const resources = await client.listResources();
                    resources.forEach(resource => {
                        console.log(`  ${resource.uri}: ${resource.name || 'No name'}`);
                    });
                } else if (command.trim() === 'prompts') {
                    const prompts = await client.listPrompts();
                    prompts.forEach(prompt => {
                        console.log(`  ${prompt.name}: ${prompt.description || 'No description'}`);
                    });
                } else if (command.startsWith('classify ')) {
                    const hgvs = command.substring(9).trim();
                    const result = await acmgClient.classifyVariant({ hgvs });
                    console.log(`Classification: ${result.classification}`);
                    console.log(`Confidence: ${(result.confidence || 0).toFixed(2)}`);
                } else if (command.startsWith('evidence ')) {
                    const variant = command.substring(9).trim();
                    const result = await acmgClient.queryEvidence(variant);
                    console.log(`Evidence gathered from: ${result.sources?.join(', ') || 'None'}`);
                } else if (command.startsWith('validate ')) {
                    const hgvs = command.substring(9).trim();
                    const result = await acmgClient.validateHgvs(hgvs);
                    console.log(`Valid: ${result.valid ? 'Yes' : 'No'}`);
                    if (result.normalized) {
                        console.log(`Normalized: ${result.normalized}`);
                    }
                } else if (command.trim()) {
                    console.log("Unknown command. Type 'help' for available commands.");
                }
            } catch (error) {
                console.error('Error:', error.message);
            }
        }

        rl.close();
    } finally {
        await client.disconnect();
    }
}

// CLI handling
if (require.main === module) {
    const args = process.argv.slice(2);
    
    if (args.includes('interactive')) {
        interactiveSession().catch(console.error);
    } else {
        exampleWorkflow().catch(console.error);
    }
}

module.exports = { MCPClient, ACMGAMPClient };