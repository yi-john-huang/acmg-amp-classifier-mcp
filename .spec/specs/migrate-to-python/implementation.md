# Implementation Log: migrate-to-python

## Task 1.1: Package and test infrastructure

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added packaging smoke tests for `pyproject.toml`, the importable package, and the `acmg` and `acmg-mcp` entry points. The standard-library test run failed with four expected errors because neither the package nor project metadata existed.

### GREEN

Added the Python 3.12/3.13 `src` package layout, project metadata, console entry points, development dependency group, and pytest/coverage/Ruff/mypy configuration. The smoke suite passed with `PYTHONPATH=src`.

### REFACTOR

Created and synchronized a Python 3.13 environment with an exact `uv.lock`, fixed import ordering and mypy target configuration, formatted the package, and made coverage enforcement part of the default pytest command.

### Verification

- `uv run pytest -q`: 4 passed, 100% coverage
- `uv run ruff check src tests/packaging`: passed
- `uv run ruff format --check src tests/packaging`: passed
- `uv run mypy`: passed
- Both installed console entry points exited successfully

## Task 1.2: Stable enums, identifiers, and error envelope

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added tests for stable workflow/criterion status values, unknown-status rejection, request/classification identifier prefixes, and a strict machine-readable error envelope. The tests failed because the domain modules and Pydantic dependency did not exist.

### GREEN

Added framework-independent status and error types, strict prefixed identifier aliases, Pydantic v2 boundary schemas, and the locked runtime dependency.

### REFACTOR

Introduced a domain-owned recursive JSON value type so error details remain serializable without coupling the domain to Pydantic. Applied Ruff formatting and import fixes.

### Verification

- `uv run pytest -q`: 8 passed, 100% coverage
- `uv run ruff check src tests/packaging tests/unit`: passed
- `uv run ruff format --check src tests/packaging tests/unit`: passed
- `uv run mypy`: passed

## Task 1.3: Canonical serialization and hashing

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added tests for mapping/set ordering, ordered-list preservation, UTC normalization, naive timestamp and non-finite number rejection, display-field exclusion, unsupported types, and non-string object keys. The suite failed because the canonical module did not exist.

### GREEN

Implemented deterministic UTF-8 JSON and SHA-256 hashing with sorted mapping keys, sorted unordered collections, preserved list/tuple order, UTC timestamps, and explicit exclusions.

### REFACTOR

Removed an unnecessary generic Enum branch after confirming stable `StrEnum` values already serialize through the string path. Added boundary cases and reached full branch coverage.

### Verification

- `uv run pytest -q`: 16 passed, 100% line and branch coverage
- Ruff lint/format: passed
- strict mypy: passed

## Task 2.1: SQLite initialization and migrations

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added real SQLite tests for first/repeated initialization, safe pragmas, forward migration, failed-migration rollback, lock contention, invalid ordering, incompatible database versions, and unopenable paths. All failed because the infrastructure adapter did not exist.

### GREEN

Implemented explicit forward-only migrations, WAL/foreign-key/busy-timeout configuration, atomic per-migration transactions, write-readiness checks, and typed busy/migration/store errors.

### REFACTOR

Closed every SQLite connection explicitly, validated migration definitions before filesystem writes, and added incompatibility/error boundary coverage.

### Verification

- `uv run pytest -q`: 28 passed, 99% total coverage
- No resource warnings
- Ruff lint/format: passed
- strict mypy: passed


## Task 1.4: Core request, variant, context, and response models

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added tests for variant-only and full-context requests, privacy/unknown-field rejection, accession and genome-build validation, domain conversion, and every workflow response discriminator. The tests failed because the models and schemas did not exist.

### GREEN

Added immutable domain variant/context values, controlled vocabularies, strict request/context schemas, and discriminated completed, needs-context, degraded, conflict, unsupported, and failed responses.

### REFACTOR

Extracted one frozen/extra-forbid schema base and kept presentation validation separate from domain dataclasses.

### Verification

- `uv run pytest -q`: 20 passed, 100% line and branch coverage
- Ruff lint/format: passed
- strict mypy: passed

## Task 2.2: Evidence and raw snapshot persistence

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added real SQLite/filesystem tests for evidence content IDs, de-duplication, immutable rows, bounded atomic raw snapshots, missing/corrupt files, order-independent evidence snapshots, and unknown evidence references. They failed because the evidence schema and repository did not exist.

### GREEN

Added the evidence migration, immutable update/delete triggers, canonical evidence and snapshot storage, raw SHA-256 files, atomic staging, and typed retrieval errors.

### REFACTOR

Verified repeated raw content and metadata consistency, detected on-disk corruption, protected evidence snapshots from mutation, and covered absent records without leaking SQLite details.

### Verification

- `uv run pytest -q`: 36 passed, 97% total coverage
- Ruff lint/format: passed
- strict mypy: passed

## Task 2.3: Draft, classification, reinterpretation, and audit records

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added real SQLite tests for draft update/expiry/completion, immutable final records, linked reinterpretation, invalid-draft and missing-previous rollback, and append-only reviews, feedback, and audit events. They failed because record migrations and repositories did not exist.

### GREEN

Added the record migration and `SQLiteRecordStore` with timezone-safe drafts, one-transaction classification finalization, explicit relationship validation, and append-only artifact APIs.

### REFACTOR

Centralized canonical payload serialization, UTC timestamps, classification reference checks, and transaction rollback. Kept dynamic SQL constrained to an internal allowlist.

### Verification

- `uv run pytest -q`: 42 passed, 97% total coverage
- Ruff lint/format: passed
- strict mypy: passed

## Task 3.1: Bundle manifest schema and compatibility selection

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added strict manifest tests for complete provenance, hashes, HTTPS sources, safe paths, version ranges, deterministic newest selection, incompatibility explanations, and exact pinning. They failed because no bundle package existed.

### GREEN

Implemented frozen Pydantic manifest models and a deterministic compatibility resolver for application/schema versions, genome build, ruleset, and bundle pins.

### REFACTOR

Kept version comparison data-driven, sorted rejection reporting, and separated manifest validation from runtime selection.

### Verification

- `uv run pytest -q`: 47 passed, 97% total coverage
- Ruff lint/format: passed
- strict mypy: passed

## Task 3.2: Signature, archive, and artifact verification

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added Ed25519 and hostile ZIP tests for valid extraction, unknown keys, bad signatures, size/checksum mismatch, traversal, absolute/duplicate/unexpected paths, symlinks, malformed archives, expansion limits, and existing staging protection. RED first failed on the missing verifier and cryptographic runtime.

### GREEN

Added Cryptography 49 and a verifier that signs canonical manifest bytes, accepts only shipped raw Ed25519 keys, validates the complete archive member set, extracts into an untrusted temporary directory, verifies bytes and SHA-256, and exposes staging only after success.

### REFACTOR

Bounded archive expansion, rejected non-regular members and directory entries, preserved existing staging, and cleaned all temporary content on failure.

### Verification

- `uv run pytest -q`: 53 passed, 97% verifier coverage
- Ruff lint/format: passed
- strict mypy: passed

## Task 3.3: Resumable install, atomic activation, pinning, and rollback

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added integration tests using a real loopback HTTP Range server and separate lock-holder process. Cases cover first/idempotent install, partial resume, safe Range fallback, invalid Content-Range, compressed download bounds, competing installers, failed-update preservation, pinning, activation, and rollback.

### GREEN

Added the platform-independent filelock runtime, resumable HTTP transport, presentation-neutral progress events, and `BundleManager` with verified staging, installed-version reuse, atomic state replacement, pinning, and rollback.

### REFACTOR

Revalidated redirect targets, bounded streaming before and during writes, preserved partial files only for resumable transport failures, deleted invalid completed downloads, and serialized every state mutation under one cross-process lock.

### Verification

- `uv run pytest -q`: 59 passed, 96% total coverage
- Real loopback HTTP and cross-process tests passed
- Ruff lint/format: passed
- strict mypy: passed

## Task 3.4: Core bundle builder and production-candidate data release

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added integration failures for malformed MANE GeneID values, orphan and symbol-mismatched ClinGen records, colliding source staging names, insecure redirect targets, release-gate traceability, and SQLite resource warnings.

### GREEN

Added checksum-locked maintainer-only source acquisition and deterministic bundle generation that rejects invalid identifiers and cross-table mappings, uses collision-safe atomic staging, requires HTTPS across redirects, closes SQLite handles, and records the real pending scientific-validation gate.

### REFACTOR

Kept upstream parsing, source acquisition, relational validation, runtime table generation, signing, and release metadata isolated. Preserved the existing runtime-verifiable artifact, manifest provenance, and deterministic output path.

### Verification

- `uv run pytest --no-cov -W error::ResourceWarning tests/integration/data_builder`: 16 passed
- `uv run pytest --no-cov -W error::ResourceWarning tests/integration/data_builder tests/unit/bundles/test_manifest.py tests/unit/bundles/test_verifier.py tests/integration/bundles/test_manager.py -q`: 33 passed

## Task 3.5: Zero-configuration BootstrapService and doctor

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added first-run, repeat-run, disk-preflight, corrupt-bundle, doctor repair, delayed-progress, real SQLite migration, real signed bundle, offline, non-mutating doctor, and tampered installed-signature tests. The missing bootstrap module and the installed-signature check failed before implementation.

### GREEN

Added a presentation-neutral BootstrapService and BundleManager-backed provisioner. Bootstrap creates only application-owned paths, initializes real SQLite state, checks disk before provisioning, returns typed actionable reports, and shares readiness policy with doctor/repair. Provisioner selection, installed artifact checks, immutable read-only SQLite inspection, and offline injection remain outside application orchestration.

### REFACTOR

Exposed public non-extracting Ed25519 manifest verification so doctor can validate installed signatures without using a private verifier implementation. Kept doctor inspection non-mutating and repair as the only mutating doctor path.

### Verification

- `uv run pytest --no-cov tests/integration/application/test_bootstrap.py tests/integration/storage/test_sqlite_state_store.py tests/integration/bundles/test_manager.py tests/unit/bundles/test_verifier.py -q`: 33 passed

## Task 4.1: Input parsing and supported-scope rejection

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added parser and application-service tests for transcript HGVS, genomic HGVS, gene-plus-variant notation, malformed input, and explicitly unsupported release-one variant forms.

### GREEN

Added the pure `VariantInputParser` boundary and a typed `VariantNormalizationService` parse result. Unsupported inputs stop before provider selection; only supported small-variant forms can enter allele normalization.

### REFACTOR

Kept parser scope separate from canonical allele validation so a syntactically accepted input cannot be mistaken for reference-validated evidence.

## Task 4.2: Canonical allele normalization and reference validation

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added canonical allele, provider-agreement, reference-mismatch, round-trip, and deterministic merged-alias tests.

### GREEN

Added immutable SPDI-like canonical allele keys and provider-neutral normalization results. The application service rejects reference mismatches, provider disagreement, and round-trip mismatches without producing a normalized allele.

### REFACTOR

Centralized canonical identity in the domain model; provider aliases and provenance remain attached without changing the canonical key.

## Task 4.3: NCBI Variation provider and local/offline provider

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added recorded NCBI response tests, local knowledge-bundle tests, and provider-contract tests for offline and failure paths.

### GREEN

Added an NCBI Variation adapter with bounded HTTPS access, rate pacing, schema-drift handling, contextual-to-canonical SPDI conversion, and explicit typed failures. Added a local bundle-backed provider that only replays validated cached results and never opens a socket.

### REFACTOR

Provider selection stays in application code; remote and offline adapters share the provider-neutral result contract.

## Task 4.4: Transcript ranking and interpretation-context resolution

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added transcript ranking, ambiguity, disease, inheritance, provenance, and deterministic context-question tests.

### GREEN

Added immutable context models, a read-only knowledge repository, and deterministic resolution precedence for user confirmation, ruleset specification, signed bundle metadata, and unresolved context. Equal valid candidates remain questions rather than silently selected values.

### REFACTOR

Used one provenance-bearing resolved-value model for every context field and JSON-safe question schemas.

## Task 5.1: Typed evidence observations and provenance

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added strict evidence-model tests for every observation and provenance discriminator, content identifiers, context scope, and malformed source evidence.

### GREEN

Added immutable Pydantic evidence observations, source/user/derived/review provenance, deterministic evidence and snapshot IDs, and typed `FactSet` indexes.

### REFACTOR

Removed free-form evidence facts from the runtime boundary; source evidence requires both source provenance and an immutable raw snapshot reference.

## Task 5.2: Source cache, freshness, rate, timeout, and retry policy

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added controlled-clock/cache/HTTP tests for fresh, stale, ineligible, offline-miss, negative-result, retry, circuit, redirect, and response-size transitions.

### GREEN

Added a replaceable SQLite source cache and shared fail-closed HTTPS request policy. Cache transactions finish before network I/O; stale, failed, and offline-miss records are never converted into absence evidence.

### REFACTOR

Made cache writes explicitly typed and retained only bounded, non-secret failure metadata.

## Task 5.3: ClinVar retrieval adapter

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added recorded ESearch and EFetch VCV fixtures covering absence, a single SCV assertion, conflicting SCV assertions, condition mismatch, source failures, and XML schema drift.

### GREEN

Added a read-only official ClinVar adapter that discovers one candidate through ESearch and creates evidence only from VCV XML. It preserves SCV accession, submitter, condition, review status, evaluation date, citations, source version, and raw-source reference.

### REFACTOR

Separated candidate discovery, cached record validation, XML conversion, and clinical-assertion sorting without interpreting the assertion as a classification.

## Task 5.4: gnomAD population adapter

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added recorded GraphQL request and response fixtures for absent variants, zero counts with adequate coverage, ancestry counts, filters, build/release mismatch, and source schema drift.

### GREEN

Added a source-versioned gnomAD GraphQL adapter that emits typed population observations from AC/AN counts while preserving coverage, ancestry, homozygote and hemizygote counts, filters, release, query key, and raw response reference.

### REFACTOR

Kept GraphQL fixture text byte-for-byte stable and retained population fact conversion without assigning ACMG criteria.

## Task 5.5: Structured case-evidence intake

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added strict schema and conversion tests for phenotype, segregation, de novo, allelic, functional, and case-control inputs, including contradictory data, PHI fields, local paths, free text, source provenance, and criterion smuggling.

### GREEN

Added a narrow presentation schema and converter that create immutable user-provenance evidence only from validated structured case data.

### REFACTOR

Shared validation and citation handling while keeping the public intake surface unable to accept patient identity, raw source references, rule scores, or classifications.

## Task 5.6: Concurrent EvidenceOrchestrator

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added delayed-adapter and storage integration tests for concurrent start, deadline, cancellation, duplicate evidence, cache/live mixtures, one/all-source failures, and completion-order-independent snapshots.

### GREEN

Added `EvidenceOrchestrator` with `asyncio.TaskGroup`, deadline-aware source statuses, scope validation before persistence, deterministic evidence ordering, durable snapshots, and honest degraded-source reporting.

### REFACTOR

Kept orchestration source-agnostic and used explicit typed IDs at the storage boundary rather than relying on post-validation optional fields.

### Variant evidence phase verification

- Scoped Ruff check: `OK`.
- Scoped Ruff format check: 68 files already formatted.
- `uv run mypy`: `OK`.
- `uv run pytest --no-cov` across data builder, bundle, application, evidence, normalization, storage, domain, infrastructure, presentation, and contract migration tests: 227 passed in 5.99s.
