from __future__ import annotations

import unittest


class StatusAndErrorContractTests(unittest.TestCase):
    def test_workflow_and_criterion_statuses_use_stable_values(self) -> None:
        from acmg_classifier.domain.enums import CriterionStatus, WorkflowStatus

        self.assertEqual(WorkflowStatus.NEEDS_CONTEXT.value, "needs_context")
        self.assertEqual(CriterionStatus.NOT_EVALUABLE.value, "not_evaluable")

    def test_unknown_status_is_rejected(self) -> None:
        from acmg_classifier.domain.enums import WorkflowStatus

        with self.assertRaises(ValueError):
            WorkflowStatus("partially_done")

    def test_identifiers_require_their_stable_prefix(self) -> None:
        from pydantic import TypeAdapter, ValidationError

        from acmg_classifier.domain.identifiers import ClassificationId, RequestId

        self.assertEqual(
            TypeAdapter(RequestId).validate_python("req_0123456789abcdef"),
            "req_0123456789abcdef",
        )
        self.assertEqual(
            TypeAdapter(ClassificationId).validate_python(
                "cls_0123456789abcdef0123456789abcdef"
            ),
            "cls_0123456789abcdef0123456789abcdef",
        )
        with self.assertRaises(ValidationError):
            TypeAdapter(ClassificationId).validate_python("req_0123456789abcdef")

    def test_error_envelope_is_strict_and_machine_readable(self) -> None:
        from pydantic import ValidationError

        from acmg_classifier.domain.errors import ApplicationError, ErrorCode
        from acmg_classifier.presentation.schemas.errors import ErrorEnvelope

        error = ApplicationError(
            code=ErrorCode.SOURCE_RATE_LIMITED,
            message="gnomAD temporarily limited the request.",
            retryable=True,
            source="gnomad",
            next_action="Retry after 30 seconds.",
            details={"retry_after_seconds": 30},
        )

        envelope = ErrorEnvelope.from_error(
            error,
            request_id="req_0123456789abcdef",
        )

        self.assertEqual(
            envelope.model_dump(mode="json"),
            {
                "error": {
                    "code": "SOURCE_RATE_LIMITED",
                    "message": "gnomAD temporarily limited the request.",
                    "retryable": True,
                    "source": "gnomad",
                    "field": None,
                    "details": {"retry_after_seconds": 30},
                    "next_action": "Retry after 30 seconds.",
                    "request_id": "req_0123456789abcdef",
                }
            },
        )
        with self.assertRaises(ValidationError):
            ErrorEnvelope.model_validate(
                {
                    **envelope.model_dump(mode="json"),
                    "debug_trace": "must not cross the boundary",
                }
            )


if __name__ == "__main__":
    unittest.main()
