# Scientist onboarding

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This guide describes the implemented Python command surface. It does not claim
that a scientific result is available before a compatible signed data catalog is
installed.

## Install from a source checkout

Use Python 3.12 or 3.13 and uv:

```sh
uv tool install --from . acmg-classifier
acmg --help
```

For package developers, `uv sync` creates the locked local environment and
`uv run acmg --help` exercises the same console command.

The focused wheel smoke test is local macOS evidence only. Linux, Windows,
reference first-use timing, and participant usability evidence are listed as
external prerequisites in the [capability support matrix](release/capabilities.md).

## Check readiness

```sh
acmg doctor --format json
acmg data status --format json
```

The default runtime owns its SQLite state directory. It has no manual database
or service configuration step. `doctor` reports either a compatible installed
bundle or a structured readiness issue and a repair action.

A newly installed experimental wheel has no configured controlled catalog. Its
normal, safe state is:

```json
{
  "schema_version": "1.0",
  "status": "failed",
  "error_code": "BUNDLE_UNAVAILABLE"
}
```

Run the supplied repair guidance only when your release administrator provides
the appropriate signed catalog. Do not substitute a synthetic fixture or an
unverified archive.

## Submit an input

```sh
acmg classify 'NM_000059.4(BRCA2):c.7008-1G>A' --no-interactive --format json
```

With no compatible catalog, this emits `BUNDLE_UNAVAILABLE`; it is not a
classification. When a release administrator supplies a compatible runtime,
structured responses can be `completed`, `needs_context`, `degraded`,
`conflict`, `unsupported`, or `failed`. Read each response's `status`, stable
error code, limitations, and source impacts before acting on it.

Use `--offline` only with a compatible local bundle/cache. Offline mode refuses
remote normalization and records unavailable evidence sources and their affected
criteria. Stored snapshot replay does not require network access.

## Connect an MCP client

Use the installed stdio command:

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

The routine surface is `classify_variant`, `explain_classification`, and
`submit_feedback`. The routine workflow does not expose legacy low-level source
queries. `get_raw_snapshot` is explicit advanced mode and identifier-scoped.

## Safe data handling

Do not provide patient names, medical-record numbers, dates of birth,
credentials, or therapy/treatment questions. The default research workflow is
not an approved privacy mode for patient data. See
[safety and privacy](safety-and-privacy.md) and
[known limits](known-limits.md).

## Next steps

- Inspect implementation and evidence status in the
  [capability support matrix](release/capabilities.md).
- Read [migration guidance](migration.md) before replacing legacy Go calls.
- Read [support policy](support-policy.md) for unresolved release prerequisites.
