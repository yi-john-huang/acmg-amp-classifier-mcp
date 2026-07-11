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

## Task 6.1: Ruleset schema, registry, and deterministic selection

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added approved, draft, retired, ambiguous, incompatible-evaluator, semantic-version, and rejection-record tests for versioned ruleset selection.

### GREEN

Added immutable ruleset and criterion specifications, semantic-version compatibility checks, a fail-closed registry, and deterministic ranked selection with accepted and rejected candidates.

### REFACTOR

Kept rule thresholds, permitted strengths, evaluator constraints, and publication state in validated specification data rather than evaluator control flow.

## Task 6.2: FactSet indexes and evaluator registry

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added typed evidence indexing, duplicate-ID rejection, evaluator-version mismatch, and complete-code registry tests.

### GREEN

Added immutable `FactSet` construction, a property-based evaluator protocol, a code-keyed immutable evaluator registry, and startup compatibility checks.

### REFACTOR

Made registry construction order-independent and corrected the evaluator protocol to expose immutable properties rather than writable structural fields.

## Task 6.3: Population criteria evaluators

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added BA1, BS1, BS2, PM2, coverage, ancestry, filter, and strength-boundary tests.

### GREEN

Added the pure population evaluator with ruleset-specified frequency, coverage, and filter comparisons. It returns typed comparisons and fails closed when the observed record is insufficient.

### REFACTOR

Centralized eligibility and comparison construction so all population criteria preserve the same audit trail.

## Task 6.4: Consequence and location evaluators

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added structured consequence tests for PVS1, PS1, PM1, PM4, PM5, BP1, BP3, and BP7, including NMD, splice, mechanism, and residue-difference boundaries.

### GREEN

Added typed consequence observations and pure evaluator logic that requires the structured consequence and gene-mechanism context selected by the active ruleset.

### REFACTOR

Used reusable typed comparisons and validated parameter readers instead of inferring evidence from free-form labels.

## Task 6.5: Functional and computational evaluators

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added calibrated-assay, duplicate-assay, score-threshold, predictor-agreement, and contradictory-predictor tests for PS3, BS3, PP3, and BP4.

### GREEN

Added pure functional/computational evaluation that requires all configured assay and prediction inputs before applying evidence.

### REFACTOR

Shared evidence-ID collection and typed ruleset parameter validation while retaining criterion-specific directions and strengths.

## Task 6.6: Case evidence evaluators

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added structured de novo, phenotype, segregation, allelic phase, case-control, and alternative-cause tests.

### GREEN

Added pure case-evidence evaluation for PS2, PM3, PM6, PP1, PP4, BP2, and BP5. Each result requires provenance-bearing, typed case facts and returns no automated success from absent context.

### REFACTOR

Kept case evidence separate from source evidence and represented contradictions as explicit non-applied or conflicting outcomes.

## Task 6.7: Remaining evidence evaluators and registry completeness

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added a complete 28-code matrix test for applied, not-applied, not-evaluable, disabled, invalid-strength, and ruleset version-incompatibility outcomes.

### GREEN

Added explicit evaluators for PS4, PP2, PP5, and BP6, including disabled/deprecated handling, and registered every 2015 criterion code in the default registry.

### REFACTOR

Removed generic successful fallbacks; unavailable or deprecated automation is represented only as explicit disabled or not-evaluable evidence.

## Task 6.8: Conflict detection and deterministic combination

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added table-driven fixtures for every project-defined ACMG 2015 pathogenic, likely-pathogenic, benign, and likely-benign combination; added generated ordering checks plus source, criterion, directional, algorithm, modified-strength, and no-match conflict tests.

### GREEN

Added a pure named/versioned combination engine with immutable decisions, canonical criteria hashes, explicit conflict records, and a data-defined matched rule for every non-conflicted result.

### REFACTOR

Moved the combination table into immutable `CombinationRule` values so matched rules are data results rather than incidental branches.

### Scientific engine phase verification

- `uv run pytest --no-cov tests/unit/domain/test_ruleset_registry.py tests/unit/domain/test_criteria_registry.py tests/unit/domain/test_population_evaluators.py tests/unit/domain/test_consequence_evaluators.py tests/unit/domain/test_functional_computational_evaluators.py tests/unit/domain/test_case_criteria_evaluators.py tests/unit/domain/test_evaluator_registry_completeness.py tests/unit/domain/test_classification_combiner.py -q`: 61 passed.
- `uv run mypy src/acmg_classifier/domain/rules.py src/acmg_classifier/domain/criteria.py src/acmg_classifier/domain/evidence.py src/acmg_classifier/domain/combination.py src/acmg_classifier/domain/evaluators`: `OK`.

## Task 7.1: Classification workflow vertical slice

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added an async integration slice with a real migrated SQLite state database, raw/evidence store, deterministic normalization port, context resolver, source adapter, ruleset, evaluator, and immutable record store. The slice covers completion, missing context, ruleset-required context, unsupported scope, bootstrap failure, partial evidence, unresolved criterion direction conflict, persistence failure, user-evidence persistence failure, and caller cancellation.

### GREEN

Added an adapter-agnostic `ClassificationService` that performs readiness, normalization, context and ruleset selection, evidence acquisition, typed user-evidence snapshot joining, fact construction, criterion evaluation, conflict-aware combination, and append-only completed-record persistence. It returns explicit completed, needs-context, degraded, conflict, unsupported, or failed states; cancellation propagates unchanged.

### REFACTOR

Added immutable workflow request, response, and record-content types. Added the evidence-query view directly to `NormalizedVariant` so the canonical normalized allele satisfies source-port requirements without a second wrapper or string reconstruction. Record content is canonicalized from sorted evidence IDs and excludes storage-assigned identifiers.

### Verification

- `uv run pytest --no-cov tests/integration/application/test_classification_service.py tests/unit/domain/test_canonical_allele.py tests/unit/application/test_variant_normalization_service.py tests/unit/application/test_context_resolution_service.py tests/integration/application/test_evidence_orchestrator_storage.py tests/integration/storage/test_record_store.py -q`: 50 passed.
- `uv run mypy src/acmg_classifier/application/classification.py src/acmg_classifier/domain/normalization.py`: `OK`.

## Task 7.2: Draft resume and progressive context questions

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added persisted-draft integration coverage for signed-token verification, expiry, completion, tampering, question-schema validation, one-question interactive progression, full-form context, unknown answers, and corrupted normalized content.

### GREEN

Added `WorkflowDraftService` with HMAC-SHA-256 continuation tokens, explicit answer states, canonical draft content, strict answer-schema checks, and state-preserving progressive context merges. `ClassificationService.resume` reuses the persisted `NormalizedVariant` and atomically completes the linked draft only after a completed classification.

### REFACTOR

Kept persistence behind a narrow draft-store protocol; normalized-content rehydration is provider-neutral and validates the canonical allele rather than recontacting a source provider.

## Task 7.3: Deterministic explanations and limitation rendering

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added compact, standard, full, conflict, template-safety, deterministic-ordering, evidence-reference, and persisted-record explanation tests.

### GREEN

Added pure `ExplanationService`, immutable structured explanation blocks, safe approved-template expansion, research-use language, and deterministic limitation aggregation. Completed, degraded, and conflict workflow responses now expose an explanation; completed records persist display prose separately from the decision payload.

### REFACTOR

Template expansion accepts only direct named rationale fields and canonicalizes non-text JSON values, preventing attribute access, conversion directives, format-spec execution, or invented facts.

## Task 7.4: Reinterpretation, replay, and difference report

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added immutable replay, deep read-only record-content, linked reinterpretation, prior-record guard, and causal evidence/context/ruleset/bundle/criteria/classification difference tests.

### GREEN

Added explicit linked `ClassificationService.reinterpret` requests and `ReinterpretationService` for source-free replay and semantic difference reports over immutable SQLite records.

### REFACTOR

Differences deliberately exclude timestamps and display prose; they compare only persisted scientific inputs and decisions.

### Scientist workflow phase verification

- `uv run pytest --no-cov tests/unit/domain/test_canonical_allele.py tests/unit/application/test_variant_normalization_service.py tests/unit/application/test_context_resolution_service.py tests/unit/application/test_explanation_service.py tests/integration/application/test_classification_service.py tests/integration/application/test_draft_workflow.py tests/integration/application/test_reinterpretation.py tests/integration/application/test_evidence_orchestrator_storage.py tests/integration/storage/test_record_store.py -q`: 71 passed.
- `uv run mypy src/acmg_classifier/application/classification.py src/acmg_classifier/application/drafts.py src/acmg_classifier/application/explanation.py src/acmg_classifier/application/reinterpretation.py src/acmg_classifier/domain/normalization.py`: `OK`.
- `uv run ruff check src/acmg_classifier/application/classification.py src/acmg_classifier/application/drafts.py src/acmg_classifier/application/explanation.py src/acmg_classifier/application/reinterpretation.py src/acmg_classifier/domain/normalization.py tests/unit/domain/test_canonical_allele.py tests/integration/application/test_classification_service.py tests/integration/application/test_draft_workflow.py tests/unit/application/test_explanation_service.py tests/integration/application/test_reinterpretation.py`: `OK`.

## Task 8.1: CLI classify, explain, doctor, and data commands

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added CLI contract tests for structured noninteractive classification, interactive one-question draft resumption, stored-detail explanation rendering, diagnostics exit codes, append-only feedback submission, feedback export/import, and bounded import files.

### GREEN

Added Typer/Rich `acmg` commands with text and deterministic JSON output, stdout/stderr separation, noninteractive exit code `2` for `needs_context`, local doctor/data readiness commands, detail-selectable immutable explanations, and feedback commands that use the shared application services.

### REFACTOR

Kept CLI request construction and response rendering in shared presentation helpers. JSON output is shared with MCP rather than independently shaped per command.

## Task 8.2: MCP server and primary tools

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added direct FastMCP contract coverage and a real stdio client/server subprocess test for initialization, tool discovery, structured workflow output, and typed runtime failure envelopes.

### GREEN

Added the official MCP SDK adapter with `classify_variant`, `explain_classification`, and `submit_feedback`. It reports progress when a request context supplies a progress token, preserves cancellation, returns structured JSON-equivalent workflow content, and keeps SDK logging off stdout.

### REFACTOR

Contained FastMCP types within `presentation/mcp`; application services remain SDK-independent.

## Task 8.3: MCP resources and advanced-mode isolation

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added resource template and advanced-tool discovery tests plus identifier-scoped evidence-resource retrieval coverage.

### GREEN

Added immutable classification, evidence snapshot, ruleset, and raw-snapshot resources. Raw content is loaded only by explicit content-addressed reference, base64 encoded, and bounded at 4 MiB. The raw snapshot tool is registered only with explicit advanced mode.

### REFACTOR

Added a narrow `ResourceService` and presentation resource protocol so raw/evidence/ruleset retrieval does not leak into routine tool responses.

## Task 8.4: Feedback CLI/MCP workflow

**Status:** Complete  
**Date:** 2026-07-11

### RED

Added integration coverage for append-only feedback, exact canonical variant/context lookup, identifier-preserving export/import, atomic import rollback, CLI serialization, and MCP submission.

### GREEN

Added immutable feedback submission/record models, `FeedbackService`, SQLite query/export support, atomic batch import, CLI feedback commands, and MCP submission. Feedback remains a separate artifact and never enters evidence acquisition, criteria evaluation, or an existing classification record.

### REFACTOR

Used canonical allele keys and copied stored interpretation context for lookup, while preserving feedback IDs and timestamps across exports and imports.

### Scientist interface phase verification

- `uv run pytest --no-cov tests/integration/storage/test_record_store.py tests/integration/application/test_feedback.py tests/integration/presentation/test_cli.py tests/integration/presentation/test_mcp_server.py tests/integration/presentation/test_mcp_stdio.py tests/unit/application/test_resources.py tests/unit/presentation/test_serialization.py -q`: 23 passed.
- `uv run mypy src`: `OK`.
- Focused Ruff checks for all Phase 8 source and test paths: `OK`.
