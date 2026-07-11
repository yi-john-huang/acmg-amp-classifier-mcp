from __future__ import annotations

import json
import unittest
from pathlib import Path

from pydantic import ValidationError

FIXTURES = Path(__file__).parents[2] / "fixtures" / "case_evidence"
VARIANT = "NM_007294.4:c.68_69delAG"


def load_fixture(name: str) -> object:
    return json.loads((FIXTURES / name).read_text())


class CaseEvidenceSchemaTests(unittest.TestCase):
    def test_variant_only_request_preserves_existing_contract(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        request = ClassificationRequest(variant=VARIANT)

        self.assertEqual(request.variant, VARIANT)
        self.assertEqual(request.case_evidence, ())

    def test_each_case_evidence_family_is_strictly_discriminated(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        expected_types = {
            "phenotype.json": "PhenotypeEvidenceInput",
            "segregation.json": "SegregationEvidenceInput",
            "de_novo.json": "DeNovoEvidenceInput",
            "allelic.json": "AllelicEvidenceInput",
            "functional.json": "FunctionalAssayEvidenceInput",
            "case_control.json": "CaseControlEvidenceInput",
        }
        for fixture_name, type_name in expected_types.items():
            with self.subTest(fixture=fixture_name):
                request = ClassificationRequest.model_validate(
                    {"variant": VARIANT, "case_evidence": [load_fixture(fixture_name)]}
                )
                self.assertEqual(type(request.case_evidence[0]).__name__, type_name)

    def test_segregation_counts_cannot_exceed_informative_meioses(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        with self.assertRaises(ValidationError):
            ClassificationRequest.model_validate(
                {
                    "variant": VARIANT,
                    "case_evidence": [
                        {
                            "kind": "segregation",
                            "informative_meioses": 2,
                            "co_segregations": 2,
                            "non_segregations": 1,
                        }
                    ],
                }
            )

    def test_unknown_and_not_applicable_states_are_explicitly_preserved(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        request = ClassificationRequest.model_validate(
            {
                "variant": VARIANT,
                "case_evidence": load_fixture("unknown_not_applicable.json"),
            }
        )

        self.assertEqual(
            [item.state for item in request.case_evidence if item.kind == "phenotype"],
            ["unknown", "not_applicable"],
        )
        self.assertEqual(request.case_evidence[-1].confirmation, "not_applicable")

    def test_contradictory_phenotype_or_de_novo_entries_are_rejected(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        contradictory_requests = (
            {
                "variant": VARIANT,
                "case_evidence": [
                    {"kind": "phenotype", "term_id": "HP:0001250", "state": "present"},
                    {"kind": "phenotype", "term_id": "HP:0001250", "state": "absent"},
                ],
            },
            {
                "variant": VARIANT,
                "case_evidence": [
                    {"kind": "de_novo", "confirmation": "confirmed"},
                    {"kind": "de_novo", "confirmation": "not_applicable"},
                ],
            },
        )

        for payload in contradictory_requests:
            with (
                self.subTest(payload=payload),
                self.assertRaisesRegex(ValidationError, "contradictory"),
            ):
                ClassificationRequest.model_validate(payload)

    def test_phi_and_free_text_fields_are_rejected_by_the_strict_boundary(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        for forbidden_field in ("patient_id", "mrn", "name", "date_of_birth", "note"):
            payload = load_fixture("phenotype.json")
            assert isinstance(payload, dict)
            payload[forbidden_field] = "synthetic-value"
            with (
                self.subTest(forbidden_field=forbidden_field),
                self.assertRaises(ValidationError),
            ):
                ClassificationRequest.model_validate(
                    {"variant": VARIANT, "case_evidence": [payload]}
                )

    def test_citations_reject_local_paths_and_insecure_urls(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        for citation in (
            "/tmp/case.pdf",
            "file:///tmp/case.pdf",
            "http://example.org/a",
            "DOI:not-a-doi",
        ):
            with self.subTest(citation=citation), self.assertRaises(ValidationError):
                ClassificationRequest.model_validate(
                    {
                        "variant": VARIANT,
                        "case_evidence": [
                            {
                                "kind": "phenotype",
                                "term_id": "HP:0001250",
                                "state": "present",
                                "citation_ids": [citation],
                            }
                        ],
                    }
                )

    def test_local_paths_are_rejected_from_structured_identifier_fields(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        invalid_evidence = (
            {
                "kind": "allelic",
                "other_variant_key": "/tmp/other.vcf",
                "phase": "trans",
            },
            {
                "kind": "functional",
                "assay_id": "file:///tmp/assay.csv",
                "assay_type": "DNA repair",
                "result": "abnormal",
                "validation_status": "validated",
            },
            {"kind": "case_control", "study_id": "C:\\case-data.csv"},
        )
        for evidence in invalid_evidence:
            with self.subTest(evidence=evidence), self.assertRaises(ValidationError):
                ClassificationRequest.model_validate(
                    {"variant": VARIANT, "case_evidence": [evidence]}
                )

    def test_source_criterion_and_raw_snapshot_smuggling_are_rejected(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        for forbidden_field in (
            "raw_snapshot_ref",
            "source_id",
            "source_record_id",
            "criterion",
            "criterion_score",
            "classification",
        ):
            with (
                self.subTest(forbidden_field=forbidden_field),
                self.assertRaises(ValidationError),
            ):
                ClassificationRequest.model_validate(
                    {
                        "variant": VARIANT,
                        "case_evidence": [
                            {
                                "kind": "phenotype",
                                "term_id": "HP:0001250",
                                "state": "present",
                                forbidden_field: "smuggled",
                            }
                        ],
                    }
                )

    def test_domain_boundaries_are_enforced_through_case_inputs(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        invalid_evidence = (
            {
                "kind": "functional",
                "assay_id": "assay-1",
                "assay_type": "protein function",
                "result": "abnormal",
                "validation_status": "validated",
                "confidence_interval_lower": 0.6,
                "confidence_interval_upper": 0.2,
            },
            {
                "kind": "case_control",
                "study_id": "PMID:23456789",
                "case_count": 2,
                "case_allele_count": 5,
            },
        )
        for evidence in invalid_evidence:
            with self.subTest(evidence=evidence), self.assertRaises(ValidationError):
                ClassificationRequest.model_validate(
                    {"variant": VARIANT, "case_evidence": [evidence]}
                )
