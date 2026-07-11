"""The linear, adapter-agnostic scientist classification workflow."""

from __future__ import annotations

import asyncio
from collections.abc import Callable, Mapping
from dataclasses import dataclass, field, replace
from datetime import UTC, datetime
from typing import Protocol, cast

from acmg_classifier import __version__
from acmg_classifier.application.context import ContextResolutionService
from acmg_classifier.application.drafts import (
    DraftAnswer,
    DraftResumeError,
    WorkflowDraftService,
)
from acmg_classifier.application.evidence_orchestrator import (
    EvidenceAcquisitionResult,
    EvidenceOrchestrator,
)
from acmg_classifier.application.explanation import Explanation, ExplanationService
from acmg_classifier.application.normalization import VariantNormalizationService
from acmg_classifier.application.review import ReviewPacket, ReviewPacketBuilder
from acmg_classifier.domain.combination import (
    ClassificationCombiner,
    ClassificationDecision,
)
from acmg_classifier.domain.context import ContextQuestion
from acmg_classifier.domain.criteria import CriteriaEngine, CriterionAssessment
from acmg_classifier.domain.enums import (
    AnalysisIntent,
    CriterionStatus,
    GenomeBuild,
    InheritanceMode,
    WorkflowStatus,
)
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import (
    ClinicalAssertionObservation,
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    EvidencePolicy,
    EvidencePolicyMode,
    EvidenceSnapshot,
    FactSet,
    SourceProvenance,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    NormalizationFailureCode,
    NormalizedVariant,
)
from acmg_classifier.domain.rules import (
    CriterionCode,
    RulesetRegistry,
    RulesetSelectionStatus,
    RulesetSpecification,
)
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationPolicy,
    NormalizationSuccess,
)


class ReadinessService(Protocol):
    """The tiny readiness port shared with bootstrap without adapter coupling."""

    def ensure_ready(self) -> object: ...


class ClassificationRecordStore(Protocol):
    """The append-only transaction boundary for completed records."""

    def finalize_classification(self, record: JsonValue, **kwargs: object) -> str: ...

    def get_classification(self, classification_id: str) -> object: ...


class UserEvidencePersistenceError(RuntimeError):
    """Typed user evidence could not be durably joined to a source snapshot."""


class UserEvidenceStore(Protocol):
    """Persistence required only when an application request includes user facts."""

    def put_evidence(self, content: object) -> str: ...

    def put_domain_evidence_snapshot(self, snapshot: EvidenceSnapshot) -> str: ...


@dataclass(frozen=True, slots=True)
class ClassificationRequest:
    """Application request independent of CLI and MCP boundary models."""

    variant: str
    context: InterpretationContext = field(default_factory=InterpretationContext)
    evidence_policy: EvidencePolicy = field(
        default_factory=lambda: EvidencePolicy(mode=EvidencePolicyMode.LIVE)
    )
    analysis_intent: AnalysisIntent = AnalysisIntent.GERMLINE_MENDELIAN
    user_evidence: tuple[EvidenceItem, ...] = ()
    requested_ruleset_id: str | None = None
    previous_classification_id: str | None = None
    interactive: bool = True

    def __post_init__(self) -> None:
        if not self.variant.strip():
            raise ValueError("classification request variant must not be empty")
        if (
            self.previous_classification_id is not None
            and not self.previous_classification_id.strip()
        ):
            raise ValueError("previous classification ID must not be empty")


@dataclass(frozen=True, slots=True)
class ClassificationRecord:
    """Immutable completed-workflow content before persistence assigns its ID."""

    request: ClassificationRequest
    normalized_variant: NormalizedVariant
    context: InterpretationContext
    evidence_snapshot: EvidenceSnapshot
    decision: ClassificationDecision
    explanation: Explanation
    ruleset: RulesetSpecification
    bundle_version: str
    created_at: datetime
    software_version: str = __version__

    def __post_init__(self) -> None:
        if not self.bundle_version:
            raise ValueError("classification record bundle_version must not be empty")
        if self.created_at.tzinfo is None or self.created_at.utcoffset() is None:
            raise ValueError("classification record timestamp must be timezone-aware")

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return the persistence-safe immutable record without its storage ID."""
        return _record_payload(
            request=self.request,
            normalized=self.normalized_variant,
            context=self.context,
            snapshot=self.evidence_snapshot,
            decision=self.decision,
            explanation=self.explanation,
            ruleset=self.ruleset,
            bundle_version=self.bundle_version,
            created_at=self.created_at,
            software_version=self.software_version,
        )


@dataclass(frozen=True, slots=True)
class CompletedClassificationResponse:
    """A durable completed decision and the immutable inputs used to make it."""

    status: WorkflowStatus
    classification_id: str
    normalized_variant: NormalizedVariant
    context: InterpretationContext
    decision: ClassificationDecision
    snapshot_id: str
    ruleset: RulesetSpecification
    record: ClassificationRecord
    explanation: Explanation
    limitations: tuple[str, ...] = ()
    review_packet: ReviewPacket | None = None

    def __post_init__(self) -> None:
        if self.status is not WorkflowStatus.COMPLETED:
            raise ValueError("completed response requires completed status")
        if self.decision.classification is None or self.decision.conflict is not None:
            raise ValueError("completed response requires a five-tier decision")


@dataclass(frozen=True, slots=True)
class NeedsContextClassificationResponse:
    """A normalized request blocked only on explicit interpretation context."""

    status: WorkflowStatus
    normalized_variant: NormalizedVariant
    questions: tuple[ContextQuestion, ...]
    draft_id: str
    resume_token: str
    limitations: tuple[str, ...] = ()
    classification: None = None

    def __post_init__(self) -> None:
        if self.status is not WorkflowStatus.NEEDS_CONTEXT:
            raise ValueError("needs-context response requires needs_context status")
        if not self.questions:
            raise ValueError("needs-context response requires at least one question")
        if not self.draft_id or not self.resume_token:
            raise ValueError("needs-context response requires a draft continuation")


@dataclass(frozen=True, slots=True)
class SourceCriterionImpact:
    """The ruleset-declared criteria made non-evaluable by one source failure."""

    source_id: str
    criterion_codes: tuple[CriterionCode, ...] = ()
    reason: str | None = None

    def __post_init__(self) -> None:
        if not self.source_id:
            raise ValueError("source impact source_id must not be empty")
        codes = tuple(sorted(set(self.criterion_codes), key=lambda code: code.value))
        if codes and self.reason is not None:
            raise ValueError("criterion impact with codes must not include a reason")
        if not codes and not self.reason:
            raise ValueError("empty criterion impact requires an explicit reason")
        object.__setattr__(self, "criterion_codes", codes)

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return stable presentation content without inferring a dependency."""
        return {
            "source_id": self.source_id,
            "criterion_codes": [code.value for code in self.criterion_codes],
            "reason": self.reason,
        }


@dataclass(frozen=True, slots=True)
class DegradedClassificationResponse:
    """A non-completed decision with source-specific availability loss."""

    status: WorkflowStatus
    normalized_variant: NormalizedVariant
    context: InterpretationContext
    decision: ClassificationDecision
    explanation: Explanation
    snapshot_id: str
    unavailable_sources: tuple[str, ...]
    source_impacts: tuple[SourceCriterionImpact, ...]
    limitations: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if self.status is not WorkflowStatus.DEGRADED:
            raise ValueError("degraded response requires degraded status")
        if not self.unavailable_sources:
            raise ValueError("degraded response requires unavailable sources")
        if tuple(sorted(set(self.unavailable_sources))) != self.unavailable_sources:
            raise ValueError("degraded unavailable sources must be sorted and unique")
        if tuple(impact.source_id for impact in self.source_impacts) != (
            self.unavailable_sources
        ):
            raise ValueError(
                "degraded source impacts must cover unavailable sources in order"
            )
        if self.decision.conflict is not None:
            raise ValueError("degraded response cannot conceal a conflict")


@dataclass(frozen=True, slots=True)
class ConflictClassificationResponse:
    """An unresolved conflict that deliberately has no five-tier result."""

    status: WorkflowStatus
    normalized_variant: NormalizedVariant
    context: InterpretationContext
    decision: ClassificationDecision
    explanation: Explanation
    snapshot_id: str
    limitations: tuple[str, ...] = ()
    classification_id: str | None = None
    record: ClassificationRecord | None = None
    review_packet: ReviewPacket | None = None
    classification: None = None

    def __post_init__(self) -> None:
        if self.status is not WorkflowStatus.CONFLICT:
            raise ValueError("conflict response requires conflict status")
        if self.decision.classification is not None or self.decision.conflict is None:
            raise ValueError("conflict response requires an unresolved decision")
        if (self.classification_id is None) != (self.record is None):
            raise ValueError(
                "conflict record and classification ID must be present together"
            )
        if self.review_packet is not None and self.classification_id is None:
            raise ValueError("conflict review packet requires a persisted record")


@dataclass(frozen=True, slots=True)
class UnsupportedClassificationResponse:
    """A request that must not proceed into evidence or criteria evaluation."""

    status: WorkflowStatus
    reason: str
    limitations: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if self.status is not WorkflowStatus.UNSUPPORTED:
            raise ValueError("unsupported response requires unsupported status")
        if not self.reason:
            raise ValueError("unsupported response requires a reason")


@dataclass(frozen=True, slots=True)
class FailedClassificationResponse:
    """A safe typed failure without leaked adapter or storage details."""

    status: WorkflowStatus
    error_code: str
    limitations: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if self.status is not WorkflowStatus.FAILED:
            raise ValueError("failed response requires failed status")
        if not self.error_code:
            raise ValueError("failed response requires an error code")


type ClassificationWorkflowResponse = (
    CompletedClassificationResponse
    | NeedsContextClassificationResponse
    | DegradedClassificationResponse
    | ConflictClassificationResponse
    | UnsupportedClassificationResponse
    | FailedClassificationResponse
)


class ClassificationService:
    """Execute one linear, reproducible workflow without presentation dependencies."""

    def __init__(
        self,
        *,
        normalizer: VariantNormalizationService,
        context_resolver: ContextResolutionService,
        rulesets: RulesetRegistry,
        criteria_engine: CriteriaEngine,
        combiner: ClassificationCombiner,
        evidence_orchestrator: EvidenceOrchestrator,
        record_store: ClassificationRecordStore | None,
        bundle_version: str,
        clock: Callable[[], datetime],
        readiness: ReadinessService | None = None,
        evaluator_versions: Mapping[str, str] | None = None,
        user_evidence_store: UserEvidenceStore | None = None,
        draft_service: WorkflowDraftService | None = None,
        explanation_service: ExplanationService | None = None,
        review_packet_builder: ReviewPacketBuilder | None = None,
    ) -> None:
        if not bundle_version:
            raise ValueError("bundle_version must not be empty")
        self._normalizer = normalizer
        self._context_resolver = context_resolver
        self._rulesets = rulesets
        self._criteria_engine = criteria_engine
        self._combiner = combiner
        self._evidence_orchestrator = evidence_orchestrator
        self._record_store = record_store
        self._user_evidence_store = user_evidence_store
        self._draft_service = draft_service
        self._explanation_service = explanation_service or ExplanationService()
        self._review_packet_builder = review_packet_builder or ReviewPacketBuilder()
        self._bundle_version = bundle_version
        self._clock = clock
        self._readiness = readiness
        registry = getattr(criteria_engine, "registry", None)
        discovered_versions = getattr(registry, "versions", {})
        self._evaluator_versions = dict(
            evaluator_versions
            if evaluator_versions is not None
            else discovered_versions
        )

    async def classify(
        self, request: ClassificationRequest
    ) -> ClassificationWorkflowResponse:
        """Start a classification workflow from an untrusted presentation request."""
        return await self._classify(request)

    async def reinterpret(
        self,
        request: ClassificationRequest,
        *,
        previous_classification_id: str,
    ) -> ClassificationWorkflowResponse:
        """Create a linked new interpretation without changing the old record."""
        if not previous_classification_id:
            return self._failed("PREVIOUS_CLASSIFICATION_NOT_FOUND")
        return await self.classify(
            replace(
                request,
                previous_classification_id=previous_classification_id,
            )
        )

    async def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> ClassificationWorkflowResponse:
        """Resume saved work without re-running normalization providers."""
        if self._draft_service is None:
            return self._failed("DRAFT_SERVICE_UNAVAILABLE")
        try:
            resumed = self._draft_service.resume(resume_token, answers=answers)
            request = _request_from_draft_content(resumed.request)
        except DraftResumeError as error:
            return self._failed(error.code)
        except (TypeError, ValueError):
            return self._failed("DRAFT_CONTENT_INVALID")
        if resumed.normalized_variant.original_input != request.variant:
            return self._failed("DRAFT_CONTENT_INVALID")
        return await self._classify(
            request,
            normalized_override=resumed.normalized_variant,
            draft_id=resumed.draft_id,
            draft_revision=resumed.revision,
            answer_states=resumed.answer_states,
        )

    async def _classify(
        self,
        request: ClassificationRequest,
        *,
        normalized_override: NormalizedVariant | None = None,
        draft_id: str | None = None,
        draft_revision: int | None = None,
        answer_states: Mapping[str, str] | None = None,
    ) -> ClassificationWorkflowResponse:
        """Execute a new or resumed workflow after normalized state is available."""
        if request.previous_classification_id is not None:
            if self._record_store is None:
                return self._failed("CLASSIFICATION_PERSISTENCE_FAILED")
            try:
                self._record_store.get_classification(
                    request.previous_classification_id
                )
            except Exception:
                return self._failed("PREVIOUS_CLASSIFICATION_NOT_FOUND")
        readiness_failure = self._ensure_ready()
        if readiness_failure is not None:
            return readiness_failure
        if normalized_override is None:
            policy = _normalization_policy(request.evidence_policy, request.context)
            normalize_async = getattr(self._normalizer, "normalize_async", None)
            if callable(normalize_async):
                normalization = await normalize_async(
                    request.variant,
                    context=request.context,
                    policy=policy,
                )
            else:
                normalization = self._normalizer.normalize(
                    request.variant,
                    context=request.context,
                    policy=policy,
                )
            if isinstance(normalization, NormalizationFailure):
                return self._normalization_failure(normalization)
            if not isinstance(normalization, NormalizationSuccess):
                return self._failed("NORMALIZATION_RESPONSE_INVALID")
            normalized = normalization.normalized
        else:
            normalized = normalized_override
        context_resolution = self._context_resolver.resolve(normalized, request.context)
        if context_resolution.status == "needs_context":
            return self._needs_context(
                request=request,
                normalized=normalized,
                questions=context_resolution.questions,
                draft_id=draft_id,
                answer_states=answer_states,
                limitation="classification is pending required context",
            )
        if context_resolution.status != "complete":
            return self._failed("INTERPRETATION_CONTEXT_CONFLICT")
        context = context_resolution.resolved
        selection = self._rulesets.select(
            context,
            gene_symbol=normalized.gene_symbol,
            requested_ruleset_id=request.requested_ruleset_id,
            evaluator_versions=self._evaluator_versions,
        )
        if selection.status is RulesetSelectionStatus.NEEDS_CONTEXT:
            questions = _ruleset_questions(selection.required_context_fields)
            if not questions:
                return self._failed("RULESET_CONTEXT_UNRESOLVABLE")
            return self._needs_context(
                request=request,
                normalized=normalized,
                questions=questions,
                draft_id=draft_id,
                answer_states=answer_states,
                limitation="classification is pending required ruleset context",
            )
        if selection.status is not RulesetSelectionStatus.SELECTED:
            return self._failed("NO_COMPATIBLE_RULESET")
        ruleset = selection.ruleset
        if ruleset is None:
            return self._failed("RULESET_SELECTION_INVALID")
        try:
            scope = _evidence_scope(context)
        except ValueError:
            return self._failed("EVIDENCE_CONTEXT_SCOPE_INVALID")
        try:
            acquired = await self._evidence_orchestrator.gather(
                normalized,
                scope,
                request.evidence_policy,
            )
        except asyncio.CancelledError:
            raise
        except Exception:
            return self._failed("EVIDENCE_ACQUISITION_FAILED")
        try:
            acquired = self._with_user_evidence(
                acquired,
                request.user_evidence,
                variant_key=normalized.variant_key,
                context_scope=scope,
            )
        except ValueError:
            return self._failed("USER_EVIDENCE_INVALID")
        except UserEvidencePersistenceError:
            return self._failed("USER_EVIDENCE_PERSISTENCE_FAILED")
        try:
            facts = FactSet.from_evidence(acquired.evidence_items)
            assessments = self._criteria_engine.evaluate(facts, context, ruleset)
            assessments = _mark_unavailable_source_dependencies(
                assessments,
                ruleset,
                acquired.unavailable_sources,
            )
            decision = self._combiner.combine(
                assessments,
                ruleset,
                source_conflicts=_source_conflicts(acquired.evidence_items),
            )
        except asyncio.CancelledError:
            raise
        except Exception:
            return self._failed("CRITERIA_EVALUATION_FAILED")
        limitations = _limitations(decision, acquired)
        snapshot_id = _snapshot_id(acquired)
        try:
            explanation = self._explanation_service.render_decision(decision)
        except ValueError:
            return self._failed("EXPLANATION_RENDERING_FAILED")
        if decision.conflict is not None:
            if self._record_store is None:
                return ConflictClassificationResponse(
                    status=WorkflowStatus.CONFLICT,
                    normalized_variant=normalized,
                    context=context,
                    decision=decision,
                    explanation=explanation,
                    snapshot_id=snapshot_id,
                    limitations=limitations,
                )
            try:
                record = ClassificationRecord(
                    request=request,
                    normalized_variant=normalized,
                    context=context,
                    evidence_snapshot=acquired.snapshot,
                    decision=decision,
                    explanation=explanation,
                    ruleset=ruleset,
                    bundle_version=self._bundle_version,
                    created_at=self._clock(),
                )
                conflict_finalization_kwargs: dict[str, object] = {}
                if draft_id is not None:
                    if draft_revision is None:
                        raise RuntimeError("resumed draft revision is unavailable")
                    conflict_finalization_kwargs["expected_draft_revision"] = (
                        draft_revision
                    )
                    conflict_finalization_kwargs["draft_id"] = draft_id
                if request.previous_classification_id is not None:
                    conflict_finalization_kwargs["previous_classification_id"] = (
                        request.previous_classification_id
                    )
                classification_id = self._record_store.finalize_classification(
                    record.to_canonical_content(),
                    **conflict_finalization_kwargs,
                )
                if not classification_id:
                    raise RuntimeError(
                        "record store returned an empty classification ID"
                    )
            except asyncio.CancelledError:
                raise
            except Exception:
                return self._failed("CLASSIFICATION_PERSISTENCE_FAILED")
            return ConflictClassificationResponse(
                status=WorkflowStatus.CONFLICT,
                normalized_variant=normalized,
                context=context,
                decision=decision,
                explanation=explanation,
                snapshot_id=snapshot_id,
                limitations=limitations,
                classification_id=classification_id,
                record=record,
                review_packet=self._build_review_packet(
                    classification_id=classification_id,
                    record=record,
                    decision=decision,
                    evidence_items=acquired.evidence_items,
                    snapshot_id=snapshot_id,
                    ruleset=ruleset,
                ),
            )
        if acquired.degraded:
            return DegradedClassificationResponse(
                status=WorkflowStatus.DEGRADED,
                normalized_variant=normalized,
                context=context,
                decision=decision,
                explanation=explanation,
                snapshot_id=snapshot_id,
                unavailable_sources=acquired.unavailable_sources,
                source_impacts=_source_impacts(
                    ruleset,
                    acquired.unavailable_sources,
                ),
                limitations=limitations,
            )
        if self._record_store is None:
            return self._failed("CLASSIFICATION_PERSISTENCE_FAILED")
        try:
            record = ClassificationRecord(
                request=request,
                normalized_variant=normalized,
                context=context,
                evidence_snapshot=acquired.snapshot,
                decision=decision,
                explanation=explanation,
                ruleset=ruleset,
                bundle_version=self._bundle_version,
                created_at=self._clock(),
            )
            finalization_kwargs: dict[str, object] = {}
            if draft_id is not None:
                if draft_revision is None:
                    raise RuntimeError("resumed draft revision is unavailable")
                finalization_kwargs["expected_draft_revision"] = draft_revision
                finalization_kwargs["draft_id"] = draft_id
            if request.previous_classification_id is not None:
                finalization_kwargs["previous_classification_id"] = (
                    request.previous_classification_id
                )
            classification_id = self._record_store.finalize_classification(
                record.to_canonical_content(),
                **finalization_kwargs,
            )
            if not classification_id:
                raise RuntimeError("record store returned an empty classification ID")
        except asyncio.CancelledError:
            raise
        except Exception:
            return self._failed("CLASSIFICATION_PERSISTENCE_FAILED")
        return CompletedClassificationResponse(
            status=WorkflowStatus.COMPLETED,
            classification_id=classification_id,
            normalized_variant=normalized,
            context=context,
            decision=decision,
            snapshot_id=snapshot_id,
            ruleset=ruleset,
            record=record,
            explanation=explanation,
            limitations=limitations,
            review_packet=self._build_review_packet(
                classification_id=classification_id,
                record=record,
                decision=decision,
                evidence_items=acquired.evidence_items,
                snapshot_id=snapshot_id,
                ruleset=ruleset,
            ),
        )

    def _build_review_packet(
        self,
        *,
        classification_id: str,
        record: ClassificationRecord,
        decision: ClassificationDecision,
        evidence_items: tuple[EvidenceItem, ...],
        snapshot_id: str,
        ruleset: RulesetSpecification,
    ) -> ReviewPacket | None:
        """Build optional review context without affecting the durable decision."""
        try:
            request = record.to_canonical_content()["request"]
            if not isinstance(request, dict):
                return None
            return self._review_packet_builder.build(
                classification_id=classification_id,
                decision=decision,
                evidence_items=evidence_items,
                request=request,
                snapshot_id=snapshot_id,
                ruleset_id=ruleset.ruleset_id,
                ruleset_version=ruleset.version,
            )
        except (TypeError, ValueError):
            return None

    def _needs_context(
        self,
        *,
        request: ClassificationRequest,
        normalized: NormalizedVariant,
        questions: tuple[ContextQuestion, ...],
        draft_id: str | None,
        answer_states: Mapping[str, str] | None,
        limitation: str,
    ) -> NeedsContextClassificationResponse | FailedClassificationResponse:
        """Persist the exact pending question state before reporting it."""
        if not questions:
            return self._failed("DRAFT_CONTEXT_UNRESOLVABLE")
        if self._draft_service is None:
            return self._failed("DRAFT_SERVICE_UNAVAILABLE")
        pending_questions = questions[:1] if request.interactive else questions
        request_content = _draft_request_content(request)
        try:
            if draft_id is None:
                continuation = self._draft_service.create(
                    request=request_content,
                    normalized_variant=normalized,
                    questions=pending_questions,
                )
            else:
                continuation = self._draft_service.update(
                    draft_id=draft_id,
                    request=request_content,
                    normalized_variant=normalized,
                    questions=pending_questions,
                    answer_states=answer_states or {},
                )
        except DraftResumeError as error:
            return self._failed(error.code)
        except (TypeError, ValueError):
            return self._failed("DRAFT_PERSISTENCE_FAILED")
        return NeedsContextClassificationResponse(
            status=WorkflowStatus.NEEDS_CONTEXT,
            normalized_variant=normalized,
            questions=pending_questions,
            draft_id=continuation.draft_id,
            resume_token=continuation.resume_token,
            limitations=(limitation,),
        )

    def _with_user_evidence(
        self,
        acquired: EvidenceAcquisitionResult,
        user_evidence: tuple[EvidenceItem, ...],
        *,
        variant_key: str,
        context_scope: EvidenceContextScope,
    ) -> EvidenceAcquisitionResult:
        if not user_evidence:
            return acquired
        if self._user_evidence_store is None:
            raise UserEvidencePersistenceError("no user-evidence store is configured")
        merged: dict[str, EvidenceItem] = {}
        for item in acquired.evidence_items:
            if item.evidence_id is None:
                raise ValueError("acquired evidence item has no evidence ID")
            merged[item.evidence_id] = item
        for item in user_evidence:
            if item.derivation is not EvidenceDerivation.USER:
                raise ValueError("workflow request accepts user-derived evidence only")
            item.assert_applies_to(
                variant_key=variant_key,
                context_scope=context_scope,
            )
            if item.evidence_id is None:
                raise ValueError("user evidence item has no evidence ID")
            existing = merged.get(item.evidence_id)
            if existing is not None and existing != item:
                raise ValueError("user evidence ID conflicts with acquired evidence")
            merged[item.evidence_id] = item
        for item in user_evidence:
            if item.evidence_id is None:
                raise ValueError("user evidence item has no evidence ID")
            try:
                stored_id = self._user_evidence_store.put_evidence(
                    item.canonical_content()
                )
            except Exception as error:
                raise UserEvidencePersistenceError(
                    "user evidence could not be persisted"
                ) from error
            if stored_id != item.evidence_id:
                raise UserEvidencePersistenceError(
                    "user evidence store returned a mismatched evidence ID"
                )
        merged_items = tuple(merged[evidence_id] for evidence_id in sorted(merged))
        snapshot = EvidenceSnapshot(
            evidence_ids=tuple(merged),
            source_statuses=acquired.snapshot.source_statuses,
            policy=acquired.snapshot.policy,
            created_at=acquired.snapshot.created_at,
        )
        try:
            stored_snapshot_id = self._user_evidence_store.put_domain_evidence_snapshot(
                snapshot
            )
        except Exception as error:
            raise UserEvidencePersistenceError(
                "combined evidence snapshot could not be persisted"
            ) from error
        if stored_snapshot_id != snapshot.snapshot_id:
            raise UserEvidencePersistenceError(
                "user evidence store returned a mismatched snapshot ID"
            )
        return replace(acquired, snapshot=snapshot, evidence_items=merged_items)

    def _ensure_ready(self) -> FailedClassificationResponse | None:
        if self._readiness is None:
            return None
        try:
            report = self._readiness.ensure_ready()
        except Exception:
            return self._failed("BOOTSTRAP_FAILED")
        if not getattr(report, "ready", False):
            return self._failed("BOOTSTRAP_NOT_READY")
        return None

    @staticmethod
    def _normalization_failure(
        failure: NormalizationFailure,
    ) -> UnsupportedClassificationResponse | FailedClassificationResponse:
        if failure.code is NormalizationFailureCode.UNSUPPORTED_VARIANT_SCOPE:
            return UnsupportedClassificationResponse(
                status=WorkflowStatus.UNSUPPORTED,
                reason=failure.message,
                limitations=(
                    "release-one scope supports germline small variants only",
                ),
            )
        return FailedClassificationResponse(
            status=WorkflowStatus.FAILED,
            error_code=failure.code.value,
            limitations=(failure.message,),
        )

    @staticmethod
    def _failed(error_code: str) -> FailedClassificationResponse:
        return FailedClassificationResponse(
            status=WorkflowStatus.FAILED,
            error_code=error_code,
        )


def _draft_request_content(
    request: ClassificationRequest,
) -> dict[str, JsonValue]:
    """Encode only application-owned request state for resumable draft storage."""
    context = request.context
    return {
        "variant": request.variant,
        "context": {
            "genome_build": (
                context.genome_build.value if context.genome_build is not None else None
            ),
            "transcript": context.transcript,
            "disease_id": context.disease_id,
            "disease_label": context.disease_label,
            "inheritance": (
                context.inheritance.value if context.inheritance is not None else None
            ),
        },
        "evidence_policy": cast(
            JsonValue,
            request.evidence_policy.model_dump(mode="json"),
        ),
        "analysis_intent": request.analysis_intent.value,
        "user_evidence": [
            cast(JsonValue, item.model_dump(mode="json"))
            for item in request.user_evidence
        ],
        "requested_ruleset_id": request.requested_ruleset_id,
        "previous_classification_id": request.previous_classification_id,
        "interactive": request.interactive,
    }


def _request_from_draft_content(
    content: Mapping[str, JsonValue],
) -> ClassificationRequest:
    """Reconstruct a boundary-safe request from signed canonical draft content."""
    try:
        variant = _required_text(content, "variant")
        context_content = _required_mapping(content, "context")
        genome_build_value = _optional_text(context_content, "genome_build")
        inheritance_value = _optional_text(context_content, "inheritance")
        evidence_policy_content = _required_mapping(content, "evidence_policy")
        user_evidence_content = content.get("user_evidence")
        if not isinstance(user_evidence_content, list):
            raise ValueError("draft user evidence must be a list")
        user_evidence = tuple(
            EvidenceItem.model_validate(_required_json_mapping(item))
            for item in user_evidence_content
        )
        interactive = content.get("interactive")
        if not isinstance(interactive, bool):
            raise ValueError("draft interactive flag must be boolean")
        return ClassificationRequest(
            variant=variant,
            context=InterpretationContext(
                genome_build=(
                    GenomeBuild(genome_build_value)
                    if genome_build_value is not None
                    else None
                ),
                transcript=_optional_text(context_content, "transcript"),
                disease_id=_optional_text(context_content, "disease_id"),
                disease_label=_optional_text(context_content, "disease_label"),
                inheritance=(
                    InheritanceMode(inheritance_value)
                    if inheritance_value is not None
                    else None
                ),
            ),
            evidence_policy=EvidencePolicy.model_validate(evidence_policy_content),
            analysis_intent=AnalysisIntent(_required_text(content, "analysis_intent")),
            user_evidence=user_evidence,
            requested_ruleset_id=_optional_text(content, "requested_ruleset_id"),
            previous_classification_id=_optional_text(
                content,
                "previous_classification_id",
            ),
            interactive=interactive,
        )
    except (KeyError, TypeError, ValueError) as error:
        raise ValueError("invalid stored draft request") from error


def _required_mapping(
    content: Mapping[str, JsonValue],
    key: str,
) -> Mapping[str, JsonValue]:
    value = content.get(key)
    if not isinstance(value, dict):
        raise ValueError(f"draft {key} must be an object")
    return value


def _required_json_mapping(value: JsonValue) -> Mapping[str, JsonValue]:
    if not isinstance(value, dict):
        raise ValueError("draft user evidence item must be an object")
    return value


def _required_text(content: Mapping[str, JsonValue], key: str) -> str:
    value = content.get(key)
    if not isinstance(value, str) or not value:
        raise ValueError(f"draft {key} must be non-empty text")
    return value


def _optional_text(content: Mapping[str, JsonValue], key: str) -> str | None:
    value = content.get(key)
    if value is not None and not isinstance(value, str):
        raise ValueError(f"draft {key} must be text or null")
    return value


def _evidence_scope(context: InterpretationContext) -> EvidenceContextScope:
    return EvidenceContextScope(
        genome_build=context.genome_build,
        transcript=context.transcript,
        disease_id=context.disease_id,
        inheritance=context.inheritance,
    )


def _ruleset_questions(fields: tuple[str, ...]) -> tuple[ContextQuestion, ...]:
    from acmg_classifier.domain.context import ContextField, ContextIssueCode

    field_aliases = {
        "genome_build": ContextField.GENOME_BUILD,
        "transcript": ContextField.TRANSCRIPT,
        "disease_id": ContextField.DISEASE,
        "inheritance": ContextField.INHERITANCE,
    }
    questions: list[ContextQuestion] = []
    for field_name in fields:
        context_field = field_aliases.get(field_name)
        if context_field is None:
            continue
        issue_code = {
            ContextField.DISEASE: ContextIssueCode.DISEASE_REQUIRED,
            ContextField.INHERITANCE: ContextIssueCode.INHERITANCE_REQUIRED,
            ContextField.TRANSCRIPT: ContextIssueCode.TRANSCRIPT_NOT_FOUND,
        }.get(context_field, ContextIssueCode.CONTEXT_CONFLICT)
        questions.append(
            ContextQuestion(
                field=context_field,
                code=issue_code,
                prompt=f"Provide {field_name.replace('_', ' ')}.",
                reason="The selected ruleset requires this interpretation context.",
                answer_schema={"type": "string", "minLength": 1},
            )
        )
    return tuple(questions)


def _source_conflicts(items: tuple[EvidenceItem, ...]) -> dict[str, tuple[str, ...]]:
    directions: dict[str, dict[str, list[str]]] = {}
    for item in items:
        observation = item.observation
        provenance = item.provenance
        if not isinstance(observation, ClinicalAssertionObservation) or not isinstance(
            provenance, SourceProvenance
        ):
            continue
        direction = _assertion_direction(observation.clinical_significance)
        if direction is None or item.evidence_id is None:
            continue
        by_direction = directions.setdefault(
            provenance.source_id,
            {"pathogenic": [], "benign": []},
        )
        by_direction[direction].append(item.evidence_id)
    return {
        source_id: tuple(sorted(values["pathogenic"] + values["benign"]))
        for source_id, values in directions.items()
        if values["pathogenic"] and values["benign"]
    }


def _assertion_direction(significance: str) -> str | None:
    normalized = significance.strip().lower().replace("_", " ").replace("-", " ")
    if "benign" in normalized:
        return "benign"
    if "pathogenic" in normalized:
        return "pathogenic"
    return None


def _normalization_policy(
    evidence_policy: EvidencePolicy,
    context: InterpretationContext,
) -> NormalizationPolicy:
    """Bridge evidence mode to provider eligibility before normalization begins."""
    offline = evidence_policy.mode is EvidencePolicyMode.OFFLINE
    return NormalizationPolicy(
        mode="offline" if offline else "live",
        allow_remote=not offline,
        genome_build=context.genome_build,
    )


def _source_impacts(
    ruleset: RulesetSpecification,
    unavailable_sources: tuple[str, ...],
) -> tuple[SourceCriterionImpact, ...]:
    dependencies = _required_source_dependencies(ruleset)
    return tuple(
        SourceCriterionImpact(
            source_id=source_id,
            criterion_codes=tuple(
                sorted(
                    (
                        code
                        for code, required_sources in dependencies.items()
                        if source_id in required_sources
                    ),
                    key=lambda code: code.value,
                )
            ),
            reason=(
                None
                if any(
                    source_id in required_sources
                    for required_sources in dependencies.values()
                )
                else "no enabled criteria declare this source dependency"
            ),
        )
        for source_id in unavailable_sources
    )


def _mark_unavailable_source_dependencies(
    assessments: Mapping[CriterionCode, CriterionAssessment],
    ruleset: RulesetSpecification,
    unavailable_sources: tuple[str, ...],
) -> dict[CriterionCode, CriterionAssessment]:
    """Prevent declared source failures from becoming absence-based conclusions."""
    unavailable = frozenset(unavailable_sources)
    dependencies = _required_source_dependencies(ruleset)
    marked: dict[CriterionCode, CriterionAssessment] = {}
    for code, assessment in assessments.items():
        missing_sources = tuple(
            source_id
            for source_id in dependencies.get(code, ())
            if source_id in unavailable
        )
        if not missing_sources:
            marked[code] = assessment
            continue
        limitations = tuple(
            sorted(
                set(assessment.limitations).union(
                    f"source unavailable: {source_id}" for source_id in missing_sources
                )
            )
        )
        marked[code] = assessment.model_copy(
            update={
                "status": CriterionStatus.NOT_EVALUABLE,
                "applied_strength": None,
                "evidence_ids": (),
                "comparisons": (),
                "rationale_template": "required source unavailable",
                "rationale_values": {"source_ids": list(missing_sources)},
                "limitations": limitations,
            }
        )
    return marked


def _required_source_dependencies(
    ruleset: RulesetSpecification,
) -> dict[CriterionCode, tuple[str, ...]]:
    """Read only explicit ruleset declarations; unknown sources imply no impact."""
    dependencies: dict[CriterionCode, tuple[str, ...]] = {}
    for specification in ruleset.criteria:
        if not specification.enabled:
            continue
        sources = _declared_required_sources(
            specification.parameters.get("required_source_ids")
        )
        if sources:
            dependencies[specification.code] = sources
    return dependencies


def _declared_required_sources(value: object) -> tuple[str, ...]:
    if not isinstance(value, (list, tuple)):
        return ()
    source_ids: list[str] = []
    for source_id in value:
        if (
            not isinstance(source_id, str)
            or not source_id
            or source_id.strip() != source_id
        ):
            return ()
        source_ids.append(source_id)
    return tuple(sorted(set(source_ids)))


def _limitations(
    decision: ClassificationDecision,
    acquired: EvidenceAcquisitionResult,
) -> tuple[str, ...]:
    unavailable = tuple(
        f"source unavailable: {source_id}" for source_id in acquired.unavailable_sources
    )
    return tuple(sorted(set(decision.limitations + unavailable)))


def _snapshot_id(acquired: EvidenceAcquisitionResult) -> str:
    return _snapshot_id_for(acquired.snapshot)


def _snapshot_id_for(snapshot: EvidenceSnapshot) -> str:
    snapshot_id = snapshot.snapshot_id
    if snapshot_id is None:
        raise RuntimeError("evidence acquisition returned an unassigned snapshot ID")
    return snapshot_id


def _record_payload(
    *,
    request: ClassificationRequest,
    normalized: NormalizedVariant,
    context: InterpretationContext,
    snapshot: EvidenceSnapshot,
    decision: ClassificationDecision,
    explanation: Explanation,
    ruleset: RulesetSpecification,
    bundle_version: str,
    created_at: datetime,
    software_version: str,
) -> dict[str, JsonValue]:
    if created_at.tzinfo is None or created_at.utcoffset() is None:
        raise ValueError("classification clock must return a timezone-aware datetime")
    snapshot_id = _snapshot_id_for(snapshot)
    return {
        "schema_version": "1.0",
        "request": {
            "variant": request.variant,
            "context": _context_content(request.context),
            "evidence_policy": request.evidence_policy.model_dump(mode="json"),
            "analysis_intent": request.analysis_intent.value,
            "user_evidence_ids": _user_evidence_ids(request.user_evidence),
            "requested_ruleset_id": request.requested_ruleset_id,
            "previous_classification_id": request.previous_classification_id,
        },
        "normalized_variant": normalized.to_canonical_content(),
        "context": _context_content(context),
        "evidence_snapshot": {
            "snapshot_id": snapshot_id,
            "content": cast(dict[str, JsonValue], snapshot.canonical_content()),
        },
        "decision": decision.to_canonical_content(),
        "explanation": explanation.to_canonical_content(),
        "ruleset": {
            "id": ruleset.ruleset_id,
            "version": ruleset.version,
            "combination_algorithm_id": ruleset.combination_algorithm_id,
            "combination_algorithm_version": ruleset.combination_algorithm_version,
        },
        "bundle_version": bundle_version,
        "software_version": software_version,
        "created_at": created_at.astimezone(UTC).isoformat(),
    }


def _user_evidence_ids(items: tuple[EvidenceItem, ...]) -> list[JsonValue]:
    values: list[JsonValue] = []
    values.extend(
        sorted({item.evidence_id for item in items if item.evidence_id is not None})
    )
    return values


def _context_content(context: InterpretationContext) -> dict[str, JsonValue]:
    return {
        "genome_build": (
            context.genome_build.value if context.genome_build is not None else None
        ),
        "transcript": context.transcript,
        "disease_id": context.disease_id,
        "disease_label": context.disease_label,
        "inheritance": (
            context.inheritance.value if context.inheritance is not None else None
        ),
    }
