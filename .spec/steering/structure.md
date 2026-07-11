# Repository Structure Steering

```text
src/acmg_classifier/
  application/       # workflow orchestration and use cases
  domain/            # immutable models, rules, and parsing
  infrastructure/    # SQLite, bundles, source adapters, HTTP policy
  presentation/      # CLI, MCP stdio adapter, serialization
  validation/        # release and scientific validation helpers
data_builder/        # signed bundle construction, not runtime import
tests/               # unit, integration, contract, security, packaging
docs/                # supported-release documentation
.spec/steering/      # project conventions
```

## Boundaries

- Domain code has no transport, filesystem, or database dependency.
- Application services depend on ports and return typed outcomes.
- Infrastructure implements those ports and validates external data before it
  reaches application/domain logic.
- Presentation translates CLI/MCP inputs and serializes safe outputs; it does
  not contain scientific interpretation rules.
- `data_builder` produces release artifacts in CI or explicitly invoked build
  environments and is not imported by normal server startup.

## Naming and tests

Use `snake_case` modules and functions, `PascalCase` types, and explicit value
objects for clinical identifiers. Place unit tests near their layer and
cross-layer behavior in `tests/integration`. Do not retain legacy Go layout or
HTTP deployment files as active examples.
