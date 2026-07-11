# Commit Message Guidelines

Commit messages should follow a consistent format to improve readability and provide clear context about changes. Each commit message should start with a type prefix that indicates the nature of the change.

## Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

## Type Prefixes

All commit messages must begin with one of these type prefixes:

- **docs**: Documentation changes (README, comments, etc.)
- **chore**: Maintenance tasks, dependency updates, etc.
- **feat**: New features or enhancements
- **fix**: Bug fixes
- **refactor**: Code changes that neither fix bugs nor add features
- **test**: Adding or modifying tests
- **style**: Changes that don't affect code functionality (formatting, whitespace)
- **perf**: Performance improvements
- **ci**: Changes to CI/CD configuration files and scripts

## Scope (Optional)

The scope provides additional context about which part of the codebase is affected:

- **bundles**: Signed bundle construction, verification, or lifecycle changes
- **evidence**: Source adapters, evidence models, or provenance changes
- **normalization**: Variant parsing or canonicalization changes
- **rules**: ACMG/AMP rule or evaluator changes
- **storage**: SQLite records, cache, or immutable artifact changes
- **cli** / **mcp**: User-interface or protocol-surface changes
- **packaging** / **security**: Release, dependency, or security-boundary changes

## Examples

```
feat(bundles): verify signed release archive
fix(normalization): reject ambiguous transcript input
docs(mcp): clarify routine resource availability
chore(packaging): update locked wheel smoke test
refactor(storage): centralize immutable record serialization
```

## Best Practices

1. Keep the subject line under 72 characters
2. Use imperative mood in the subject line ("add" not "added")
3. Don't end the subject line with a period
4. Separate subject from body with a blank line
5. Use the body to explain what and why, not how
6. Reference issues and pull requests in the footer

These guidelines help maintain a clean and useful git history that makes it easier to track changes and understand the project's evolution.
