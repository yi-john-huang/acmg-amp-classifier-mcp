# External release evidence intake

> Research and educational use only. This process does not authorize clinical
diagnosis, patient care, or treatment decisions.

The repository can validate evidence metadata, signatures, digests, and review
fields. It cannot create an independent scientific dataset, security
assessment, participant study, live workload, or release-owner decision. Those
artifacts must be supplied through the controlled release process.

## Evidence contract

Start with [`evidence-manifest.template.json`](evidence-manifest.template.json)
and replace every placeholder. The manifest must contain exactly one record for
each kind:

| Kind | Owner | Required external artifact |
| --- | --- | --- |
| `scientific_validation` | Scientific validation owner | Independently curated, provenance-approved validation dataset, protocol, results, license/redistribution decision, and independent review |
| `controlled_catalog` | Controlled release owner | Ed25519-signed catalog, trusted public-key record, catalog/archive/manifest digests, manifest/signature verification results, publication, retention, and revocation records |
| `bundle_installation` | Controlled release owner | Exact candidate bundle installed from the catalog, compatible cache, clean-install and runtime-smoke record |
| `platform_usability` | Platform and usability owner | Linux and Windows clean-install/first-use records, Python versions, a non-automation participant usability record, and a digested report |
| `security_assessment` | Security owner | Independent external assessment or certification report, scope, validity, findings disposition, and review |
| `live_performance` | Performance owner | Retained non-synthetic workload samples with platform, sample count, p95/p99, collection period, report digest, and review |
| `release_owner_approval` | Controlled release owner | Candidate-bound release decision, approval timestamp, compatibility-window end date, candidate/evidence-manifest digests, owner identity, and approval record |

Each record must include an HTTPS artifact reference, SHA-256 digest, retention
reference, owner, submission time, and review identity. The repository stores
metadata only. Do not copy report contents, raw source payloads, patient data,
credentials, private keys, or inline signatures into this repository.

## Validate before updating the matrix

```sh
uv run python scripts/validate_release_evidence.py \
  --manifest /controlled/retention/core-2026.7.10.evidence.json \
  --format text
uv run python scripts/validate_release_evidence.py \
  --manifest /controlled/retention/core-2026.7.10.evidence.json \
  --format json > /controlled/retention/core-2026.7.10.evidence-validation.json
```

Exit status `0` means all seven records are accepted and reviewed. Exit status
`1` means the manifest is structurally valid but still incomplete. Exit status
`2` means malformed or contradictory metadata. The validator performs no network
requests and never reads the referenced reports.

After the controlled release review, update only the source-controlled matrix
metadata block in `docs/release/capabilities.json`:

```json
"evidence_manifest": {
  "status": "accepted",
  "release_id": "core-2026.7.10",
  "candidate_bundle_version": "2026.7.10",
  "required_kinds": [
    "scientific_validation",
    "controlled_catalog",
    "bundle_installation",
    "platform_usability",
    "security_assessment",
    "live_performance",
    "release_owner_approval"
  ],
  "path": "controlled-release/evidence/core-2026.7.10.json",
  "sha256": "<digest of the retained manifest>",
  "verified_at": "<UTC timestamp>",
  "verified_by": "<controlled reviewer>",
  "review_status": "approved"
}
```

The matrix update must be reviewed together with the retained validation output.
Do not set `status: accepted` before every record is independently reviewed.

## Build and verify the controlled catalog

The maintainer-only data builder requires the bundle signing public key and an
operator-supplied catalog private key. Keep both key materials in the approved
secret/key-management process; never commit them.

```sh
uv run python -m data_builder catalog \
  --manifest /controlled/build/core-2026.7.10.manifest.json \
  --manifest-signature /controlled/build/core-2026.7.10.manifest.sig \
  --archive-url https://release.example/core-2026.7.10.zip \
  --catalog-version 2026.7.10 \
  --signer-key-id release-2026 \
  --private-key /secure/secret-store/catalog.key \
  --bundle-public-key /secure/key-store/bundle.pub \
  --output /controlled/release/catalog-2026.7.10.json
```

The command verifies the bundle manifest signature before signing the catalog,
uses canonical JSON, and writes no private key. `FileBundleCatalog` verifies the
catalog signature and every candidate manifest before returning candidates to
the existing bounded HTTPS bundle transport.

## Readiness decision

Run the canonical evaluator after the matrix update:

```sh
uv run python scripts/check_release_readiness.py --format json
```

The evaluator remains fail-closed if any record is missing, pending, rejected,
expired, mismatched to the candidate bundle, or not approved. Local tests,
synthetic benchmark output, package smoke, dependency scans, and history secret
scans remain engineering evidence and cannot satisfy the external records.
