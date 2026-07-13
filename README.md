# ACMG/AMP Classifier

> **Research and educational use only. Not for clinical diagnostic use, patient care, or treatment decisions.**

`acmg-classifier` is a Python package for reproducible ACMG/AMP-oriented
variant-classification workflows. It keeps scientific inputs, ruleset versions,
evidence snapshots, and workflow status explicit. It does not make a clinical
diagnosis or recommend treatment.

## Release status

This repository is an **experimental** migration release. The source-controlled
[capability support matrix](docs/release/capabilities.md) is the authoritative
record of implemented interfaces, validation level, and release blockers.
See the [release readiness decision](docs/release/readiness.md) for the
machine-checked verdict and the evidence required before any controlled
publication.

The package starts with local SQLite state and structured CLI/MCP behavior. It
does **not** ship a controlled signed data catalog, trusted release keys, or a
completed scientific classification runtime. The installed interface has no
catalog-configuration command. On clean first use, classification therefore
returns `BUNDLE_UNAVAILABLE` rather than a fabricated result; other bootstrap
or runtime failures retain their own structured error codes.

## Quick start from a source checkout

Prerequisite: Python 3.12 or 3.13 and [uv](https://docs.astral.sh/uv/).

```sh
uv tool install .
acmg --help
acmg classify 'NM_000059.4(BRCA2):c.7008-1G>A' --no-interactive --format json
```

The last command exercises the real entry point. In the current experimental
release it returns a JSON failure with `BUNDLE_UNAVAILABLE`; this is a safety
boundary, not a pathogenicity result.

For a checked-out development environment, use the equivalent:

```sh
uv sync
uv run acmg doctor --format json
```

See [scientist onboarding](docs/onboarding.md) for current commands and how to
interpret readiness states.

## MCP stdio

The supported MCP transport is stdio. A desktop-client configuration uses the
installed console entry point:

```json
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "acmg-mcp",
      "args": []
    }
  }
}
```

The primary tools are `classify_variant`, `explain_classification`, and
`submit_feedback`. `get_raw_snapshot` exists only for embedded callers of
`create_server(..., advanced=True)`; the shipped `acmg-mcp` command has no
advanced-mode switch. Tool schemas, resources, and structured response states
are recorded in the [capability matrix](docs/release/capabilities.md).

## Scope and limits

- Intended scope: germline Mendelian SNVs and small indels.
- Out of scope: somatic, CNV, structural, fusion, mitochondrial, and
  pharmacogenomic interpretation.
- Do not submit patient identifiers, credentials, or treatment questions.
- Offline replay and degraded status are implementation-tested, but usable
  offline classification requires a compatible local signed bundle/cache.
- Independent scientific golden data, live performance measurements,
  multi-platform first-use evidence, and participant usability evidence remain
  external prerequisites.

Read [known limits](docs/known-limits.md),
[safety and privacy](docs/safety-and-privacy.md), and the
[release evidence ledger](docs/release/capabilities.json) before relying on an
output.

## Documentation

- [Onboarding](docs/onboarding.md): install, readiness, CLI, and MCP use.
- [Capability support matrix](docs/release/capabilities.md): machine-checked
  interface and validation status.
- [Migration guide](docs/migration.md): legacy Go contract decisions and
  replacements.
- [Support policy](docs/support-policy.md): explicit support and deprecation
  prerequisites.
- [Safety and privacy](docs/safety-and-privacy.md): research-use boundaries and
  data-handling controls.

## Development checks

```sh
uv run pytest --no-cov tests/contract/test_release_documentation.py -q
uv run pytest --no-cov tests/integration/presentation tests/packaging -q
```

These checks prove documented local contracts. They do not establish clinical
validation, production readiness, certification, or live-source performance.
