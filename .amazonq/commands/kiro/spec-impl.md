---
description: Provide implementation guidance via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Implementation Guidance

This template defines the `/kiro:spec-impl` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute these patterns:
```
/kiro:spec-impl <feature-name>
/kiro:spec-impl <feature-name> <task-numbers>
```

## Implementation Logic

When a user types `/kiro:spec-impl feature-name [task-numbers]` in `q chat --agent sdd`:

### 1. Validate Prerequisites
- Check that `.kiro/specs/{feature-name}/` directory exists
- Verify tasks.md exists and contains task breakdown
- Check spec.json shows "ready_for_implementation": true

### 2. Load Implementation Context
- Read requirements.md for functional context
- Read design.md for technical architecture
- Read tasks.md for implementation plan
- Check steering files for project guidelines

### 3. Provide Implementation Guidance

If no specific tasks specified:
```
📋 **Implementation Guidance for {feature-name}**

**Available Tasks:**
1. {Task 1 summary}
2. {Task 2 summary}  
3. {Task 3 summary}
...

**Recommended Starting Point:**
Task 1: {First task description}

**How to proceed:**
- Use `/kiro:spec-impl {feature-name} 1` for specific task guidance
- Use `/kiro:spec-impl {feature-name} 1,3,5` for multiple tasks
- Use `/kiro:spec-status {feature-name}` to track progress

**Implementation Context:**
- Architecture: {Brief architecture summary}
- Key Requirements: {Top 3 requirements}
- Technical Stack: {From design document}
```

If specific tasks specified:
```
🛠️ **Implementation Guidance: Tasks {task-numbers}**

**Task {N}: {Task Title}**

**Context from Requirements:**
{Relevant requirements that this task addresses}

**Context from Design:**  
{Relevant design components and architecture}

**Implementation Approach:**
1. {Step-by-step implementation guidance}
2. {Code examples or pseudocode if helpful}
3. {Integration points and dependencies}

**Testing Approach:**
- {Unit testing guidance}
- {Integration testing notes}
- {Acceptance criteria validation}

**Completion Criteria:**
- [ ] {Specific deliverable 1}
- [ ] {Specific deliverable 2}
- [ ] {Testing completed}
- [ ] {Documentation updated}

**Next Steps:**
After completing this task, proceed to Task {N+1}: {Next task title}
```

### 4. Track Progress (Optional)
Update spec.json to track implementation progress:
```json
{
  "implementation": {
    "status": "in-progress",
    "started_at": "current_timestamp",
    "completed_tasks": [1, 3],
    "current_task": 4
  }
}
```

## Agent Response Format

```
🛠️ **Implementation Ready**

📁 **Feature**: {feature-name}
📋 **Total Tasks**: {X}
🎯 **Focus**: {Specific tasks or general overview}

**Implementation Context Loaded:**
- ✅ Requirements: {X} functional requirements
- ✅ Design: {Y} components and {Z} APIs
- ✅ Tasks: {W} implementation tasks

**Ready to Code!**
Follow the implementation guidance above and reference the specification documents as needed.

**Status Tracking:**
Use `/kiro:spec-status {feature-name}` to update progress and track completion.
```

## Error Handling

- If feature doesn't exist: "Feature '{feature-name}' not found."
- If not ready for implementation: "Feature not ready. Complete tasks generation first with `/kiro:spec-tasks {feature-name}`"
- If invalid task numbers: "Invalid task numbers. Use `/kiro:spec-status {feature-name}` to see available tasks."

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:spec-impl` in Amazon Q CLI chat with `--agent sdd`.