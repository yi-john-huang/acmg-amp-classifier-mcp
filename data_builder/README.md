# Core data bundle builder

This maintainer-only pipeline turns checksum-pinned official sources into the signed,
immutable core bundle consumed by the runtime. It is outside `src/`, so it is not
shipped in the scientist-facing Python wheel.

The production-candidate recipe pins MANE Select v1.5 and the dated ClinGen
Gene-Disease Validity snapshot. Both source terms were reviewed before inclusion;
the generated `NOTICE.json` retains the releases, hashes, URLs, terms, retrieval
times, reviewers, and transformation versions. Raw CSpec guideline prose is not
compiled or activated by this pipeline.

```bash
python -m data_builder fetch \
  --recipe data_builder/recipes/core-2026.7.10.json \
  --inputs /secure/build-cache/core-2026.7.10

python -m data_builder build \
  --recipe data_builder/recipes/core-2026.7.10.json \
  --inputs /secure/build-cache/core-2026.7.10 \
  --output dist/data-bundles \
  --private-key /secure/release-keys/core-release-2026.pem
```

The signing key must be supplied from controlled release storage and must never be
placed in this repository. Given the same recipe, source bytes, ruleset bytes,
Python/SQLite toolchain, and Ed25519 key, the archive, manifest, signature, and build
report are byte-identical. Release automation should retain the locked raw source
files because ClinGen's official download endpoint serves a real-time export.

Transformation `1.0.0` includes MANE rows only when they have a stable HGNC
identifier. MANE v1.5 has 52 rows without an HGNC assignment; they are excluded
from the runtime mapping table and counted in the candidate build review. A
non-empty malformed HGNC identifier still aborts the build.
