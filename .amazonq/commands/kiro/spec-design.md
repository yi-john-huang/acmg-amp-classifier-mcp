---
description: Generate technical design document via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Technical Design

This template defines the `/kiro:spec-design` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute this pattern:
```
/kiro:spec-design <feature-name>
```

## Implementation Logic

When a user types `/kiro:spec-design feature-name` in `q chat --agent sdd`:

### 1. Validate Prerequisites
- Check that `.kiro/specs/{feature-name}/` directory exists
- Verify requirements.md exists and is complete
- Check spec.json approval status for requirements

### 2. Interactive Approval Check
If requirements not marked as approved in spec.json:
```
🔍 **Requirements Review Check**

Have you reviewed and approved the requirements.md file? [y/N]

**Why this matters:**
- Design decisions should be based on approved requirements
- Changes to requirements after design may require redesign
- This ensures proper workflow progression

**To review:** Open .kiro/specs/{feature-name}/requirements.md

Type 'y' to confirm you've reviewed requirements, or 'N' to cancel.
```

### 3. Generate Technical Design
Create comprehensive design document with:

#### Structure Template:
```markdown
# Technical Design

## Overview
{AI-generated overview based on requirements}

## Requirements Mapping
{Map each requirement to design components}

## Architecture
{High-level architecture diagram and description}

## Components and Interfaces
{Detailed component design}

## Data Models
{Database schemas, data structures}

## API Specifications
{REST endpoints, GraphQL schemas, etc.}

## Security Considerations
{Security implementation details}

## Performance & Scalability
{Performance targets and scaling approach}

## Testing Strategy
{Unit, integration, e2e test approach}
```

### 4. Update Workflow State
Update spec.json after successful generation:
```json
{
  "phase": "design-generated",
  "updated_at": "current_timestamp",
  "approvals": {
    "requirements": {
      "generated": true,
      "approved": true
    },
    "design": {
      "generated": true,
      "approved": false
    }
  }
}
```

## Agent Response Format

After successful generation:

```
✅ **Technical Design Generated Successfully**

📁 **Feature**: {feature-name}
📝 **Generated**: Comprehensive technical design
📂 **Updated**: .kiro/specs/{feature-name}/design.md

**Design Components:**
- Architecture overview and component breakdown
- Data models and API specifications
- Security and performance considerations
- Testing strategy and approach

**⚠️ Review Required:**
Please review the design.md file carefully before proceeding.

**Next Step:**
After reviewing, use `/kiro:spec-tasks {feature-name}` to generate implementation tasks.

**Workflow:**
1. ✅ spec-init
2. ✅ spec-requirements (approved)
3. ✅ spec-design ← You are here
4. ⏳ spec-tasks (requires design approval)
5. ⏳ spec-impl
```

## Error Handling

- If feature doesn't exist: "Feature '{feature-name}' not found. Use `/kiro:spec-init` to create it first."
- If requirements not approved: Show interactive approval check
- If design already exists: "Design already generated. Use `/kiro:spec-status {feature-name}` to check status."

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:spec-design` in Amazon Q CLI chat with `--agent sdd`.