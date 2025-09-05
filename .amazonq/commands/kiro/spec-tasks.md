---
description: Generate implementation task breakdown via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Task Generation

This template defines the `/kiro:spec-tasks` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute this pattern:
```
/kiro:spec-tasks <feature-name>
```

## Implementation Logic

When a user types `/kiro:spec-tasks feature-name` in `q chat --agent sdd`:

### 1. Validate Prerequisites
- Check that `.kiro/specs/{feature-name}/` directory exists
- Verify design.md exists and is complete
- Check spec.json approval status for both requirements and design

### 2. Interactive Approval Check
If design not marked as approved:
```
🔍 **Design Review Check**

Have you reviewed and approved both requirements.md AND design.md? [y/N]

**Required approvals:**
- ✅ Requirements reviewed and approved
- ⏳ Design reviewed and approved ← Missing

**Why this matters:**
- Tasks should implement approved design components
- Changes to design after task breakdown may require re-planning
- Ensures proper workflow progression

**To review:** 
- Requirements: .kiro/specs/{feature-name}/requirements.md
- Design: .kiro/specs/{feature-name}/design.md

Type 'y' to confirm you've reviewed both, or 'N' to cancel.
```

### 3. Generate Task Breakdown
Create detailed implementation plan with:

#### Structure Template:
```markdown
# Implementation Plan

## Foundation Tasks

- [ ] 1. {Foundation task}
  - {Sub-task description}
  - {Another sub-task}
  - _Requirements: {Reference to requirements}_

## Core Implementation

- [ ] 2. {Core feature task}
  - {Implementation details}
  - {Testing requirements}
  - _Requirements: {Requirement mapping}_

## Integration & Testing

- [ ] 3. {Integration task}
  - {Integration points}
  - {Testing approach}
  - _Requirements: {Coverage mapping}_

## Deployment & Documentation

- [ ] 4. {Deployment task}
  - {Deployment steps}
  - {Documentation needs}
  - _Requirements: {Final validations}_
```

### 4. Update Workflow State
Update spec.json:
```json
{
  "phase": "tasks-generated",
  "updated_at": "current_timestamp",
  "approvals": {
    "requirements": {
      "generated": true,
      "approved": true
    },
    "design": {
      "generated": true,
      "approved": true
    },
    "tasks": {
      "generated": true,
      "approved": false
    }
  },
  "ready_for_implementation": true
}
```

## Agent Response Format

```
✅ **Implementation Tasks Generated Successfully**

📁 **Feature**: {feature-name}
📝 **Generated**: Detailed task breakdown
📂 **Updated**: .kiro/specs/{feature-name}/tasks.md

**Task Summary:**
- {X} foundation and setup tasks
- {Y} core implementation tasks
- {Z} integration and testing tasks
- All tasks mapped to requirements and design components

**⚠️ Review Required:**
Please review the tasks.md file before starting implementation.

**Next Step:**
Use `/kiro:spec-impl {feature-name}` to get implementation guidance.

**Workflow:**
1. ✅ spec-init
2. ✅ spec-requirements (approved)
3. ✅ spec-design (approved)
4. ✅ spec-tasks ← You are here
5. ⏳ spec-impl (ready to start!)
```

## Error Handling

- If feature doesn't exist: "Feature '{feature-name}' not found."
- If design not approved: Show interactive approval check
- If tasks already exist: "Tasks already generated. Use `/kiro:spec-status {feature-name}` to check status."

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:spec-tasks` in Amazon Q CLI chat with `--agent sdd`.