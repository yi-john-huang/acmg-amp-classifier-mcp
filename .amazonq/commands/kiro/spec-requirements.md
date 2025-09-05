---
description: Generate detailed requirements document for a feature via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Requirements Generation

This template defines the `/kiro:spec-requirements` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute this pattern:
```
/kiro:spec-requirements <feature-name>
```

## Implementation Logic

When a user types `/kiro:spec-requirements feature-name` in `q chat --agent sdd`:

### 1. Validate Feature Exists
- Check that `.kiro/specs/{feature-name}/` directory exists
- Verify spec.json file is present
- Read current project description from requirements.md

### 2. Read Project Context
- Load existing requirements.md for user's project description
- Read `.kiro/steering/` files if they exist for project context
- Check current phase in spec.json

### 3. Generate Comprehensive Requirements in EARS Format
Create detailed requirements document using EARS (Easy Approach to Requirements Syntax) with:

#### Structure Template (EARS Format):
```markdown
# Requirements Document - EARS Format

## Introduction
{AI-generated introduction based on project description}

## Functional Requirements

### REQ-001: {Functional Area}
WHEN {trigger condition}, the system SHALL {required behavior}.

### REQ-002: {Another Functional Area}
IF {conditional state} THEN the system SHALL {consequent action}.

### REQ-003: {Ongoing Behavior}
WHILE {continuous condition}, the system SHALL {sustained behavior}.

### REQ-004: {Location/Scope Constraint}
WHERE {location/scope condition}, the system SHALL {scoped behavior}.

### REQ-005: {Complex Conditional}
IF {condition} THEN the system SHALL {primary action}, OTHERWISE the system SHALL {alternative action}.

[Continue with REQ-006, REQ-007, etc. using appropriate EARS keywords]

## Non-Functional Requirements

### REQ-NFR-001: Performance
The system SHALL respond to user requests within {X} seconds.

### REQ-NFR-002: Throughput  
The system SHALL support {Y} concurrent users.

### REQ-NFR-003: Security
The system SHALL encrypt all data transmission using TLS 1.3 or higher.

### REQ-NFR-004: Availability
The system SHALL maintain {Z}% uptime during business hours.

### REQ-NFR-005: Usability
The system SHALL require no more than {N} clicks to complete primary user tasks.

## Constraints

### REQ-CON-001: Technology
The system SHALL use {specified technology stack}.

### REQ-CON-002: Compliance
The system SHALL comply with {relevant regulations}.

## Assumptions

### REQ-ASM-001: User Environment
Users SHALL have {specified environment/access}.

### REQ-ASM-002: Data Availability
Required data SHALL be accessible via {specified means}.

## Success Metrics
- Key performance indicators with measurable targets
- Acceptance criteria for each requirement category  
- Testing and validation approach with specific metrics
```

### 4. Update Workflow State
Update spec.json:
```json
{
  "phase": "requirements-generated",
  "updated_at": "current_timestamp",
  "approvals": {
    "requirements": {
      "generated": true,
      "approved": false
    }
  }
}
```

## Agent Response Format

After successful generation:

```
✅ **Requirements Generated Successfully**

📁 **Feature**: {feature-name}
📝 **Generated**: Comprehensive requirements document
📂 **Updated**: .kiro/specs/{feature-name}/requirements.md

**What Was Generated:**
- {X} functional requirements with user stories
- {Y} acceptance criteria
- Non-functional requirements (performance, security, usability)
- Success metrics and validation approach

**⚠️ Review Required:**
Please review the requirements.md file carefully before proceeding.

**Next Step:**
After reviewing, use `/kiro:spec-design {feature-name}` to generate technical design.

**Workflow:**
1. ✅ spec-init
2. ✅ spec-requirements ← You are here  
3. ⏳ spec-design (requires requirements approval)
4. ⏳ spec-tasks  
5. ⏳ spec-impl
```

## Approval Gate

The agent should enforce that design cannot proceed until requirements are approved:
- Check spec.json approval status before allowing spec-design
- Prompt user to review requirements.md before proceeding

## Error Handling

- If feature doesn't exist: "Feature '{feature-name}' not found. Use `/kiro:spec-init` to create it first."
- If already generated: "Requirements already generated. Use `/kiro:spec-status {feature-name}` to check status."
- If no project description: "No project description found. Please check requirements.md file."

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:spec-requirements` in Amazon Q CLI chat with `--agent sdd`.