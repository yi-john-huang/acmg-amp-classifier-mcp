from __future__ import annotations

import unittest

from pydantic import TypeAdapter, ValidationError


class ClassificationContractTests(unittest.TestCase):
    def test_variant_is_the_only_required_request_field(self) -> None:
        from acmg_classifier.domain.enums import AnalysisIntent, DetailLevel
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        request = ClassificationRequest(
            variant="NM_007294.4:c.5266dupC",
        )

        self.assertEqual(request.variant, "NM_007294.4:c.5266dupC")
        self.assertEqual(request.analysis_intent, AnalysisIntent.GERMLINE_MENDELIAN)
        self.assertEqual(request.detail_level, DetailLevel.STANDARD)
        self.assertIsNone(request.disease)

    def test_full_context_is_strict_and_converts_to_domain(self) -> None:
        from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
        from acmg_classifier.domain.models import InterpretationContext, VariantInput
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        request = ClassificationRequest(
            variant="NM_007294.4:c.5266dupC",
            genome_build="GRCh38",
            transcript="NM_007294.4",
            disease={
                "identifier": "MONDO:0011450",
                "label": "hereditary breast cancer",
            },
            inheritance="autosomal_dominant",
        )

        variant, context = request.to_domain()

        self.assertEqual(variant, VariantInput("NM_007294.4:c.5266dupC"))
        self.assertEqual(
            context,
            InterpretationContext(
                genome_build=GenomeBuild.GRCH38,
                transcript="NM_007294.4",
                disease_id="MONDO:0011450",
                disease_label="hereditary breast cancer",
                inheritance=InheritanceMode.AUTOSOMAL_DOMINANT,
            ),
        )

    def test_unknown_or_privacy_fields_and_invalid_accessions_are_rejected(
        self,
    ) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationRequest,
        )

        invalid_requests = (
            {"variant": "NM_007294.4:c.5266dupC", "patient_id": "MRN-123"},
            {"variant": "NM_007294.4:c.5266dupC", "transcript": "NM_007294"},
            {"variant": "NM_007294.4:c.5266dupC", "genome_build": "hg19"},
            {"variant": ""},
        )
        for invalid_request in invalid_requests:
            with (
                self.subTest(invalid_request=invalid_request),
                self.assertRaises(ValidationError),
            ):
                ClassificationRequest.model_validate(invalid_request)

    def test_response_union_requires_fields_for_each_status(self) -> None:
        from acmg_classifier.presentation.schemas.classification import (
            ClassificationResponse,
        )

        adapter = TypeAdapter(ClassificationResponse)
        cases = {
            "completed": {
                "status": "completed",
                "classification_id": "cls_0123456789abcdef0123456789abcdef",
                "classification": "pathogenic",
            },
            "needs_context": {
                "status": "needs_context",
                "draft_id": "draft_0123456789abcdef",
                "resume_token": "resume_0123456789abcdef0123456789abcdef",
                "questions": [
                    {
                        "field": "disease",
                        "prompt": "Which disease is being evaluated?",
                        "reason": "Classification is disease-specific.",
                    }
                ],
            },
            "degraded": {
                "status": "degraded",
                "classification": None,
                "unavailable_sources": ["gnomad"],
            },
            "conflict": {
                "status": "conflict",
                "conflicting_criteria": ["PS3", "BS3"],
            },
            "unsupported": {
                "status": "unsupported",
                "reason": "structural variants are outside release 1",
            },
            "failed": {
                "status": "failed",
                "error_code": "INTERNAL_ERROR",
            },
        }

        for expected_status, response in cases.items():
            with self.subTest(status=expected_status):
                parsed = adapter.validate_python(response)
                self.assertEqual(parsed.status.value, expected_status)

        with self.assertRaises(ValidationError):
            adapter.validate_python(
                {"status": "completed", "classification": "pathogenic"}
            )


if __name__ == "__main__":
    unittest.main()
