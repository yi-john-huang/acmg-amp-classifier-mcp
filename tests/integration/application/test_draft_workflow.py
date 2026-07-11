from __future__ import annotations

import unittest
from datetime import UTC, datetime, timedelta
from pathlib import Path
from tempfile import TemporaryDirectory

from acmg_classifier.application.drafts import (
    DraftAnswer,
    DraftAnswerState,
    DraftResumeError,
    WorkflowDraftService,
)
from acmg_classifier.domain.context import (
    ContextField,
    ContextIssueCode,
    ContextQuestion,
)
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.infrastructure.storage.records import SQLiteRecordStore
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore


class WorkflowDraftServiceIntegrationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.database = Path(self.temporary_directory.name) / "state.sqlite3"
        SQLiteStateStore(self.database).initialize()
        self.now = datetime(2026, 7, 11, 13, 0, tzinfo=UTC)
        self.store = SQLiteRecordStore(self.database, clock=lambda: self.now)
        self.service = WorkflowDraftService(
            store=self.store,
            signing_key=b"a" * 32,
            clock=lambda: self.now,
            lifetime=timedelta(hours=1),
        )

    def test_resumes_only_answered_context_without_re_normalizing(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )

        resumed = self.service.resume(
            continuation.resume_token,
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

        self.assertEqual(resumed.draft_id, continuation.draft_id)
        self.assertEqual(resumed.normalized_variant, _normalized())
        self.assertEqual(resumed.request["context"]["disease_id"], "MONDO:0000001")
        self.assertEqual(resumed.request["context"]["disease_label"], "Test disease")
        self.assertTrue(resumed.resume_token.startswith("resume_"))

    def test_progressive_answers_preserve_prior_context_and_answer_states(
        self,
    ) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )
        first = self.service.resume(
            continuation.resume_token,
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
        next_step = self.service.update(
            draft_id=first.draft_id,
            request=first.request,
            normalized_variant=first.normalized_variant,
            questions=(_inheritance_question(),),
            answer_states=first.answer_states,
        )

        completed_context = self.service.resume(
            next_step.resume_token,
            answers=(
                DraftAnswer(
                    field=ContextField.INHERITANCE,
                    state=DraftAnswerState.PROVIDED,
                    value="autosomal_dominant",
                ),
            ),
        )

        self.assertEqual(
            completed_context.request["context"]["disease_id"],
            "MONDO:0000001",
        )
        self.assertEqual(
            completed_context.request["context"]["inheritance"],
            "autosomal_dominant",
        )
        self.assertEqual(
            completed_context.answer_states,
            {"disease": "provided", "inheritance": "provided"},
        )

    def test_accepts_disease_identifier_selected_from_question_choices(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_identifier_question(),),
        )

        resumed = self.service.resume(
            continuation.resume_token,
            answers=(
                DraftAnswer(
                    field=ContextField.DISEASE,
                    state=DraftAnswerState.PROVIDED,
                    value="MONDO:0000001",
                ),
            ),
        )

        self.assertEqual(resumed.request["context"]["disease_id"], "MONDO:0000001")
        self.assertEqual(resumed.request["context"]["disease_label"], "Test disease")

    def test_rejects_answer_outside_question_schema(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_identifier_question(),),
        )

        with self.assertRaisesRegex(DraftResumeError, "DRAFT_ANSWER_INVALID"):
            self.service.resume(
                continuation.resume_token,
                answers=(
                    DraftAnswer(
                        field=ContextField.DISEASE,
                        state=DraftAnswerState.PROVIDED,
                        value="MONDO:9999999",
                    ),
                ),
            )

    def test_rejects_corrupt_persisted_normalized_content_as_a_typed_error(
        self,
    ) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )
        revision = self.store.get_draft(continuation.draft_id).revision
        self.store.update_draft(
            continuation.draft_id,
            {
                "schema_version": "1.0",
                "request": _request(),
                "normalized_variant": {},
                "questions": [_disease_question().to_canonical_content()],
                "answer_states": {},
            },
            expected_revision=revision,
        )

        with self.assertRaisesRegex(DraftResumeError, "DRAFT_CONTENT_INVALID"):
            self.service.resume(continuation.resume_token, answers=())

    def test_unknown_answer_is_retained_without_fabricating_context(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )

        resumed = self.service.resume(
            continuation.resume_token,
            answers=(
                DraftAnswer(
                    field=ContextField.DISEASE,
                    state=DraftAnswerState.UNKNOWN,
                ),
            ),
        )

        self.assertIsNone(resumed.request["context"]["disease_id"])
        self.assertEqual(resumed.answer_states["disease"], "unknown")

    def test_unknown_replaces_previously_provided_context(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )
        answered = self.service.resume(
            continuation.resume_token,
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

        unknown = self.service.resume(
            answered.resume_token,
            answers=(
                DraftAnswer(
                    field=ContextField.DISEASE,
                    state=DraftAnswerState.UNKNOWN,
                ),
            ),
        )

        self.assertIsNone(unknown.request["context"]["disease_id"])
        self.assertIsNone(unknown.request["context"]["disease_label"])
        self.assertEqual(unknown.answer_states["disease"], "unknown")

    def test_rejects_unasked_or_malformed_answers_without_mutating_draft(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )

        with self.assertRaisesRegex(DraftResumeError, "DRAFT_ANSWER_INVALID"):
            self.service.resume(
                continuation.resume_token,
                answers=(
                    DraftAnswer(
                        field=ContextField.TRANSCRIPT,
                        state=DraftAnswerState.PROVIDED,
                        value="NM_000001.1",
                    ),
                ),
            )
        stored = self.store.get_draft(continuation.draft_id)
        self.assertIn(b'"disease_id":null', stored.request_json)

    def test_rejects_tampered_expired_and_completed_tokens(self) -> None:
        continuation = self.service.create(
            request=_request(),
            normalized_variant=_normalized(),
            questions=(_disease_question(),),
        )
        tampered = continuation.resume_token[:-1] + (
            "0" if continuation.resume_token[-1] != "0" else "1"
        )
        with self.assertRaisesRegex(DraftResumeError, "DRAFT_TOKEN_INVALID"):
            self.service.resume(tampered, answers=())

        self.now += timedelta(hours=2)
        with self.assertRaisesRegex(DraftResumeError, "DRAFT_EXPIRED"):
            self.service.resume(continuation.resume_token, answers=())

        self.now -= timedelta(hours=2)
        self.store.finalize_classification(
            {"classification": "uncertain_significance"},
            draft_id=continuation.draft_id,
            expected_draft_revision=0,
        )
        with self.assertRaisesRegex(DraftResumeError, "DRAFT_ALREADY_COMPLETED"):
            self.service.resume(continuation.resume_token, answers=())


def _request() -> dict[str, object]:
    return {
        "variant": "NC_000001.11:g.101A>G",
        "context": {
            "genome_build": "GRCh38",
            "transcript": "NM_000001.1",
            "disease_id": None,
            "disease_label": None,
            "inheritance": None,
        },
        "evidence_policy": {"mode": "live", "max_age_seconds": None},
        "analysis_intent": "germline_mendelian",
        "user_evidence": [],
        "interactive": True,
    }


def _disease_identifier_question() -> ContextQuestion:
    return ContextQuestion(
        field=ContextField.DISEASE,
        code=ContextIssueCode.DISEASE_REQUIRED,
        prompt="Select a disease context.",
        reason="The selected ruleset is disease-specific.",
        answer_schema={"type": "string", "enum": ["MONDO:0000001"]},
        choices=(
            {
                "value": "MONDO:0000001",
                "label": "Test disease",
            },
        ),
    )


def _disease_question() -> ContextQuestion:
    return ContextQuestion(
        field=ContextField.DISEASE,
        code=ContextIssueCode.DISEASE_REQUIRED,
        prompt="Select a disease context.",
        reason="The selected ruleset is disease-specific.",
        answer_schema={"type": "object"},
    )


def _inheritance_question() -> ContextQuestion:
    return ContextQuestion(
        field=ContextField.INHERITANCE,
        code=ContextIssueCode.INHERITANCE_REQUIRED,
        prompt="Select an inheritance context.",
        reason="The selected ruleset is inheritance-specific.",
        answer_schema={"type": "string"},
    )


def _normalized() -> NormalizedVariant:
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
                retrieved_at=datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
                bundle_version="core-test-1",
                raw_snapshot_ref=None,
                query_key=key.value,
            ),
        ),
    )


if __name__ == "__main__":
    unittest.main()
