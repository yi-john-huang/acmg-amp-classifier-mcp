# AI Agents Integration Guide

## Purpose

This repository ships `acmg-mcp`, a Python stdio Model Context Protocol (MCP)
server for research and educational ACMG/AMP variant interpretation. It is not a
clinical diagnostic device and its output requires professional interpretation.

## Runtime contract

- Install from the repository with `uv tool install .`.
- Launch `acmg-mcp` over standard input/output. It exposes no HTTP service.
- The supported local state store is SQLite. PostgreSQL, Redis, Docker Swarm,
  Kubernetes, and the legacy Go server are not runtime contracts.
- Run `acmg doctor` before first use to inspect readiness. The default wheel
  has no release catalog or trusted release keys, so `doctor --repair` reports
  diagnostics but cannot install a scientific runtime.

## Routine MCP surface

Routine clients receive these tools:

| Tool | Purpose |
| --- | --- |
| `classify_variant` | Classify one supported variant or resume a typed context continuation. |
| `explain_classification` | Read an immutable explanation by classification ID. |
| `submit_feedback` | Append non-authoritative feedback to a stored classification. |

Use `classify_variant` for normal workflows. The stable workflow statuses are
`completed`, `needs_context`, `degraded`, `conflict`, `unsupported`, and
`failed`; never infer a diagnosis from a non-completed response.

Routine resources are identifier-scoped:

- `acmg://classifications/{classification_id}`
- `acmg://evidence/{snapshot_id}`
- `acmg://rulesets/{ruleset_id}/{version}`

Raw source payloads and `get_raw_snapshot` are advanced-only. They require an
explicit content-addressed reference and must not be exposed in ordinary client
interfaces or copied into routine prompts.

## Engineering expectations

- Validate untrusted input at the schema boundary.
- Preserve immutable evidence, explanations, and feedback records.
- Do not log credentials, patient-identifying data, or raw source payloads.
- Add focused behavioral tests for new code and integration paths.
- Keep documentation aligned with the Python stdio/SQLite release surface.
