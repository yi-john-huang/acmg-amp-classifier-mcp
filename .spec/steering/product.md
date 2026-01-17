# Product Context - ACMG-AMP MCP Server

## Overview

The ACMG-AMP MCP Server is a **Model Context Protocol (MCP)** compliant service that provides AI agents with direct access to professional-grade genetic variant classification tools. It implements the complete **ACMG/AMP 2015 guidelines** with all 28 evidence criteria, enabling AI assistants to perform standardized variant interpretation through natural language interactions.

## Vision

Bridge clinical genetics and AI agents with standardized ACMG/AMP guidelines, making professional-grade variant interpretation accessible through natural language interactions while maintaining clinical accuracy and reproducibility.

## Target Users

### Primary Users
- **Clinical Geneticists**: Professionals interpreting genetic variants for patient care
- **Molecular Pathologists**: Specialists analyzing genetic test results
- **Genetic Counselors**: Healthcare providers explaining genetic findings to patients
- **Medical Researchers**: Scientists studying genetic variants and their clinical significance

### AI Agent Users
- **Claude**: Anthropic's AI assistant via MCP integration
- **ChatGPT**: OpenAI's assistant with MCP compatibility
- **Gemini**: Google's AI assistant
- **Custom MCP Clients**: Third-party AI applications using MCP protocol

## Core Features

### ACMG/AMP Classification Engine
- **All 28 ACMG/AMP Rules**: Complete implementation of PVS1, PS1-PS4, PM1-PM6, PP1-PP5, BA1, BS1-BS4, BP1-BP7
- **Evidence Combination Logic**: Full 2015 ACMG/AMP guidelines for classification determination
- **HGVS Parser**: Medical-grade variant notation validation and normalization
- **Gene Symbol Support**: Input variants using gene symbols (e.g., "BRCA1:c.123A>G") with automatic transcript resolution

### External Evidence Integration
- **ClinVar**: Clinical significance and expert-reviewed classifications
- **gnomAD**: Population frequency data across ancestries
- **COSMIC**: Somatic mutation data for cancer variants
- **PubMed**: Literature references and research evidence
- **LOVD**: Locus-specific database annotations
- **HGMD**: Human Gene Mutation Database (professional)

### Gene Database APIs
- **HGNC**: Official gene symbol validation
- **RefSeq**: Transcript reference sequences
- **Ensembl**: Gene and transcript annotations

### MCP Protocol
- **Native MCP Integration**: Direct tool access via JSON-RPC 2.0
- **Transport Options**: Stdio and HTTP-SSE transport layers
- **Tool Registry**: All ACMG/AMP tools registered and accessible
- **Session Management**: Client tracking, rate limiting, graceful shutdown

## Key Value Propositions

1. **Professional-Grade Interpretation**: Clinical-quality variant classification following established guidelines
2. **AI-Native Design**: Built specifically for AI agent integration via MCP
3. **Standardized Results**: Consistent ACMG/AMP classification across all analyses
4. **Evidence-Based**: Automated evidence gathering from 6 major databases
5. **Reproducible**: Same variant always yields same classification with same evidence

## Success Metrics

### Classification Quality
- **Accuracy**: Concordance with expert-reviewed classifications in ClinVar
- **Completeness**: Percentage of applicable rules evaluated per variant
- **Evidence Coverage**: Number of external sources queried per classification

### Performance
- **Response Latency**: Time from request to complete classification
- **Cache Hit Rate**: Efficiency of external API caching
- **Availability**: Service uptime and reliability

### Adoption
- **API Usage**: Number of classification requests
- **AI Agent Integration**: Number of unique AI clients using the service
- **Tool Utilization**: Distribution of tool usage across available MCP tools

## License and Usage Restrictions

### Permitted Uses (Non-Commercial)
- Academic research and education
- Personal experimentation and learning
- Non-profit organization internal research
- Open source contributions
- Clinical research (non-patient care)

### Prohibited Uses
- Clinical practice and patient care (requires regulatory approval)
- Commercial products or services
- Revenue-generating operations

### Important Disclaimer
This software is for **research and educational purposes only**. It is NOT approved for clinical use or patient care, and requires additional validation and regulatory approval for clinical settings.
