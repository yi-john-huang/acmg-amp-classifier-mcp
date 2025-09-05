---
description: Create/update project steering documents via Amazon Q CLI Custom Agent
allowed-tools: fs_read, fs_write
model: amazon-q-developer
agent: sdd
---

# Amazon Q CLI Custom Agent: Project Steering

This template defines the `/kiro:steering` command behavior for the SDD Custom Agent in Amazon Q CLI.

## Agent Command Recognition

The SDD Custom Agent should recognize and execute this pattern:
```
/kiro:steering
```

## Implementation Logic

When a user types `/kiro:steering` in `q chat --agent sdd`:

### 1. Create Steering Directory Structure
- Create `.kiro/steering/` directory if it doesn't exist
- Check for existing steering files

### 2. Generate Core Steering Documents

The agent should create three core documents:
- `product.md` - Business objectives and user context
- `tech.md` - Technology stack and architectural decisions  
- `structure.md` - File organization and coding patterns

## Agent Response Format

After successful generation:

```
✅ **Project Steering Documents Created**

📂 **Created/Updated Files:**
- .kiro/steering/product.md - Business objectives and user personas
- .kiro/steering/tech.md - Technology stack and architectural decisions  
- .kiro/steering/structure.md - File organization and coding patterns

**What These Provide:**
- **Product Context**: Clear business objectives and user requirements
- **Technical Guidance**: Technology choices and development standards
- **Organization Rules**: Consistent file structure and naming conventions

**Next Steps:**
- Review and customize the steering documents for your project
- Use `/kiro:spec-init "feature description"` to start your first specification
```

## Integration Notes

This template is embedded in the SDD Custom Agent configuration and executed when users type `/kiro:steering` in Amazon Q CLI chat with `--agent sdd`.