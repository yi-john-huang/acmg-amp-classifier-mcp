# Support and deprecation policy

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This experimental migration has no published support SLA, release cadence,
security-response interval, or data-bundle retention window. Those are
**external prerequisites** for a release owner to define; this page does not
invent dates or guarantees.

## Current policy fields

| Policy area | Current state | Evidence or next action |
| --- | --- | --- |
| Package support | Experimental | Python package and local contract tests are recorded in the [capability support matrix](release/capabilities.md). |
| Compatibility window | External prerequisite | A release owner must choose a version and end date for the opt-in legacy adapter. |
| Deprecation notice | External prerequisite | Publish a notice period with the compatibility-window decision. |
| Security updates | External prerequisite | Publish a private reporting contact, triage process, and response targets. |
| Data bundles | Experimental | Candidate metadata is reproducible, but controlled signing, catalog distribution, retention, and revocation policy are not published. |
| Scientific validation | External prerequisite | An independently curated provenance-approved dataset and retained report are required. |
| Performance | External prerequisite | Live platform/workload evidence is required before latency commitments. |

## Using experimental interfaces

Use only documented Python CLI and stdio MCP interfaces. Every capability is
labeled `experimental`, `unavailable`, `deprecated`, or `external_prerequisite`
until evidence supports a stronger status. `validated_for_research` must have
independent scientific evidence recorded in the matrix; it is not inferred from
a passing unit test.

## Legacy migration

Legacy Go requests are not a supported runtime. The opt-in compatibility adapter
classifies a request as safely mapped or explicitly rejected; it neither registers
aliases nor preserves fabricated results. See [migration guidance](migration.md).

## Escalation

If an output lacks a compatible bundle, required context, available source, or
independent validation evidence, stop at its structured status. Do not bypass a
safety check with an unverified archive, raw source payload, or hand-edited
classification. See [safety and privacy](safety-and-privacy.md) and
[known limits](known-limits.md).
