from __future__ import annotations

import math
import unittest
from datetime import UTC, datetime, timedelta, timezone


class CanonicalSerializationTests(unittest.TestCase):
    def test_mapping_order_does_not_change_bytes_or_hash(self) -> None:
        from acmg_classifier.domain.canonical import (
            canonical_hash,
            canonical_json_bytes,
        )

        first = {"variant": "NC_000017.11:43071077:G:A", "build": "GRCh38"}
        second = {"build": "GRCh38", "variant": "NC_000017.11:43071077:G:A"}

        self.assertEqual(canonical_json_bytes(first), canonical_json_bytes(second))
        self.assertEqual(canonical_hash(first), canonical_hash(second))

    def test_unordered_sets_are_sorted_but_lists_preserve_meaningful_order(
        self,
    ) -> None:
        from acmg_classifier.domain.canonical import canonical_json_bytes

        unordered = {"evidence_ids": {"ev_b", "ev_a"}}

        self.assertEqual(
            canonical_json_bytes(unordered),
            b'{"evidence_ids":["ev_a","ev_b"]}',
        )
        self.assertNotEqual(
            canonical_json_bytes({"ranked": ["first", "second"]}),
            canonical_json_bytes({"ranked": ["second", "first"]}),
        )

    def test_timestamps_are_normalized_to_utc(self) -> None:
        from acmg_classifier.domain.canonical import canonical_json_bytes

        taipei = timezone(timedelta(hours=8))
        observed_at = datetime(2026, 7, 11, 12, 30, 45, 123456, tzinfo=taipei)

        self.assertEqual(
            canonical_json_bytes({"observed_at": observed_at}),
            b'{"observed_at":"2026-07-11T04:30:45.123456Z"}',
        )
        self.assertEqual(
            canonical_json_bytes({"observed_at": observed_at.astimezone(UTC)}),
            canonical_json_bytes({"observed_at": observed_at}),
        )

    def test_naive_timestamp_and_non_finite_number_are_rejected(self) -> None:
        from acmg_classifier.domain.canonical import (
            CanonicalizationError,
            canonical_json_bytes,
        )

        with self.assertRaisesRegex(CanonicalizationError, "timezone-aware"):
            canonical_json_bytes({"observed_at": datetime(2026, 7, 11)})

        for invalid_number in (math.nan, math.inf, -math.inf):
            with (
                self.subTest(invalid_number=invalid_number),
                self.assertRaisesRegex(CanonicalizationError, "finite"),
            ):
                canonical_json_bytes({"value": invalid_number})

    def test_explicit_display_fields_can_be_excluded_from_decision_hash(self) -> None:
        from acmg_classifier.domain.canonical import canonical_hash

        first = {"classification": "VUS", "explanation": "First wording"}
        second = {"classification": "VUS", "explanation": "Revised wording"}

        self.assertEqual(
            canonical_hash(first, excluded_keys={"explanation"}),
            canonical_hash(second, excluded_keys={"explanation"}),
        )

    def test_unsupported_values_are_rejected(self) -> None:
        from acmg_classifier.domain.canonical import (
            CanonicalizationError,
            canonical_json_bytes,
        )

        with self.assertRaisesRegex(CanonicalizationError, "Unsupported"):
            canonical_json_bytes({"value": object()})

    def test_finite_numbers_and_string_enums_are_supported(self) -> None:
        from acmg_classifier.domain.canonical import canonical_json_bytes
        from acmg_classifier.domain.enums import WorkflowStatus

        self.assertEqual(
            canonical_json_bytes(
                {"confidence": 0.5, "status": WorkflowStatus.NEEDS_CONTEXT}
            ),
            b'{"confidence":0.5,"status":"needs_context"}',
        )

    def test_non_string_mapping_keys_are_rejected(self) -> None:
        from acmg_classifier.domain.canonical import (
            CanonicalizationError,
            canonical_json_bytes,
        )

        with self.assertRaisesRegex(CanonicalizationError, "keys must be strings"):
            canonical_json_bytes({1: "not a JSON object key"})


if __name__ == "__main__":
    unittest.main()
