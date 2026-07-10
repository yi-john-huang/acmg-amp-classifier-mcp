from __future__ import annotations

import asyncio
import json
import unittest
from dataclasses import FrozenInstanceError, dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from tempfile import TemporaryDirectory
from typing import cast

from acmg_classifier.application.classification import (
    ClassificationRequest,
    ClassificationService,
    CompletedClassificationResponse,
    ConflictClassificationResponse,
    DegradedClassificationResponse,
    FailedClassificationResponse,
    NeedsContextClassificationResponse,
    UnsupportedClassificationResponse,
)
from acmg_classifier.application.drafts import (
    DraftAnswer,
    DraftAnswerState,
    DraftContinuation,
    WorkflowDraftService,
)
from acmg_classifier.application.evidence_orchestrator import (
    EvidenceAcquisitionResult,
    EvidenceOrchestrator,
)
from acmg_classifier.domain.combination import ClassificationCombiner
from acmg_classifier.domain.context import (
    ContextField,
    ContextIssueCode,
    ContextQuestion,
)
from acmg_classifier.domain.criteria import (
    CriteriaEngine,
    CriterionAssessment,
    EvaluatorRegistry,
)
from acmg_classifier.domain.enums import (
    ClassificationTier,
    CriterionStatus,
    GenomeBuild,
    WorkflowStatus,
)
from acmg_classifier.domain.evaluators.population import PopulationCriterionEvaluator
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    EvidencePolicy,
    EvidencePolicyMode,
    EvidenceSnapshot,
    PhenotypeObservation,
    PopulationObservation,
    SourceProvenance,
    SourceStatus,
    SourceStatusValue,
    UserProvenance,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizationFailureCode,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetRegistry,
    RulesetScope,
    RulesetSpecification,
    RulesetState,
)
from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore
from acmg_classifier.infrastructure.storage.records import SQLiteRecordStore
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore
from acmg_classifier.ports.evidence import CacheState, EvidenceSourceResult
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationSuccess,
)

_NOW = datetime(2026, 7, 11, 12, 0, tzinfo=UTC)


@dataclass(frozen=True, slots=True)
class _ContextResolution:
    resolved: InterpretationContext
    status: str = "complete"
    questions: tuple[ContextQuestion, ...] = ()


class _ContextResolver:
    def __init__(self, resolution: _ContextResolution) -> None:
        self.resolution = resolution

    def resolve(
        self,
        normalized: NormalizedVariant,
        context: InterpretationContext,
    ) -> _ContextResolution:
        del normalized, context
        return self.resolution


class _Normalizer:
    def __init__(self, result: NormalizationSuccess | NormalizationFailure) -> None:
        self.result = result

    def normalize(
        self,
        value: str,
        *,
        context: InterpretationContext,
    ) -> NormalizationSuccess | NormalizationFailure:
        del value, context
        return self.result


class _CountingNormalizer(_Normalizer):
    def __init__(self, result: NormalizationSuccess | NormalizationFailure) -> None:
        super().__init__(result)
        self.calls = 0

    def normalize(
        self,
        value: str,
        *,
        context: InterpretationContext,
    ) -> NormalizationSuccess | NormalizationFailure:
        self.calls += 1
        return super().normalize(value, context=context)


class _DiseaseContextResolver:
    def resolve(
        self,
        normalized: NormalizedVariant,
        context: InterpretationContext,
    ) -> _ContextResolution:
        del normalized
        if context.disease_id is None:
            return _ContextResolution(
                resolved=context,
                status="needs_context",
                questions=(
                    ContextQuestion(
                        field=ContextField.DISEASE,
                        code=ContextIssueCode.DISEASE_REQUIRED,
                        prompt="Select a disease context.",
                        reason="The selected ruleset is disease-specific.",
                        answer_schema={"type": "object"},
                    ),
                ),
            )
        return _ContextResolution(resolved=context)


class _DraftService:
    def __init__(self) -> None:
        self.created = 0

    def create(self, **_: object) -> DraftContinuation:
        self.created += 1
        return DraftContinuation(
            draft_id=f"draft_{self.created:032x}",
            resume_token=f"resume_{self.created}",
        )

    def update(self, *, draft_id: str, **_: object) -> DraftContinuation:
        return DraftContinuation(
            draft_id=draft_id,
            resume_token=f"resume_{draft_id.removeprefix('draft_')}",
        )


@dataclass(frozen=True, slots=True)
class _ReadinessReport:
    ready: bool


class _Readiness:
    def __init__(self, ready: bool) -> None:
        self.ready = ready

    def ensure_ready(self) -> _ReadinessReport:
        return _ReadinessReport(ready=self.ready)


class _PopulationAdapter:
    source_id = "gnomad"

    def __init__(self, item: EvidenceItem) -> None:
        self.item = item

    async def query(
        self,
        variant: NormalizedVariant,
        *,
        policy: EvidencePolicy,
    ) -> EvidenceSourceResult:
        del policy
        return EvidenceSourceResult(
            source_id=self.source_id,
            evidence_items=(self.item,),
            source_status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.FRESH,
                checked_at=_NOW,
                normalized_query_key=variant.variant_key,
            ),
            cache_state=CacheState.LIVE,
        )


class _CancelledEvidence:
    async def gather(
        self, *args: object, **kwargs: object
    ) -> EvidenceAcquisitionResult:
        del args, kwargs
        raise asyncio.CancelledError


class _StaticEvidence:
    def __init__(self, result: EvidenceAcquisitionResult) -> None:
        self.result = result

    async def gather(
        self, *args: object, **kwargs: object
    ) -> EvidenceAcquisitionResult:
        del args, kwargs
        return self.result


class _FailingRecordStore:
    def finalize_classification(self, record: object, **kwargs: object) -> str:
        del record, kwargs
        raise RuntimeError("sqlite unavailable")


class _ConflictingCriteriaEngine:
    def evaluate(
        self,
        facts: object,
        context: InterpretationContext,
        ruleset: RulesetSpecification,
    ) -> dict[CriterionCode, CriterionAssessment]:
        del facts, context
        return {
            CriterionCode.PS1: _assessment(
                CriterionCode.PS1,
                CriterionStrength.STRONG,
                ruleset,
            ),
            CriterionCode.BS1: _assessment(
                CriterionCode.BS1,
                CriterionStrength.STRONG,
                ruleset,
            ),
        }


class ClassificationServiceIntegrationTests(unittest.IsolatedAsyncioTestCase):
    async def test_completed_workflow_persists_immutable_sqlite_record(self) -> None:
        with TemporaryDirectory() as temporary_directory:
            root = Path(temporary_directory)
            database = root / "state.sqlite3"
            SQLiteStateStore(database).initialize()
            evidence_store = SQLiteEvidenceStore(database, root / "raw")
            normalized = _normalized_variant()
            item = _population_item(
                normalized.variant_key,
                _scope(),
                evidence_store.put_raw_snapshot(
                    b"{}", "application/json"
                ).snapshot_hash,
            )
            ruleset = _ruleset({CriterionCode.PM2})
            user_item = _phenotype_item(normalized.variant_key, _scope())
            service = _service(
                normalized=normalized,
                evidence=EvidenceOrchestrator(
                    adapters=(_PopulationAdapter(item),),
                    evidence_store=evidence_store,
                    clock=lambda: _NOW,
                    total_deadline_seconds=1.0,
                ),
                record_store=SQLiteRecordStore(database, clock=lambda: _NOW),
                ruleset=ruleset,
                criteria_engine=CriteriaEngine(
                    EvaluatorRegistry(
                        (PopulationCriterionEvaluator(CriterionCode.PM2),)
                    )
                ),
                user_evidence_store=evidence_store,
            )

            response = await service.classify(_request(user_evidence=(user_item,)))

            self.assertIsInstance(response, CompletedClassificationResponse)
            completed = cast(CompletedClassificationResponse, response)
            self.assertEqual(completed.status, WorkflowStatus.COMPLETED)
            self.assertEqual(
                completed.decision.classification,
                ClassificationTier.UNCERTAIN_SIGNIFICANCE,
            )
            self.assertEqual(
                next(
                    assessment.status
                    for assessment in completed.decision.assessments
                    if assessment.code is CriterionCode.PM2
                ),
                CriterionStatus.APPLIED,
            )
            self.assertTrue(completed.classification_id.startswith("cls_"))
            stored = SQLiteRecordStore(database).get_classification(
                completed.classification_id
            )
            record = json.loads(stored.canonical_json)
            self.assertEqual(
                record["normalized_variant"]["canonical_key"]["assembly"],
                "GRCh38",
            )
            self.assertEqual(
                record["decision"]["classification"], "uncertain_significance"
            )
            self.assertEqual(
                record["explanation"],
                completed.explanation.to_canonical_content(),
            )
            self.assertIn("Research use only", record["explanation"]["disclaimer"])
            self.assertEqual(
                record["evidence_snapshot"]["snapshot_id"], completed.snapshot_id
            )
            self.assertEqual(record["request"]["analysis_intent"], "germline_mendelian")
            self.assertEqual(
                record["request"]["user_evidence_ids"], [user_item.evidence_id]
            )
            self.assertIn(
                user_item.evidence_id,
                completed.record.evidence_snapshot.evidence_ids,
            )
            self.assertTrue(evidence_store.get_evidence(user_item.evidence_id))
            self.assertEqual(completed.record.to_canonical_content(), record)
            with self.assertRaises(FrozenInstanceError):
                completed.record.bundle_version = "changed"  # type: ignore[misc]

    async def test_reinterpretation_creates_a_new_linked_immutable_record(
        self,
    ) -> None:
        with TemporaryDirectory() as temporary_directory:
            database = Path(temporary_directory) / "state.sqlite3"
            SQLiteStateStore(database).initialize()
            records = SQLiteRecordStore(database, clock=lambda: _NOW)
            service = _service(record_store=records)

            original = await service.classify(_request())
            self.assertIsInstance(original, CompletedClassificationResponse)
            original_id = cast(
                CompletedClassificationResponse, original
            ).classification_id
            reinterpreted = await service.reinterpret(
                _request(),
                previous_classification_id=original_id,
            )

            self.assertIsInstance(reinterpreted, CompletedClassificationResponse)
            new_id = cast(
                CompletedClassificationResponse, reinterpreted
            ).classification_id
            self.assertNotEqual(new_id, original_id)
            self.assertEqual(
                records.get_classification(new_id).previous_classification_id,
                original_id,
            )

    async def test_reinterpretation_rejects_missing_prior_record_before_sources(
        self,
    ) -> None:
        with TemporaryDirectory() as temporary_directory:
            database = Path(temporary_directory) / "state.sqlite3"
            SQLiteStateStore(database).initialize()
            normalized = _normalized_variant()
            normalizer = _CountingNormalizer(NormalizationSuccess(normalized))
            service = _service(
                normalizer=normalizer,
                record_store=SQLiteRecordStore(database, clock=lambda: _NOW),
            )

            response = await service.reinterpret(
                _request(),
                previous_classification_id="cls_" + "f" * 32,
            )

            self.assertIsInstance(response, FailedClassificationResponse)
            self.assertEqual(
                cast(FailedClassificationResponse, response).error_code,
                "PREVIOUS_CLASSIFICATION_NOT_FOUND",
            )
            self.assertEqual(normalizer.calls, 0)

    async def test_missing_context_returns_typed_non_completed_state(self) -> None:
        normalized = _normalized_variant()
        question = ContextQuestion(
            field=ContextField.DISEASE,
            code=ContextIssueCode.DISEASE_REQUIRED,
            prompt="Select a disease context.",
            reason="The selected ruleset is disease-specific.",
            answer_schema={"type": "string", "minLength": 1},
        )
        service = _service(
            normalized=normalized,
            context_resolution=_ContextResolution(
                resolved=InterpretationContext(genome_build=GenomeBuild.GRCH38),
                status="needs_context",
                questions=(question,),
            ),
            evidence=_CancelledEvidence(),
        )

        response = await service.classify(_request())

        self.assertIsInstance(response, NeedsContextClassificationResponse)
        needs_context = cast(NeedsContextClassificationResponse, response)
        self.assertEqual(needs_context.status, WorkflowStatus.NEEDS_CONTEXT)
        self.assertEqual(needs_context.questions, (question,))
        self.assertIsNone(needs_context.classification)

    async def test_interactive_context_flow_asks_one_question_per_resume_step(
        self,
    ) -> None:
        questions = (
            ContextQuestion(
                field=ContextField.DISEASE,
                code=ContextIssueCode.DISEASE_REQUIRED,
                prompt="Select a disease context.",
                reason="The selected ruleset is disease-specific.",
                answer_schema={"type": "object"},
            ),
            ContextQuestion(
                field=ContextField.INHERITANCE,
                code=ContextIssueCode.INHERITANCE_REQUIRED,
                prompt="Select an inheritance context.",
                reason="The selected ruleset is inheritance-specific.",
                answer_schema={"type": "string"},
            ),
        )
        resolution = _ContextResolution(
            resolved=InterpretationContext(genome_build=GenomeBuild.GRCH38),
            status="needs_context",
            questions=questions,
        )

        interactive = await _service(
            context_resolution=resolution,
            evidence=_CancelledEvidence(),
        ).classify(_request())
        full_form = await _service(
            context_resolution=resolution,
            evidence=_CancelledEvidence(),
        ).classify(
            ClassificationRequest(
                variant="NC_000001.11:g.101A>G",
                context=_resolved_context(),
                interactive=False,
            )
        )

        self.assertIsInstance(interactive, NeedsContextClassificationResponse)
        self.assertEqual(
            cast(NeedsContextClassificationResponse, interactive).questions,
            questions[:1],
        )
        self.assertIsInstance(full_form, NeedsContextClassificationResponse)
        self.assertEqual(
            cast(NeedsContextClassificationResponse, full_form).questions,
            questions,
        )

    async def test_resuming_draft_reuses_normalized_variant_and_finalizes_once(
        self,
    ) -> None:
        with TemporaryDirectory() as temporary_directory:
            database = Path(temporary_directory) / "state.sqlite3"
            SQLiteStateStore(database).initialize()
            record_store = SQLiteRecordStore(database, clock=lambda: _NOW)
            normalized = _normalized_variant()
            normalizer = _CountingNormalizer(NormalizationSuccess(normalized))
            service = ClassificationService(
                normalizer=normalizer,
                context_resolver=_DiseaseContextResolver(),
                rulesets=RulesetRegistry((_ruleset(),)),
                criteria_engine=CriteriaEngine(EvaluatorRegistry(())),
                combiner=ClassificationCombiner(),
                evidence_orchestrator=cast(
                    EvidenceOrchestrator,
                    _StaticEvidence(_evidence_result(normalized)),
                ),
                record_store=record_store,
                bundle_version="core-test-1",
                clock=lambda: _NOW,
                draft_service=WorkflowDraftService(
                    store=record_store,
                    signing_key=b"d" * 32,
                    clock=lambda: _NOW,
                    lifetime=timedelta(hours=1),
                ),
            )
            request = _request(
                context=InterpretationContext(
                    genome_build=GenomeBuild.GRCH38,
                    transcript="NM_000001.1",
                )
            )

            initial = await service.classify(request)

            self.assertIsInstance(initial, NeedsContextClassificationResponse)
            pending = cast(NeedsContextClassificationResponse, initial)
            self.assertIsNone(
                record_store.get_draft(pending.draft_id).completed_classification_id
            )
            resumed = await service.resume(
                pending.resume_token,
                answers=(
                    DraftAnswer(
                        field=ContextField.DISEASE,
                        state=DraftAnswerState.PROVIDED,
                        value={
                            "identifier": "MONDO:0000001",
                            "label": "Test disease",
                        },
                    ),
                ),
            )

            self.assertIsInstance(resumed, CompletedClassificationResponse)
            completed = cast(CompletedClassificationResponse, resumed)
            self.assertEqual(normalizer.calls, 1)
            self.assertEqual(
                record_store.get_draft(pending.draft_id).completed_classification_id,
                completed.classification_id,
            )

    async def test_unsupported_normalization_never_queries_evidence(self) -> None:
        failure = NormalizationFailure(
            code=NormalizationFailureCode.UNSUPPORTED_VARIANT_SCOPE,
            message="structural variants are outside release-one scope",
            retryable=False,
        )
        service = _service(
            normalizer=_Normalizer(failure),
            evidence=_CancelledEvidence(),
        )

        response = await service.classify(_request())

        self.assertIsInstance(response, UnsupportedClassificationResponse)
        unsupported = cast(UnsupportedClassificationResponse, response)
        self.assertEqual(unsupported.status, WorkflowStatus.UNSUPPORTED)
        self.assertIn("structural variants", unsupported.reason)

    async def test_bootstrap_not_ready_stops_before_normalization(self) -> None:
        service = _service(
            normalizer=_Normalizer(NormalizationSuccess(_normalized_variant())),
            evidence=_CancelledEvidence(),
            readiness=_Readiness(False),
        )

        response = await service.classify(_request())

        self.assertIsInstance(response, FailedClassificationResponse)
        failed = cast(FailedClassificationResponse, response)
        self.assertEqual(failed.error_code, "BOOTSTRAP_NOT_READY")

    async def test_ruleset_missing_context_returns_generated_question(self) -> None:
        context = InterpretationContext(
            genome_build=GenomeBuild.GRCH38,
            transcript="NM_000001.1",
        )
        service = _service(
            context_resolution=_ContextResolution(resolved=context),
            evidence=_CancelledEvidence(),
            ruleset=_ruleset(scope=RulesetScope(disease_id="MONDO:0000001")),
        )

        response = await service.classify(_request(context=context))

        self.assertIsInstance(response, NeedsContextClassificationResponse)
        needs_context = cast(NeedsContextClassificationResponse, response)
        self.assertEqual(needs_context.questions[0].field, ContextField.DISEASE)
        self.assertEqual(
            needs_context.questions[0].code,
            ContextIssueCode.DISEASE_REQUIRED,
        )

    async def test_partial_evidence_returns_degraded_result_with_decision(self) -> None:
        normalized = _normalized_variant()
        service = _service(
            normalized=normalized,
            evidence=_StaticEvidence(_evidence_result(normalized, unavailable=True)),
        )

        response = await service.classify(_request())

        self.assertIsInstance(response, DegradedClassificationResponse)
        degraded = cast(DegradedClassificationResponse, response)
        self.assertEqual(degraded.status, WorkflowStatus.DEGRADED)
        self.assertEqual(degraded.unavailable_sources, ("clinvar",))
        self.assertEqual(
            degraded.decision.classification,
            ClassificationTier.UNCERTAIN_SIGNIFICANCE,
        )
        self.assertEqual(degraded.explanation.classification, "Uncertain Significance")

    async def test_conflict_returns_no_five_tier_classification(self) -> None:
        normalized = _normalized_variant()
        service = _service(
            normalized=normalized,
            evidence=_StaticEvidence(_evidence_result(normalized)),
            criteria_engine=_ConflictingCriteriaEngine(),
            ruleset=_ruleset({CriterionCode.PS1, CriterionCode.BS1}),
            evaluator_versions={"test": "1.0.0"},
        )

        response = await service.classify(_request())

        self.assertIsInstance(response, ConflictClassificationResponse)
        conflict = cast(ConflictClassificationResponse, response)
        self.assertEqual(conflict.status, WorkflowStatus.CONFLICT)
        self.assertIsNone(conflict.classification)
        self.assertEqual(conflict.decision.conflict.kind.value, "directional")
        self.assertIsNone(conflict.explanation.classification)
        self.assertIn("Conflict", conflict.explanation.blocks[0].text)

    async def test_persistence_failure_is_typed_and_cancellation_propagates(
        self,
    ) -> None:
        normalized = _normalized_variant()
        failed_service = _service(
            normalized=normalized,
            evidence=_StaticEvidence(_evidence_result(normalized)),
            record_store=_FailingRecordStore(),
        )

        failed = await failed_service.classify(_request())

        self.assertIsInstance(failed, FailedClassificationResponse)
        self.assertEqual(failed.status, WorkflowStatus.FAILED)
        self.assertEqual(failed.error_code, "CLASSIFICATION_PERSISTENCE_FAILED")

        user_evidence_failure = await failed_service.classify(
            _request(user_evidence=(_phenotype_item(normalized.variant_key, _scope()),))
        )
        self.assertIsInstance(user_evidence_failure, FailedClassificationResponse)
        self.assertEqual(
            cast(FailedClassificationResponse, user_evidence_failure).error_code,
            "USER_EVIDENCE_PERSISTENCE_FAILED",
        )

        cancelled_service = _service(
            normalized=normalized, evidence=_CancelledEvidence()
        )
        with self.assertRaises(asyncio.CancelledError):
            await cancelled_service.classify(_request())


def _service(
    *,
    normalized: NormalizedVariant | None = None,
    normalizer: _Normalizer | None = None,
    context_resolution: _ContextResolution | None = None,
    evidence: object | None = None,
    criteria_engine: object | None = None,
    record_store: object | None = None,
    ruleset: RulesetSpecification | None = None,
    evaluator_versions: dict[str, str] | None = None,
    readiness: _Readiness | None = None,
    user_evidence_store: object | None = None,
    draft_service: object | None = None,
) -> ClassificationService:
    selected_normalized = normalized or _normalized_variant()
    return ClassificationService(
        normalizer=normalizer or _Normalizer(NormalizationSuccess(selected_normalized)),
        context_resolver=_ContextResolver(
            context_resolution or _ContextResolution(resolved=_resolved_context())
        ),
        rulesets=RulesetRegistry((ruleset or _ruleset(),)),
        criteria_engine=cast(
            CriteriaEngine,
            criteria_engine or CriteriaEngine(EvaluatorRegistry(())),
        ),
        combiner=ClassificationCombiner(),
        evidence_orchestrator=cast(
            EvidenceOrchestrator,
            evidence or _StaticEvidence(_evidence_result(selected_normalized)),
        ),
        record_store=record_store,
        bundle_version="core-test-1",
        clock=lambda: _NOW,
        readiness=readiness,
        evaluator_versions=evaluator_versions,
        user_evidence_store=cast(object, user_evidence_store),
        draft_service=cast(
            WorkflowDraftService,
            draft_service or _DraftService(),
        ),
    )


def _request(
    *,
    context: InterpretationContext | None = None,
    user_evidence: tuple[EvidenceItem, ...] = (),
) -> ClassificationRequest:
    return ClassificationRequest(
        variant="NC_000001.11:g.101A>G",
        context=context or _resolved_context(),
        evidence_policy=EvidencePolicy(mode=EvidencePolicyMode.LIVE),
        user_evidence=user_evidence,
    )


def _resolved_context() -> InterpretationContext:
    return InterpretationContext(
        genome_build=GenomeBuild.GRCH38,
        transcript="NM_000001.1",
        disease_id="MONDO:0000001",
        disease_label="Test disease",
    )


def _scope() -> EvidenceContextScope:
    context = _resolved_context()
    return EvidenceContextScope(
        genome_build=context.genome_build,
        transcript=context.transcript,
        disease_id=context.disease_id,
    )


def _normalized_variant() -> NormalizedVariant:
    key = CanonicalAlleleKey(
        assembly=GenomeBuild.GRCH38,
        sequence_accession="NC_000001.11",
        start=100,
        end=101,
        deleted_sequence="A",
        inserted_sequence="G",
    )
    return NormalizedVariant(
        original_input="NC_000001.11:g.101A>G",
        parsed_input="NC_000001.11:g.101A>G",
        canonical_key=key,
        genome_build=GenomeBuild.GRCH38,
        genomic_accession="NC_000001.11",
        genomic_start=100,
        genomic_end=101,
        reference_allele="A",
        alternate_allele="G",
        normalized_hgvs="NC_000001.11:g.101A>G",
        hgvs_aliases=(HgvsAlias("NC_000001.11:g.101A>G", "genomic"),),
        gene_symbol="TEST1",
        transcript="NM_000001.1",
        transcript_hgvs="NM_000001.1:c.1A>G",
        provider_provenance=(
            ProviderProvenance(
                provider_id="test-normalizer",
                provider_version="1.0.0",
                source_record_id="record-1",
                retrieved_at=_NOW,
                bundle_version="core-test-1",
                raw_snapshot_ref=None,
                query_key=key.value,
            ),
        ),
    )


def _population_item(
    variant_key: str,
    scope: EvidenceContextScope,
    raw_snapshot_ref: str,
) -> EvidenceItem:
    return EvidenceItem(
        variant_key=variant_key,
        kind="population",
        observation=PopulationObservation(
            kind="population",
            source_release="test-1",
            allele_count=0,
            allele_number=10_000,
            allele_frequency=0.0,
            coverage=30.0,
            filter_status="PASS",
        ),
        context_scope=scope,
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id="gnomad",
            source_record_id="record-1",
            source_version="test-1",
            retrieved_at=_NOW,
            normalized_query_key=variant_key,
        ),
        raw_snapshot_ref=raw_snapshot_ref,
        derivation=EvidenceDerivation.SOURCE,
    )


def _phenotype_item(
    variant_key: str,
    scope: EvidenceContextScope,
) -> EvidenceItem:
    return EvidenceItem(
        variant_key=variant_key,
        kind="phenotype",
        observation=PhenotypeObservation(
            kind="phenotype",
            term_id="HP:0003002",
            state="present",
            specificity="highly_specific",
        ),
        context_scope=scope,
        provenance=UserProvenance(
            kind=EvidenceDerivation.USER,
            actor_id="usr_test",
            submitted_at=_NOW,
            confirmation_method="attested",
        ),
        derivation=EvidenceDerivation.USER,
    )


def _evidence_result(
    normalized: NormalizedVariant,
    *,
    unavailable: bool = False,
) -> EvidenceAcquisitionResult:
    item = _population_item(normalized.variant_key, _scope(), "raw_" + "b" * 64)
    source_statuses = [
        SourceStatus(
            source_id="gnomad",
            status=SourceStatusValue.FRESH,
            checked_at=_NOW,
            normalized_query_key=normalized.variant_key,
        )
    ]
    if unavailable:
        source_statuses.append(
            SourceStatus(
                source_id="clinvar",
                status=SourceStatusValue.UNAVAILABLE,
                checked_at=_NOW,
                detail="source_down",
            )
        )
    return EvidenceAcquisitionResult(
        snapshot=EvidenceSnapshot(
            evidence_ids=(item.evidence_id,),
            source_statuses=tuple(source_statuses),
            policy=EvidencePolicy(mode=EvidencePolicyMode.LIVE),
            created_at=_NOW,
        ),
        evidence_items=(item,),
        source_results=(),
        degraded=unavailable,
        unavailable_sources=("clinvar",) if unavailable else (),
    )


def _assessment(
    code: CriterionCode,
    strength: CriterionStrength,
    ruleset: RulesetSpecification,
) -> CriterionAssessment:
    return CriterionAssessment(
        code=code,
        status=CriterionStatus.APPLIED,
        original_strength=strength,
        applied_strength=strength,
        evidence_ids=("ev_" + "a" * 64,),
        comparisons=(),
        rationale_template=f"{code.value} applied",
        rationale_values={},
        limitations=(),
        ruleset_id=ruleset.ruleset_id,
        ruleset_version=ruleset.version,
        evaluator_id="test",
        evaluator_version="1.0.0",
    )


def _ruleset(
    enabled_codes: set[CriterionCode] | None = None,
    *,
    scope: RulesetScope | None = None,
) -> RulesetSpecification:
    enabled = enabled_codes or set()
    criteria = []
    for code in CriterionCode:
        is_enabled = code in enabled
        if code is CriterionCode.PM2 and is_enabled:
            criteria.append(
                CriterionSpecification(
                    code=code,
                    enabled=True,
                    base_strength=CriterionStrength.MODERATE,
                    allowed_strengths=(CriterionStrength.MODERATE,),
                    evaluator_id="population",
                    evaluator_version_constraint=">=1.0.0,<2.0.0",
                    rationale_template="PM2 rationale",
                    parameters={
                        "minimum_allele_number": 1,
                        "minimum_coverage": 20.0,
                        "required_filter_status": "PASS",
                        "maximum_allele_frequency": 0.0,
                        "applied_strength": "moderate",
                    },
                )
            )
            continue
        criteria.append(
            CriterionSpecification(
                code=code,
                enabled=is_enabled,
                base_strength=(CriterionStrength.STRONG if is_enabled else None),
                allowed_strengths=(CriterionStrength.STRONG,) if is_enabled else (),
                evaluator_id="test" if is_enabled else "disabled",
                evaluator_version_constraint=">=1.0.0,<2.0.0",
                rationale_template=f"{code.value} rationale",
            )
        )
    return RulesetSpecification(
        ruleset_id="workflow-test",
        version="1.0.0",
        state=RulesetState.APPROVED,
        publication_reference="PMID:25741868",
        scope=scope or RulesetScope(),
        criteria=tuple(criteria),
        combination_algorithm_id="acmg_2015",
        combination_algorithm_version="1.0.0",
    )


if __name__ == "__main__":
    unittest.main()
