# Amazon Q CLI SDD Custom Agent

This directory contains templates and configuration for the SDD (Spec-Driven Development) Custom Agent for Amazon Q CLI.

## Installation

```bash
# Install the SDD agent
npx amazonq-sdd

# Start using the agent
q chat --agent sdd

# Try your first command  
"Initialize a new specification for user authentication system"
```

## Agent Configuration

The SDD Custom Agent is configured with:
- **Name**: `sdd`
- **Tools**: All available tools (`*`)
- **Access Level**: Unrestricted system access
- **Languages**: JavaScript, Java, Go, Python, and all supported languages
- **Command Prefix**: `/kiro:`

## Available Commands

The SDD agent recognizes natural language requests for these SDD workflow actions:

| Intent | Example Usage | Description |
|--------|---------------|-------------|
| **Spec Initialization** | "Initialize a new specification for [description]" | Creates new feature specification directory and files |
| **Requirements Generation** | "Generate requirements for [feature-name]" | Creates detailed requirements document |
| **Design Creation** | "Create technical design for [feature-name]" | Generates technical design document |
| **Task Breakdown** | "Break down tasks for [feature-name]" | Creates implementation task list |
| **Implementation Guidance** | "Help me implement [feature-name]" | Provides implementation guidance |
| **Status Check** | "Show status of [feature-name]" | Displays workflow progress |
| **Project Steering** | "Set up project steering documents" | Creates project context and guidelines |
| **Custom Steering** | "Create custom steering for [area]" | Generates specialized steering documents |

## Workflow Phases

1. **Initialization** → "Initialize a new specification for [description]"
2. **Requirements** → "Generate requirements for [feature-name]" + review
3. **Design** → "Create technical design for [feature-name]" + review  
4. **Tasks** → "Break down tasks for [feature-name]" + review
5. **Implementation** → "Help me implement [feature-name]"

## File Structure

```
.kiro/
├── steering/              # Project guidelines
│   ├── product.md        # Business context
│   ├── tech.md           # Technology decisions
│   ├── structure.md      # Code organization
│   └── linus-review.md   # Linus Torvalds code review philosophy
└── specs/                # Feature specifications
    └── feature-name/
        ├── requirements.md
        ├── design.md
        ├── tasks.md
        └── spec.json
```

## Command Templates

Command behavior is defined in:
- `commands/kiro/spec-init.md`
- `commands/kiro/spec-requirements.md`
- `commands/kiro/spec-design.md`
- `commands/kiro/spec-tasks.md`
- `commands/kiro/spec-status.md`
- `commands/kiro/steering.md`

## Security Model

The SDD agent operates with unrestricted system access:
- **File Access**: Full file system read/write/delete access
- **Command Execution**: Any shell command or system operation
- **Network Access**: HTTP requests, API interactions, web scraping
- **Tool Access**: All Amazon Q CLI tools and capabilities without restrictions

## Integration Notes

This Custom Agent integrates with Amazon Q CLI's native capabilities:
- Uses all available Amazon Q CLI tools without restrictions (`tools: "*"`)
- Leverages Amazon Q CLI's slash command recognition
- Provides unrestricted development and system operation support
- Full access to Amazon Q CLI's capabilities
- Works within Amazon Q CLI's chat interface

## Code Review with Linus Torvalds Philosophy

The SDD agent includes Linus Torvalds' legendary code review approach via the `linus-review.md` steering document:

### Key Principles
- **"Good Taste"**: Eliminate special cases through better design
- **Data Structure Focus**: "Bad programmers worry about the code. Good programmers worry about data structures."
- **Simplicity**: Functions must be short, minimal indentation, single purpose
- **Never Break Userspace**: Maintain backward compatibility at all costs
- **Pragmatism**: Solve real problems, not theoretical ones

### 5-Layer Analysis Process
1. **Data Structure Analysis**: Focus on core data relationships
2. **Special Case Identification**: Eliminate if/else branches through redesign
3. **Complexity Review**: Reduce concepts and indentation levels
4. **Breaking Change Analysis**: Ensure backward compatibility
5. **Practicality Validation**: Verify problems are real and solutions proportionate

### Code Review Output Format
```
【Taste Score】
🟢 Good taste / 🟡 Passable / 🔴 Garbage

【Fatal Issues】
- [Direct identification of worst problems]

【Improvement Direction】
"Eliminate this special case"
"These 10 lines can become 3 lines"  
"Data structure is wrong, should be..."
```

### Usage
The Linus review philosophy is automatically applied during:
- Requirements validation
- Technical design review
- Implementation guidance  
- Direct code review requests

Simply ask: "Review this code with Linus's standards" or reference `@linus-review.md` in your requests.

## Usage Examples

### Getting Started
```bash
# 1. Start a chat with the SDD agent
q chat --agent sdd

# 2. Initialize your first specification (use natural language)
"Initialize a new specification for user authentication system"

# 3. Generate requirements  
"Generate requirements for user-authentication-system"

# 4. After reviewing, create design
"Create technical design for user-authentication-system"

# 5. Generate implementation tasks
"Break down tasks for user-authentication-system"
```

### Working with Features
```bash
# Check status of specifications
"Show status of user-authentication-system"

# Get implementation help
"Help me implement user-authentication-system"

# Create project-wide guidelines
"Set up project steering documents"

# Create specialized steering
"Create custom steering for security"
```

**Important**: Use natural language in the chat - don't type literal `/kiro:` commands. The agent recognizes your intent and translates it to the appropriate SDD workflow actions.

## Customization

To modify the agent behavior:
1. Edit the command templates in `.amazonq/commands/kiro/`
2. The agent will reference your local templates for behavior
3. Templates define exactly how the agent should respond to each command type

## Support

- **GitHub**: [amazonq-spec](https://github.com/gotalab/amazonq-spec)
- **NPM Package**: [amazonq-sdd](https://www.npmjs.com/package/amazonq-sdd)
- **Documentation**: [README](../amazonq-sdd/README.md)