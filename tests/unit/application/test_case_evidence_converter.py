from __future__ import annotations

import json
import unittest
from datetime import UTC, datetime
from pathlib import Path

FIXTURES = Path(__file__).parents[2] / "fixtures" / "case_evidence"
VARIANT_KEY = "ga4gh:VA.example"


def load_fixture(name: str) -> dict[str, object]:
    return json.loads((FIXTURES / name).read_text())


class CaseEvidenceConverterTests(unittest.TestCase):
    @staticmethod
    def submission_context() -> object:
        from acmg_classifier.application.case_evidence import (
            CaseEvidenceSubmissionContext,
        )
        from acmg_classifier.domain.evidence import EvidenceContextScope

        return CaseEvidenceSubmissionContext(
            actor_id="usr_researcher_001",
            submitted_at=datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
            confirmation_method="attested",
            variant_key=VARIANT_KEY,
            context_scope=EvidenceContextScope(
                genome_build="GRCh38",
                transcript="NM_007294.4",
                disease_id="MONDO:0011450",
                inheritance="autosomal_dominant",
            ),
        )

    @staticmethod
    def parsed_evidence(*fixture_names: str) -> tuple[object, ...]:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        request = ClassificationRequest.model_validate(
            {
                "variant": "NM_007294.4:c.68_69delAG",
                "case_evidence": [load_fixture(name) for name in fixture_names],
            }
        )
        return request.case_evidence

    def test_each_family_converts_to_matching_immutable_user_evidence(self) -> None:
        from acmg_classifier.application.case_evidence import CaseEvidenceConverter

        converter = CaseEvidenceConverter()
        context = self.submission_context()
        fixture_names = (
            "phenotype.json",
            "segregation.json",
            "de_novo.json",
            "allelic.json",
            "functional.json",
            "case_control.json",
        )
        expected_kinds = {
            "phenotype.json": "phenotype",
            "segregation.json": "segregation",
            "de_novo.json": "de_novo",
            "allelic.json": "allelic",
            "functional.json": "functional",
            "case_control.json": "case_control",
        }

        for fixture_name in fixture_names:
            with self.subTest(fixture=fixture_name):
                items = converter.convert(self.parsed_evidence(fixture_name), context)
                self.assertEqual(len(items), 1)
                item = items[0]
                self.assertEqual(item.kind.value, expected_kinds[fixture_name])
                self.assertEqual(item.derivation.value, "user")
                self.assertEqual(item.variant_key, VARIANT_KEY)
                self.assertIsNone(item.raw_snapshot_ref)
                self.assertEqual(item.provenance.actor_id, "usr_researcher_001")
                self.assertEqual(
                    item.provenance.submitted_at,
                    datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                )
                self.assertEqual(item.provenance.confirmation_method, "attested")
                self.assertTrue(item.evidence_id.startswith("ev_"))

    def test_unknown_and_not_applicable_are_preserved_as_observations(self) -> None:
        from acmg_classifier.application.case_evidence import CaseEvidenceConverter
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        request = ClassificationRequest.model_validate(
            {
                "variant": "NM_007294.4:c.68_69delAG",
                "case_evidence": json.loads(
                    (FIXTURES / "unknown_not_applicable.json").read_text()
                ),
            }
        )

        items = CaseEvidenceConverter().convert(
            request.case_evidence, self.submission_context()
        )

        states = [
            item.observation.state for item in items if item.kind.value == "phenotype"
        ]
        confirmations = [
            item.observation.confirmation
            for item in items
            if item.kind.value == "de_novo"
        ]
        self.assertEqual(states, ["not_applicable", "unknown"])
        self.assertEqual(confirmations, ["not_applicable"])

    def test_conversion_order_and_ids_are_deterministic(self) -> None:
        from acmg_classifier.application.case_evidence import CaseEvidenceConverter

        evidence = self.parsed_evidence(
            "phenotype.json", "functional.json", "case_control.json"
        )
        converter = CaseEvidenceConverter()

        ordered = converter.convert(evidence, self.submission_context())
        reordered = converter.convert(
            tuple(reversed(evidence)), self.submission_context()
        )

        self.assertEqual(
            tuple(item.evidence_id for item in ordered),
            tuple(item.evidence_id for item in reordered),
        )
        self.assertEqual(
            tuple(item.evidence_id for item in ordered),
            tuple(sorted(item.evidence_id for item in ordered)),
        )

    def test_converter_rechecks_conflicts_when_called_outside_request_parsing(
        self,
    ) -> None:
        from acmg_classifier.application.case_evidence import CaseEvidenceConverter
        from acmg_classifier.presentation.schemas.case_evidence import (
            PhenotypeEvidenceInput,
        )

        evidence = (
            PhenotypeEvidenceInput(
                kind="phenotype", term_id="HP:0001250", state="present"
            ),
            PhenotypeEvidenceInput(
                kind="phenotype", term_id="HP:0001250", state="absent"
            ),
        )

        with self.assertRaisesRegex(ValueError, "contradictory"):
            CaseEvidenceConverter().convert(evidence, self.submission_context())

    def test_submission_context_rejects_invalid_actor_and_naive_timestamp(self) -> None:
        from pydantic import ValidationError

        from acmg_classifier.application.case_evidence import (
            CaseEvidenceSubmissionContext,
        )
        from acmg_classifier.domain.evidence import EvidenceContextScope

        base = {
            "submitted_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
            "confirmation_method": "attested",
            "variant_key": VARIANT_KEY,
            "context_scope": EvidenceContextScope(disease_id="MONDO:0011450"),
        }
        with self.assertRaises(ValidationError):
            CaseEvidenceSubmissionContext(actor_id="patient-123", **base)
        with self.assertRaises(ValidationError):
            CaseEvidenceSubmissionContext(
                actor_id="usr_researcher_001",
                submitted_at=datetime(2026, 7, 11, 9, 0),
                confirmation_method="attested",
                variant_key=VARIANT_KEY,
                context_scope=base["context_scope"],
            )
