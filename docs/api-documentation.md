# CLI and MCP reference

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

The authoritative machine-readable interface inventory is the
[capability support matrix](release/capabilities.json). This page documents only
the implemented primary Python surface.

## CLI

The `acmg` console command provides:

- `classify VARIANT`
- `explain CLASSIFICATION_ID`
- `feedback CLASSIFICATION_ID`
- `feedback-export` and `feedback-import`
- `doctor`
- `data status`, `data update`, and `data install`

Use `--format text` or `--format json` where supported. Workflow output has a
stable `schema_version`, `status`, and `limitations`; failed/unsupported states
include a stable error code. `data install` does not accept a manually supplied
unsigned archive.

## MCP stdio server

Start the installed command with:

```sh
acmg-mcp
```

The current primary tools are:

| Tool | Purpose |
| --- | --- |
| `classify_variant` | Submit or resume one shared classification workflow. |
| `explain_classification` | Render a selected detail level from an immutable stored record. |
| `submit_feedback` | Append an anchored agreement/correction review note. |

`get_raw_snapshot` and raw-resource templates are advanced factory features,
not installed `acmg-mcp` capabilities. The shipped command starts routine mode
only and has no public advanced-mode switch.

The routine server advertises identifier-scoped classification, evidence, and
ruleset templates. Classification reads use the local immutable-record store and
return stored content or `CLASSIFICATION_NOT_FOUND`. The default wheel composes
no evidence/ruleset resource backend, so those reads return
`RESOURCE_UNAVAILABLE`.

## Classification request behavior

`classify_variant` accepts a supported variant and optional interpretation
context, offline/interactive controls, resume token/answers, and an explicit
optional review request. It returns one structured workflow state rather than a
collection of low-level evidence tool results. On clean first use, the installed
default composition returns `BUNDLE_UNAVAILABLE`: it ships no compatible signed
catalog, trusted release keys, or configurable scientific runtime. Other
bootstrap or runtime failures retain their own structured error codes.

Do not use an MCP output as a clinical report or treatment recommendation. See
[onboarding](onboarding.md), [known limits](known-limits.md), and
[safety and privacy](safety-and-privacy.md).
