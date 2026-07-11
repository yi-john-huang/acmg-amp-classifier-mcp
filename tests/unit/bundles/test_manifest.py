from __future__ import annotations

import unittest

from pydantic import ValidationError


def valid_manifest(version: str = "2026.7.1") -> dict[str, object]:
    return {
        "format_version": "1.0",
        "bundle_version": version,
        "application_version": {"minimum": "0.1.0", "maximum": "0.9.9"},
        "schema_version": {"minimum": "1.0", "maximum": "1.2"},
        "created_at": "2026-07-11T08:00:00Z",
        "channel": "stable",
        "artifacts": [
            {
                "path": "knowledge.sqlite3",
                "byte_size": 128,
                "sha256": "a" * 64,
                "media_type": "application/vnd.sqlite3",
                "role": "knowledge",
            }
        ],
        "sources": [
            {
                "name": "NCBI MANE",
                "release": "1.5",
                "url": "https://www.ncbi.nlm.nih.gov/refseq/MANE/",
                "retrieved_at": "2026-07-11T07:00:00Z",
                "license": "United States government work",
                "sha256": "b" * 64,
                "terms_url": "https://www.ncbi.nlm.nih.gov/home/about/policies/",
                "transformation_version": "1.0.0",
            }
        ],
        "genome_builds": ["GRCh38"],
        "transcript_release": "MANE Select v1.5",
        "rulesets": [{"identifier": "acmg-amp", "version": "2015.1"}],
        "signer_key_id": "release-2026",
        "signature_algorithm": "Ed25519",
    }


class BundleManifestTests(unittest.TestCase):
    def test_valid_manifest_is_strict_and_immutable(self) -> None:
        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest

        manifest = BundleManifest.model_validate(valid_manifest())

        self.assertEqual(manifest.bundle_version, "2026.7.1")
        self.assertEqual(manifest.artifacts[0].byte_size, 128)
        self.assertEqual(manifest.genome_builds[0].value, "GRCh38")
        with self.assertRaises(ValidationError):
            BundleManifest.model_validate({**valid_manifest(), "unknown": True})

    def test_missing_or_invalid_provenance_is_rejected(self) -> None:
        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest

        cases = []
        missing_license = valid_manifest()
        missing_license["sources"] = [{**missing_license["sources"][0], "license": ""}]
        cases.append(missing_license)
        invalid_hash = valid_manifest()
        invalid_hash["artifacts"] = [{**invalid_hash["artifacts"][0], "sha256": "abc"}]
        cases.append(invalid_hash)
        insecure_url = valid_manifest()
        insecure_url["sources"] = [
            {**insecure_url["sources"][0], "url": "http://example.test/mane"}
        ]
        cases.append(insecure_url)
        unsafe_path = valid_manifest()
        unsafe_path["artifacts"] = [
            {**unsafe_path["artifacts"][0], "path": "../escape.sqlite3"}
        ]
        cases.append(unsafe_path)

        for case in cases:
            with self.subTest(case=case), self.assertRaises(ValidationError):
                BundleManifest.model_validate(case)

    def test_resolver_selects_highest_compatible_version(self) -> None:
        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
        from acmg_classifier.infrastructure.bundles.resolver import (
            BundleRequirements,
            select_bundle,
        )

        requirements = BundleRequirements(
            application_version="0.1.0",
            schema_version="1.0",
            genome_build="GRCh38",
            ruleset_id="acmg-amp",
            ruleset_version="2015.1",
        )
        older = BundleManifest.model_validate(valid_manifest("2026.7.1"))
        newest = BundleManifest.model_validate(valid_manifest("2026.8.0"))

        selection = select_bundle((newest, older), requirements)

        self.assertEqual(selection.selected, newest)
        self.assertEqual(selection.rejected, ())

    def test_resolver_reports_every_incompatibility(self) -> None:
        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
        from acmg_classifier.infrastructure.bundles.resolver import (
            BundleRequirements,
            select_bundle,
        )

        requirements = BundleRequirements(
            application_version="1.0.0",
            schema_version="2.0",
            genome_build="GRCh37",
            ruleset_id="clingen-brca",
            ruleset_version="2.0",
        )
        manifest = BundleManifest.model_validate(valid_manifest())

        selection = select_bundle((manifest,), requirements)

        self.assertIsNone(selection.selected)
        reasons = selection.rejected[0].reasons
        self.assertIn("application version 1.0.0 is outside 0.1.0 to 0.9.9", reasons)
        self.assertIn("schema version 2.0 is outside 1.0 to 1.2", reasons)
        self.assertIn("genome build GRCh37 is not included", reasons)
        self.assertIn("ruleset clingen-brca 2.0 is not included", reasons)

    def test_pinned_version_is_exact_and_explained(self) -> None:
        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
        from acmg_classifier.infrastructure.bundles.resolver import (
            BundleRequirements,
            select_bundle,
        )

        requirements = BundleRequirements(
            application_version="0.1.0",
            schema_version="1.0",
            genome_build="GRCh38",
            ruleset_id="acmg-amp",
            ruleset_version="2015.1",
            pinned_bundle_version="2026.7.1",
        )
        pinned = BundleManifest.model_validate(valid_manifest("2026.7.1"))
        other = BundleManifest.model_validate(valid_manifest("2026.8.0"))

        selection = select_bundle((other, pinned), requirements)

        self.assertEqual(selection.selected, pinned)
        self.assertEqual(
            selection.rejected[0].reasons,
            ("bundle version 2026.8.0 does not match pin 2026.7.1",),
        )


if __name__ == "__main__":
    unittest.main()
