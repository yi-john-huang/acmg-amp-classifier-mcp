# Core data bundle builder

This maintainer-only pipeline turns checksum-pinned official sources into a
candidate core bundle. It is outside `src/` and is not shipped in the
scientist-facing Python wheel.

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

## Candidate metadata

The `core-2026.7.10` recipe pins MANE Select v1.5 and a dated ClinGen
Gene-Disease Validity snapshot. Recipe metadata records each source URL, SHA-256,
license identifier, redistribution review, reviewer, retrieval time, and
transformation version. The release metadata records candidate bundle/archive
hashes and explicitly states that a controlled release key is required and the
scientific release gate is pending.

This metadata is reproducibility evidence. It is **not** an installed signed
catalog, independently curated scientific validation dataset, or release
approval. The public support status is recorded in
[the capability matrix](../docs/release/capabilities.md).

## Controlled build flow

A release operator supplies the recipe, checked source bytes, output directory,
and a controlled signing key through the established secure release process. Do
not add a private key, source credential, or raw patient data to this repository.

Given the same recipe, source bytes, ruleset bytes, Python/SQLite toolchain, and
Ed25519 key, the builder targets byte-identical archive, manifest, signature, and
build-report outputs. Retain locked raw source inputs because public source
endpoints can change over time.

After a bundle manifest and its manifest signature have been independently
checked, a release operator can create a signed catalog with
`python -m data_builder catalog`. The command requires the bundle public key and
an operator-supplied Ed25519 catalog key; it rejects non-HTTPS archive URLs and
never writes key material. See
[`docs/release/evidence-intake.md`](../docs/release/evidence-intake.md) for the
complete handoff contract.

Transformation `1.0.0` includes MANE rows only when they have a stable HGNC
identifier. MANE v1.5 has 52 rows without an HGNC assignment; they are excluded
from the runtime mapping table and counted in candidate build review. A
non-empty malformed HGNC identifier aborts the build.
