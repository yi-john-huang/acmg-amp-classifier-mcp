# Capability support matrix

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This page is a readable view of
[`capabilities.json`](capabilities.json). The JSON artifact is the
source-controlled contract used by documentation tests. A passing local test is
not a clinical validation or a release certification.

## Status vocabulary

| Status | Meaning |
| --- | --- |
| `experimental` | Implemented and tested at the recorded level; no independent scientific or release-wide claim. |
| `validated_for_research` | Requires independent evidence recorded in the JSON artifact. No current capability has this status. |
| `unavailable` | Deliberately absent or outside scope. |
| `deprecated` | Legacy behavior is retained only as an explicit migration decision. |
| `external_prerequisite` | Code or a local test may exist, but external evidence or release-owner input is still required. |

## Current interface summary

| Interface | Status | Evidence | Limit |
| --- | --- | --- | --- |
| `acmg` CLI | Experimental | Contract and local wheel smoke tests | First-use classification returns `BUNDLE_UNAVAILABLE` without a signed catalog. |
| `acmg-mcp` stdio | Experimental | MCP protocol tests | Only the documented primary tool surface is routine. |
| Default scientific classification | Unavailable | Structured-failure test | No bundled signed catalog or complete scientific runtime is shipped. |
| Offline replay/degraded status | Experimental | Socket-denial and replay tests | Requires compatible local bundle/cache state. |
| Legacy Go compatibility adapter | Experimental | Legacy contract test | Opt-in decision adapter only; it does not execute aliases. |

The intended scope is germline Mendelian SNVs and small indels. Somatic, CNV,
structural, fusion, mitochondrial, and pharmacogenomic interpretation are
unavailable. See [known limits](../known-limits.md).

## Candidate data bundle

The data builder records candidate metadata for bundle `2026.7.10`, including
MANE Select v1.5 and ClinGen Gene-Disease Validity source hashes, license review,
and reproducibility metadata. The release metadata explicitly says controlled
release signing is required and its scientific release gate is pending. It is
therefore an **external prerequisite**, not an installed distribution or a
scientific validation claim. Exact hashes, source URLs, licenses, and
redistribution flags are in [`capabilities.json`](capabilities.json).

## Release evidence ledger

| Task | Current status | What local evidence proves | External prerequisite |
| --- | --- | --- | --- |
| 10.1 legacy contracts | Experimental | Explicit machine-readable mappings/rejections | Release owner sets compatibility end date. |
| 10.2 scientific validation | External prerequisite | Synthetic fixture ingestion, masking, metrics, and report rendering | Provenance-approved independently curated case set with redistribution permission. |
| 10.3 offline/degraded | Experimental | No-socket normalization/replay and criterion-impact tests | Compatible end-user bundle/cache artifacts. |
| 10.4 packaging/first use | External prerequisite | Local macOS wheel install and help smoke | Linux/Windows, reference journey, and participant usability evidence. |
| 10.5 security/privacy | External prerequisite | Focused controls, locked dependency audit, and secret-scan workflow | Independent external security assessment or certification evidence. |
| 10.6 performance/tokens | External prerequisite | Deterministic byte/tool/token budget evaluation | Retained live platform/workload samples for latency claims. |

## Sources and criteria

ClinVar and gnomAD adapters are experimental fixture-tested components. The
legacy COSMIC query surface is unavailable. Criterion evaluator implementation
is experimental; it must not be described as independent clinical validation.
PP5 and BP6 are unavailable by policy. The machine-readable matrix records every
criterion row, source version, evidence path, and limitation.

## Safety boundary

Do not supply patient identifiers, credentials, or patient-care/treatment
requests. Read [safety and privacy](../safety-and-privacy.md),
[onboarding](../onboarding.md), and [migration guidance](../migration.md) before
using an interface.
