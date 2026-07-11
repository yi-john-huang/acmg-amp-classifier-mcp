# Migration from legacy Go contracts

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

The Python release replaces the legacy Go runtime with a local Python/SQLite
stdio surface. The source of truth is
[`legacy_go_manifest.json`](../src/acmg_classifier/presentation/compat/legacy_go_manifest.json),
which is contract-tested and mirrored in the
[capability support matrix](release/capabilities.md).

## Compatibility status

The compatibility adapter is opt-in and presentation-only. It does **not**
register legacy aliases, execute a mapped request, preserve legacy evidence or
results, generate a clinical report, or accept feedback without an immutable
current classification anchor. It returns a machine-readable mapping or
rejection envelope.

The compatibility-window version and end date are an **external prerequisite**:
a release owner must publish them before any removal schedule is claimed.

## Contract decisions

| Legacy tool | Decision | Current replacement | Boundary |
| --- | --- | --- | --- |
| `classify_variant` | `MAP_SAFE_CLASSIFY_REQUEST` | `classify_variant` | Only supported HGVS aliases are mapped; the adapter does not classify. |
| `validate_hgvs` | `REJECT_DEPRECATED_TOOL` | `classify_variant` | Validation is integrated into the current workflow. |
| `apply_rule` | `REJECT_DEPRECATED_TOOL` | `classify_variant` | Criterion evaluation is internal. |
| `combine_evidence` | `REJECT_DEPRECATED_TOOL` | `classify_variant` | Evidence combination is internal. |
| `query_evidence` | `REJECT_LOW_LEVEL_EVIDENCE_TOOL` | `classify_variant` | Raw evidence is not a routine compatibility surface. |
| `batch_query_evidence` | `REJECT_LOW_LEVEL_EVIDENCE_TOOL` | `classify_variant` | Batch source access is not exposed. |
| `query_clinvar` | `REJECT_LOW_LEVEL_EVIDENCE_TOOL` | `classify_variant` | Direct source access is not exposed. |
| `query_gnomad` | `REJECT_LOW_LEVEL_EVIDENCE_TOOL` | `classify_variant` | Direct source access is not exposed. |
| `query_cosmic` | `REJECT_LOW_LEVEL_EVIDENCE_TOOL` | `classify_variant` | Direct source access is not exposed. |
| `generate_report` | `REJECT_DEPRECATED_REPORT_TOOL` | `explain_classification` | A current immutable classification ID is required; no clinical report is generated. |
| `format_report` | `REJECT_DEPRECATED_REPORT_TOOL` | `explain_classification` | A current immutable classification ID is required. |
| `validate_report` | `REJECT_DEPRECATED_REPORT_TOOL` | `explain_classification` | A current immutable classification ID is required. |
| `submit_feedback` | `REJECT_UNANCHORED_FEEDBACK` | `submit_feedback` | Current feedback requires classification ID, type, rationale, and actor ID. |
| `query_feedback` | `REJECT_DEPRECATED_TOOL` | `submit_feedback` | Retrieval is not exposed by the adapter. |
| `export_feedback` | `REJECT_DEPRECATED_TOOL` | `submit_feedback` | Legacy exports lack immutable anchors. |
| `import_feedback` | `REJECT_DEPRECATED_TOOL` | `submit_feedback` | Legacy imports lack immutable anchors. |
| `list_feedback` | `REJECT_DEPRECATED_TOOL` | `submit_feedback` | Listing is not exposed by the adapter. |

## Safe request transition

For the one mapped request shape, rename `hgvs_notation` to `variant` and map a
versioned `transcript_id` or `preferred_isoform` to `transcript`. Do not map
ambiguous gene-only notation, CNV/structural/fusion inputs, mock fields, expanded
evidence payloads, report fields, or unanchored feedback. The adapter rejects
those inputs rather than fabricating compatibility.

Use the current Python CLI or the stdio MCP primary tools described in
[onboarding](onboarding.md). Consult [known limits](known-limits.md) before
assuming a legacy capability has a replacement.
