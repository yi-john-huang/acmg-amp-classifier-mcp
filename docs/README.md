# ACMG/AMP MCP Server Documentation

Complete documentation for the ACMG/AMP Model Context Protocol (MCP) Server - a clinical-grade genetic variant classification system designed for AI agent integration.

## Quick Start

### For Clinicians
- **[User Guide](./user-guide.md)** - Complete guide for clinical users
- **[Clinical Workflows](../examples/ai-agents/workflows/README.md)** - Step-by-step clinical processes
- **[Claude Desktop Setup](../examples/ai-agents/README.md)** - Quick integration with Claude

### For Developers  
- **[API Documentation](./api-documentation.md)** - Complete MCP tools, resources, and prompts reference
- **[Integration Guide](../examples/ai-agents/custom-clients/README.md)** - Custom client development
- **[Deployment Guide](../scripts/deployment/README.md)** - Production deployment instructions

### For System Administrators
- **[Security & Compliance](./security-compliance.md)** - Clinical security requirements
- **[Maintenance Guide](./maintenance-troubleshooting.md)** - Operations and troubleshooting
- **[Validation Guide](../validation/README.md)** - System validation procedures

## What is the ACMG/AMP MCP Server?

The ACMG/AMP MCP Server is a specialized implementation of the Model Context Protocol that provides AI agents with access to clinical-grade genetic variant classification capabilities. It transforms complex ACMG/AMP guidelines into AI-accessible tools, enabling natural language interactions with sophisticated genetic analysis.

### Key Features

**🧬 Clinical-Grade Classification**
- Full ACMG/AMP 2015 guidelines implementation
- Integration with ClinVar, gnomAD, COSMIC databases
- Evidence aggregation and confidence scoring
- Automated report generation

**🤖 AI Agent Integration**
- Native MCP protocol support
- Compatible with Claude Desktop, ChatGPT, and other AI systems
- Natural language variant interpretation
- Real-time classification results

**🏥 Production Ready**
- HIPAA-compliant security measures
- Comprehensive audit logging
- High availability deployment options
- Clinical validation test suites

**⚡ High Performance**
- Sub-2-second classification response times
- Intelligent caching and optimization
- Concurrent client support
- Scalable architecture

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        AI Agents                               │
├─────────────────┬─────────────────┬─────────────────┬──────────┤
│ Claude Desktop  │     ChatGPT     │     Gemini      │  Custom  │
│                 │  (via bridge)   │  (via bridge)   │ Clients  │
└─────────────────┼─────────────────┼─────────────────┼──────────┘
                  │                 │                 │
                  └─────────────────┼─────────────────┘
                                    │
                  ┌─────────────────┼─────────────────┐
                  │                 │                 │
              ┌───▼────┐        ┌───▼────┐        ┌───▼────┐
              │ stdio  │        │  HTTP  │        │   WS   │
              │Transport│        │Transport│        │Transport│
              └───┬────┘        └───┬────┘        └───┬────┘
                  │                 │                 │
                  └─────────────────┼─────────────────┘
                                    │
                ┌───────────────────▼───────────────────┐
                │         MCP Server Core               │
                │  ┌─────────────────────────────────┐  │
                │  │        JSON-RPC 2.0 Engine     │  │
                │  └─────────────────────────────────┘  │
                │  ┌─────────────────────────────────┐  │
                │  │      ACMG/AMP Tools             │  │
                │  │  • classify_variant             │  │
                │  │  • validate_hgvs               │  │
                │  │  • query_evidence              │  │
                │  │  • generate_report             │  │
                │  └─────────────────────────────────┘  │
                │  ┌─────────────────────────────────┐  │
                │  │      MCP Resources              │  │
                │  │  • variant/{id}                │  │
                │  │  • interpretation/{id}         │  │
                │  │  • evidence/{variant_id}       │  │
                │  │  • acmg/rules                  │  │
                │  └─────────────────────────────────┘  │
                │  ┌─────────────────────────────────┐  │
                │  │      MCP Prompts                │  │
                │  │  • clinical_interpretation     │  │
                │  │  • evidence_review             │  │
                │  │  • report_generation           │  │
                │  │  • acmg_training               │  │
                │  └─────────────────────────────────┘  │
                └───────────────┬───────────────────────┘
                                │
                ┌───────────────▼───────────────────┐
                │        Data Layer                │
                │  ┌─────────────────────────────┐  │
                │  │      PostgreSQL             │  │
                │  │   • Variants & Evidence     │  │
                │  │   • Classifications         │  │
                │  │   • Audit Logs              │  │
                │  └─────────────────────────────┘  │
                │  ┌─────────────────────────────┐  │
                │  │         Redis               │  │
                │  │   • Response Cache          │  │
                │  │   • Session Data            │  │
                │  └─────────────────────────────┘  │
                └───────────────┬───────────────────┘
                                │
                ┌───────────────▼───────────────────┐
                │      External APIs                │
                │  ┌─────────────────────────────┐  │
                │  │ ClinVar  │ gnomAD │ COSMIC │  │
                │  └─────────────────────────────┘  │
                └───────────────────────────────────┘
```

## MCP Capabilities

### Tools (Active Functions)
- **classify_variant** - Complete ACMG/AMP classification workflow
- **validate_hgvs** - HGVS notation validation and normalization
- **apply_rule** - Individual ACMG/AMP criteria evaluation
- **combine_evidence** - Evidence combination per guidelines
- **query_evidence** - Multi-database evidence gathering
- **query_clinvar** - ClinVar-specific queries
- **query_gnomad** - gnomAD population frequency data
- **query_cosmic** - COSMIC cancer mutation database
- **generate_report** - Clinical report generation
- **format_report** - Multi-format report output

### Resources (Data Access)
- **variant/{id}** - Detailed variant information
- **interpretation/{id}** - Classification results
- **evidence/{variant_id}** - Aggregated evidence data
- **acmg/rules** - Complete ACMG/AMP criteria definitions

### Prompts (AI Guidance)
- **clinical_interpretation** - Systematic workflow guidance
- **evidence_review** - Structured evidence evaluation
- **report_generation** - Clinical report customization
- **acmg_training** - Educational guideline learning

## Installation & Setup

### Quick Start (Docker)

```bash
# Clone repository
git clone https://github.com/your-org/acmg-amp-classifier-mcp.git
cd acmg-amp-classifier-mcp

# Start with Docker Compose
docker-compose up -d

# Verify server is running
curl http://localhost:8080/health
```

### Production Deployment

```bash
# Kubernetes deployment
kubectl apply -f deployments/kubernetes/

# Or using deployment script
./scripts/deployment/deploy-k8s.sh deploy --replicas 3 --image v1.0.0
```

### Claude Desktop Integration

1. Add to Claude Desktop config file:
```json
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "/path/to/mcp-server",
      "args": ["--config", "/path/to/config.yaml"]
    }
  }
}
```

2. Restart Claude Desktop

3. Test integration:
```
"Please classify the CFTR variant NM_000492.3:c.1521_1523delCTT"
```

## Usage Examples

### Basic Variant Classification
```
Human: "I need to classify the variant NM_000492.3:c.1521_1523delCTT found in a patient with cystic fibrosis symptoms."

AI: "I'll classify this CFTR variant using ACMG/AMP guidelines..."

[Tool: classify_variant]
Classification: Pathogenic (Confidence: 98%)
Applied Criteria: PVS1, PS3, PM2, PP3, PP5
Clinical Significance: Confirms cystic fibrosis diagnosis
```

### Evidence Gathering
```
Human: "What evidence is available for the BRCA1 variant c.185delA?"

AI: "Let me gather evidence from multiple databases..."

[Tool: query_evidence]  
ClinVar: Pathogenic (4-star rating, 47 submissions)
gnomAD: Absent in all populations (0/282,596 alleles)
Literature: 150+ publications confirming pathogenicity
```

### Clinical Report Generation
```
Human: "Generate a clinical report for this classification."

AI: "I'll create a comprehensive clinical report..."

[Tool: generate_report]
# Complete clinical report with:
- Executive summary
- Classification rationale  
- Treatment implications
- Family counseling recommendations
```

## Validation & Quality Assurance

### Protocol Compliance
```bash
# Validate MCP protocol compliance
python3 validation/mcp-protocol-compliance.py ./bin/mcp-server
```

### Clinical Accuracy
```bash
# Validate against reference datasets
python3 validation/clinical-accuracy-validation.py test_cases.json
```

### Performance Testing
```bash  
# Load testing with multiple concurrent clients
python3 tests/load-testing.py --clients 10 --duration 300
```

## Security & Compliance

### Clinical Grade Security
- ✅ HIPAA compliance measures
- ✅ End-to-end encryption
- ✅ Comprehensive audit logging
- ✅ Access control and authentication
- ✅ Data integrity validation

### Regulatory Compliance
- ✅ ACMG/AMP 2015 guidelines adherence
- ✅ CAP/CLIA laboratory standards compatibility
- ✅ FDA guidance consideration
- ✅ International classification harmonization

## Documentation Structure

```
docs/
├── README.md                    # This file - overview and quick start
├── user-guide.md               # Complete user guide for clinicians
├── api-documentation.md        # Full MCP API reference
├── security-compliance.md      # Security and regulatory compliance
├── maintenance-troubleshooting.md # Operations and troubleshooting
├── deployment/                 # Deployment guides
│   ├── docker-deployment.md    # Docker setup guide
│   ├── kubernetes-deployment.md # Kubernetes deployment
│   └── cloud-deployment.md     # Cloud provider guides
├── development/                # Developer documentation
│   ├── contributing.md         # Contribution guidelines
│   ├── testing.md             # Testing procedures
│   └── architecture.md        # Technical architecture
└── tutorials/                  # Step-by-step tutorials
    ├── first-classification.md # Your first variant classification
    ├── batch-processing.md     # Bulk variant analysis
    └── custom-integration.md   # Building custom integrations
```

## Community & Support

### Getting Help
- 📖 **Documentation**: Comprehensive guides and references
- 💬 **Community Forum**: User discussions and Q&A
- 🐛 **Issue Tracker**: Bug reports and feature requests
- 📧 **Professional Support**: Enterprise support options

### Contributing
- 🔧 **Code Contributions**: Pull requests welcome
- 📝 **Documentation**: Help improve guides and tutorials
- 🧪 **Testing**: Validate with clinical datasets
- 💡 **Feature Requests**: Suggest improvements

### Professional Services
- 🏥 **Clinical Implementation**: Expert deployment assistance
- 🎓 **Training Programs**: User and administrator training
- 🔒 **Security Auditing**: Compliance validation services
- 🚀 **Custom Development**: Specialized integrations

## License & Legal

This project is licensed under the MIT License. See [LICENSE](../LICENSE) for details.

### Clinical Use Disclaimer
This software is provided for research and educational purposes. While designed to clinical standards, users are responsible for validation in their specific clinical environment and compliance with applicable regulations.

### Third-Party Data Sources
- ClinVar: Public domain (NCBI)
- gnomAD: Available under Fort Lauderdale Agreement
- COSMIC: Academic license required for commercial use

---

## Next Steps

1. **[Quick Setup](../examples/ai-agents/README.md)** - Get started in 5 minutes
2. **[User Guide](./user-guide.md)** - Learn clinical workflows  
3. **[API Reference](./api-documentation.md)** - Explore all capabilities
4. **[Security Guide](./security-compliance.md)** - Ensure compliance
5. **[Troubleshooting](./maintenance-troubleshooting.md)** - Resolve issues

For questions or support, please refer to our [support channels](#community--support) or contact the development team.