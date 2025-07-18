---
inclusion: always
---

# Metadata Generation and Step Tracking Rule

## Purpose
Generate and maintain metadata to record the latest execution step, enabling recovery from unexpected errors and providing execution context for debugging and continuation.

## Implementation Requirements

### Metadata File Location
- Create/update `.kiro/metadata/execution-state.json` for each major operation
- Include timestamp, current step, task status, and context information

### Metadata Structure
```json
{
  "timestamp": "2025-01-16T10:30:00Z",
  "current_step": "task_1_implementation",
  "task_details": {
    "task_id": "1",
    "task_name": "Set up project structure and core interfaces",
    "status": "in_progress",
    "sub_steps_completed": ["interfaces_defined", "models_created"],
    "current_sub_step": "configuration_setup"
  },
  "context": {
    "spec_name": "acmg-amp-mcp-server",
    "phase": "implementation",
    "files_modified": ["internal/domain/interfaces.go", "internal/config/config.go"],
    "last_successful_operation": "interface_validation"
  },
  "error_recovery": {
    "last_checkpoint": "configuration_validated",
    "rollback_point": "task_1_start",
    "recovery_instructions": "Resume from configuration validation step"
  }
}
```

### When to Generate Metadata
1. **Task Start**: Record task initiation and setup
2. **Major Milestones**: After completing significant sub-steps
3. **Before Risky Operations**: Before file modifications, external API calls, or complex operations
4. **Error Conditions**: When catching errors or exceptions
5. **Task Completion**: Final status and summary

### Metadata Content Requirements
- **Timestamp**: ISO 8601 format for precise timing
- **Current Step**: Human-readable description of current operation
- **Task Context**: Task ID, name, status, and progress indicators
- **File State**: List of files being modified or created
- **Recovery Information**: Checkpoint data for resuming operations
- **Error Context**: If applicable, error details and recovery suggestions

### Implementation Pattern
```go
// Example metadata generation function
func updateExecutionMetadata(step string, context map[string]interface{}) error {
    metadata := ExecutionMetadata{
        Timestamp: time.Now().UTC(),
        CurrentStep: step,
        Context: context,
        // ... other fields
    }
    
    return writeMetadataFile(".kiro/metadata/execution-state.json", metadata)
}
```

### Usage Guidelines
1. **Call at Step Boundaries**: Update metadata when transitioning between major steps
2. **Include Sufficient Context**: Provide enough information to resume from any point
3. **Handle Metadata Errors Gracefully**: Don't fail main operations due to metadata issues
4. **Clean Up on Success**: Archive or clean metadata after successful completion
5. **Preserve on Failure**: Keep metadata intact for debugging and recovery

### Recovery Process
1. **Check for Existing Metadata**: Look for execution state file on startup
2. **Validate State**: Ensure metadata is consistent with current file system state
3. **Offer Recovery Options**: Present user with options to resume, restart, or rollback
4. **Resume Execution**: Continue from last known good checkpoint

This rule ensures robust execution tracking and enables reliable recovery from unexpected interruptions during complex multi-step operations.