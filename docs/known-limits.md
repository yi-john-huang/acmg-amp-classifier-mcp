# Known limits

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

The authoritative machine-readable list is the
[capability support matrix](release/capabilities.json). This page explains the
limits that matter when interpreting an interface response.

## Scientific status

- The intended scope is germline Mendelian SNVs and small indels.
- Somatic variants, CNVs, structural variants, fusions, mitochondrial variants,
  and pharmacogenomic interpretation are unavailable.
- Implemented criterion evaluators are experimental implementation work. They
  are not an independently clinically validated assertion.
- PP5 and BP6 are unavailable by policy and are not silently applied.
- The included validation fixture is a synthetic smoke fixture. A
  provenance-approved independently curated scientific case set with
  redistribution permission is an external prerequisite for a scientific
  validation claim.

## Runtime and data availability

The installed package initializes local SQLite state, but it deliberately ships
without a controlled signed release catalog or a completed scientific
classification composition. A first-use `BUNDLE_UNAVAILABLE` response means no
scientific result was generated.

Candidate data-builder metadata records reproducible source hashes and license
review. It is not an installed signed distribution, an independent scientific
validation dataset, or permission to invent a release key. Consult the
[candidate data section](release/capabilities.md#candidate-data-bundle) before
handling a release artifact.

## Evidence and offline behavior

ClinVar and gnomAD adapters are implementation/fixture tested. Their presence
does not prove current live-source availability or clinical validity. Other
legacy low-level source tools are unavailable in this Python release.

Offline mode refuses remote normalization. It can replay stored snapshots and
return degraded status only when compatible local bundle/cache state exists.
Unavailable sources identify affected criteria; missing data never becomes
absence evidence.

## Validation and release evidence

The following remain external prerequisites:

1. An independently curated, provenance-approved scientific validation dataset.
2. Clean Linux and Windows installation evidence, reference first-use timing,
   and participant usability study evidence.
3. Retained live workload/platform benchmark samples for latency claims.
4. Independent external security assessment or certification evidence.
5. A release-owner decision for the legacy compatibility-window end date.

Local tests, synthetic benchmark arithmetic, and workflow definitions are not
substitutes for those records. See [support policy](support-policy.md) and
[safety and privacy](safety-and-privacy.md).
