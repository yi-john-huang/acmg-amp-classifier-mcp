---
description: Create custom steering documents via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Custom Steering

This template defines the `/kiro:steering-custom` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute this pattern:
```
/kiro:steering-custom <name>
```

## Implementation Logic

When a user types `/kiro:steering-custom name` in `q chat --agent sdd`:

### 1. Validate Input
- Check that name parameter is provided
- Sanitize name (convert to kebab-case, remove special characters)
- Ensure name doesn't conflict with core steering files (product, tech, structure)

### 2. Create Custom Steering Document
Generate specialized steering document based on the name:

#### Common Custom Steering Types:
- **security**: Security guidelines and practices
- **performance**: Performance standards and optimization
- **testing**: Testing strategies and quality assurance  
- **deployment**: Deployment and DevOps practices
- **accessibility**: Accessibility standards and guidelines
- **api**: API design and documentation standards
- **database**: Database design and data modeling
- **monitoring**: Observability and monitoring practices

#### Structure Template:
```markdown
# {Title} Steering Document

## Overview
{Purpose and scope of this steering document}

## Guidelines

### Guideline 1: {Area}
**Principle**: {Core principle}
**Implementation**: {How to apply this}
**Examples**: {Concrete examples}
**Validation**: {How to verify compliance}

### Guideline 2: {Another Area}
[... continue pattern ...]

## Standards and Requirements

### Standard 1: {Requirement Area}
- **Must Have**: {Non-negotiable requirements}
- **Should Have**: {Recommended practices}
- **Could Have**: {Optional enhancements}

## Tools and Resources

### Recommended Tools
- **{Tool Category}**: {Tool name} - {Purpose}
- **{Another Category}**: {Tool name} - {Purpose}

### Documentation and References
- {Reference 1}: {URL or location}
- {Reference 2}: {URL or location}

## Integration with SDD Workflow

### Requirements Phase
{How this steering applies to requirements generation}

### Design Phase  
{How this steering influences design decisions}

### Implementation Phase
{How this steering guides implementation}

## Compliance and Validation

### Checklist
- [ ] {Compliance item 1}
- [ ] {Compliance item 2}
- [ ] {Compliance item 3}

### Review Process
{How to review compliance with these guidelines}
```

### 3. Write Custom Steering File
- Create `.kiro/steering/{name}.md` file
- Write the generated content
- Update AMAZONQ.md to reference the new steering file (if present)

### 4. Integration Options
Provide options for how this custom steering should be used:
- **Always**: Include in every SDD workflow interaction
- **Conditional**: Include only for specific file patterns or contexts
- **Manual**: Reference manually with @{name}.md syntax

## Agent Response Format

```
✅ **Custom Steering Document Created**

📂 **Created**: .kiro/steering/{name}.md
🎯 **Type**: {Detected/inferred steering type}
📋 **Content**: {X} guidelines and standards

**What This Provides:**
- Specialized {area} guidance for your project
- Standards and requirements specific to {domain}
- Integration points with SDD workflow phases
- Compliance checklist and validation approach

**Integration Options:**
1. **Always Active**: Include in all SDD interactions
   - Add to AMAZONQ.md active steering list
   - Will influence all requirements, design, and implementation

2. **Conditional**: Activate for specific contexts
   - Triggered by file patterns (e.g., *.test.js for testing steering)
   - Applied to relevant workflow phases only

3. **Manual Reference**: Use when needed
   - Reference with @{name}.md in SDD commands
   - On-demand guidance for specific situations

**Next Steps:**
- Review and customize the steering document
- Decide on integration approach (always/conditional/manual)
- Use in your next SDD workflow with existing features

**Usage Examples:**
- `/kiro:spec-requirements feature-name` (if always active)
- `/kiro:spec-design feature-name @{name}` (manual reference)
```

## Error Handling

- If no name provided: "Please specify a name for the custom steering document. Example: `/kiro:steering-custom security`"
- If invalid name: "Invalid name '{name}'. Use alphanumeric characters and hyphens only."
- If file already exists: "Steering document '{name}.md' already exists. Use a different name or edit the existing file."
- If directory creation fails: "Could not create .kiro/steering/ directory. Check permissions."

## Common Custom Steering Templates

### Security Steering
Focus on: Authentication, authorization, data protection, security testing, compliance

### Performance Steering  
Focus on: Response time targets, scalability requirements, optimization strategies, monitoring

### Testing Steering
Focus on: Test coverage requirements, testing strategies, quality gates, automation

### API Steering
Focus on: REST/GraphQL standards, documentation, versioning, error handling

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:steering-custom` in Amazon Q CLI chat with `--agent sdd`.