---
description: Initialize a new specification with detailed project description and requirements via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Spec Initialization

This template defines the `/kiro:spec-init` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize natural language requests that match these intents:
- "Initialize a new specification for [description]"
- "Create a new spec for [description]" 
- "Start a new feature specification: [description]"
- Any similar phrasing indicating intent to initialize a new SDD specification

The agent internally maps these to the `/kiro:spec-init` pattern but users should use natural language.

## Implementation Logic

When a user types `/kiro:spec-init "project description"` in `q chat --agent sdd`:

### 1. Parse Command and Description
- Extract the project description from the command
- Validate that description is provided

### 2. Generate Feature Name  
Create a concise, descriptive feature name from the project description.
**Check existing `.kiro/specs/` directory to ensure the generated feature name is unique. If a conflict exists, append a number suffix (e.g., feature-name-2).**

### 3. Create Spec Directory Structure
Create `.kiro/specs/{generated-feature-name}/` directory with template files:
- `requirements.md` - Template with user input
- `design.md` - Empty template for technical design  
- `tasks.md` - Empty template for implementation tasks
- `spec.json` - Metadata and approval tracking

### 4. Initialize spec.json Metadata
```json
{
  "feature_name": "{generated-feature-name}",
  "created_at": "current_timestamp",
  "updated_at": "current_timestamp", 
  "language": "english",
  "phase": "initialized",
  "approvals": {
    "requirements": {
      "generated": false,
      "approved": false
    },
    "design": {
      "generated": false,
      "approved": false
    },
    "tasks": {
      "generated": false,
      "approved": false
    }
  },
  "ready_for_implementation": false
}
```

## Agent Response Format

After successful initialization, the agent should respond with:

```
✅ **Specification Initialized Successfully**

📁 **Feature**: {generated-feature-name}
📝 **Description**: {brief-summary}
📂 **Created**: .kiro/specs/{generated-feature-name}/

**Files Created:**
- requirements.md (with your project description)
- design.md (empty template)
- tasks.md (empty template)  
- spec.json (workflow metadata)

**Next Step:**
Use `/kiro:spec-requirements {generated-feature-name}` to generate detailed requirements.

**Workflow:**
1. ✅ spec-init ← You are here
2. ⏳ spec-requirements 
3. ⏳ spec-design
4. ⏳ spec-tasks
5. ⏳ spec-impl
```

## Error Handling

- If no description provided: "Please provide a project description: `/kiro:spec-init "your project description"`"
- If .kiro directory doesn't exist: Create it automatically
- If feature name conflict: Append number suffix and notify user

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:spec-init` in Amazon Q CLI chat with `--agent sdd`.