---
description: Show workflow progress and next steps via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Status Check

This template defines the `/kiro:spec-status` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute this pattern:
```
/kiro:spec-status <feature-name>
```

## Implementation Logic

When a user types `/kiro:spec-status feature-name` in `q chat --agent sdd`:

### 1. Validate Feature Exists
- Check that `.kiro/specs/{feature-name}/` directory exists
- Verify spec.json file is present and readable

### 2. Read Current Status
- Load spec.json for workflow state
- Check file existence and modification dates
- Analyze task completion if in implementation phase

### 3. Generate Status Report

The agent should display comprehensive status with:
- Current workflow phase and progress
- File status and modification dates
- Next recommended actions
- Implementation progress if applicable

## Agent Response Format

Standard status response with appropriate icons:
- ✅ Completed
- 🔄 In Progress  
- ⏳ Pending
- ⚠️ Needs Attention
- ❌ Blocked/Error

## Error Handling

- If feature doesn't exist: "Feature '{feature-name}' not found. Available features: {list_existing_features}"
- If spec.json corrupted: "Workflow metadata corrupted. Please check .kiro/specs/{feature-name}/spec.json"
- If files missing: "Required files missing. Expected: requirements.md, design.md, tasks.md"

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:spec-status` in Amazon Q CLI chat with `--agent sdd`.