# Technology Steering

## Runtime

- Python 3.12+ package managed with `uv`.
- `src/acmg_classifier` is the only runtime implementation.
- `acmg-mcp` runs FastMCP over standard input/output only.
- Local persisted state uses SQLite; signed bundles and raw snapshots live under
the application data directory.

## Core dependencies

- Pydantic models validate public boundaries.
- Typer implements the `acmg` CLI.
- FastMCP implements the stdio MCP adapter.
- `cryptography` verifies signed data bundles.

## Commands

```sh
uv sync
uv run pytest --no-cov tests/packaging
uv run ruff check src tests
uv build
uv run acmg-mcp
uv run acmg doctor
```

## Constraints

Do not add a Go runtime, HTTP listener, PostgreSQL, Redis, Docker Swarm, or
Kubernetes deployment path. Source adapters may make controlled HTTPS requests;
the server transport remains stdio.

Use focused tests and `uv`-managed dependencies. Package artifacts must be
built from the locked environment.
