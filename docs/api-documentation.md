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

`get_raw_snapshot` is advanced mode only. It retrieves explicit raw content by
identifier and is not part of the routine classification workflow.

The server exposes identifier-scoped resources for classifications, evidence
snapshots, rulesets, and advanced raw snapshots. The exact resource templates
are listed in [capabilities.json](release/capabilities.json).

## Classification request behavior

`classify_variant` accepts a supported variant and optional interpretation
context, offline/interactive controls, resume token/answers, and explicit
optional review request. It returns one structured workflow state rather than a
collection of low-level evidence tool results. Current default first use returns
`BUNDLE_UNAVAILABLE` until a compatible signed catalog and scientific runtime
are supplied.

Do not use an MCP output as a clinical report or treatment recommendation. See
[onboarding](onboarding.md), [known limits](known-limits.md), and
[safety and privacy](safety-and-privacy.md).
