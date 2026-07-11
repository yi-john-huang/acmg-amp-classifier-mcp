from __future__ import annotations

import math
import unittest
import warnings
from datetime import UTC, datetime, timedelta, timezone

from pydantic import TypeAdapter, ValidationError

OBSERVATION_PAYLOADS: dict[str, dict[str, object]] = {
    "population": {
        "kind": "population",
        "source_release": "gnomad-r4.1",
        "ancestry": "global",
        "allele_count": 1,
        "allele_number": 1000,
        "allele_frequency": 0.001,
        "homozygote_count": 0,
        "hemizygote_count": None,
        "coverage": None,
        "filter_status": "pass",
    },
    "clinical_assertion": {
        "kind": "clinical_assertion",
        "accession": "SCV000123456",
        "clinical_significance": "pathogenic",
        "review_status": "criteria_provided_single_submitter",
        "condition_id": "MONDO:0011450",
        "condition_label": "hereditary breast cancer",
        "submitter": "Example laboratory",
        "last_evaluated": datetime(2026, 7, 10, tzinfo=UTC),
        "citation_ids": ("PMID:12345678",),
    },
    "computational": {
        "kind": "computational",
        "predictor": "REVEL",
        "prediction": "pathogenic_supporting",
        "score": 0.91,
        "threshold": 0.75,
        "dataset_version": "1.3",
        "transcript": "NM_007294.4",
    },
    "functional": {
        "kind": "functional",
        "assay_id": "PMID:12345678:assay-1",
        "assay_type": "DNA repair",
        "result": "abnormal",
        "validation_status": "validated",
        "effect_size": None,
        "confidence_interval_lower": None,
        "confidence_interval_upper": None,
        "calibration_reference": "ClinGen-SVI-v1",
    },
    "segregation": {
        "kind": "segregation",
        "family_count": 2,
        "informative_meioses": 4,
        "lod_score": 1.2,
        "co_segregations": 4,
        "non_segregations": 0,
        "phenotype_defined": True,
    },
    "de_novo": {
        "kind": "de_novo",
        "confirmation": "confirmed",
        "occurrence_count": 1,
        "paternity_confirmed": True,
        "maternity_confirmed": True,
        "phenotype_consistent": True,
    },
    "allelic": {
        "kind": "allelic",
        "other_variant_key": "ga4gh:VA.other-allele",
        "phase": "trans",
        "occurrence_count": 2,
        "observed_in_affected": True,
    },
    "phenotype": {
        "kind": "phenotype",
        "term_id": "HP:0003002",
        "state": "present",
        "specificity": "highly_specific",
    },
    "gene_mechanism": {
        "kind": "gene_mechanism",
        "gene_id": "HGNC:1100",
        "disease_id": "MONDO:0011450",
        "mechanism": "loss_of_function",
        "validity": "definitive",
        "inheritance": "autosomal_dominant",
    },
    "consequence": {
        "kind": "consequence",
        "transcript": "NM_007294.4",
        "consequence": "missense",
        "protein_change": "p.Arg1Gly",
        "nmd_predicted": None,
        "same_amino_acid_change": False,
        "same_amino_acid_splice_difference": False,
        "same_residue_different_amino_acid": True,
        "inframe_length": None,
        "splice_impact": "none",
    },
    "variant_location": {
        "kind": "variant_location",
        "region_type": "critical_domain",
        "region_id": "BRCA1:RING",
        "is_critical": True,
        "distance_to_splice_site": None,
    },
    "case_control": {
        "kind": "case_control",
        "study_id": "PMID:23456789",
        "case_count": 100,
        "control_count": 1000,
        "case_allele_count": 6,
        "control_allele_count": 1,
        "odds_ratio": 4.2,
        "p_value": 0.01,
    },
}


def source_provenance() -> dict[str, object]:
    return {
        "kind": "source",
        "source_id": "gnomad",
        "source_record_id": "1-43071077-G-A",
        "source_version": "4.1",
        "retrieved_at": datetime(2026, 7, 11, 8, 0, tzinfo=UTC),
        "normalized_query_key": "ga4gh:VA.example",
    }


def evidence_payload(observation: dict[str, object]) -> dict[str, object]:
    return {
        "variant_key": "ga4gh:VA.example",
        "kind": observation["kind"],
        "observation": observation,
        "context_scope": {
            "genome_build": "GRCh38",
            "transcript": "NM_007294.4",
            "disease_id": "MONDO:0011450",
            "inheritance": "autosomal_dominant",
        },
        "provenance": source_provenance(),
        "raw_snapshot_ref": "raw_" + "a" * 64,
        "quality_flags": ("verified",),
        "derivation": "source",
    }


class EvidenceModelTests(unittest.TestCase):
    def test_all_observation_discriminators_are_typed_and_content_addressed(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import EvidenceItem, Observation

        adapter = TypeAdapter(Observation)
        for expected_kind, observation in OBSERVATION_PAYLOADS.items():
            with self.subTest(kind=expected_kind):
                parsed_observation = adapter.validate_python(observation)
                item = EvidenceItem.model_validate(evidence_payload(observation))

                self.assertEqual(parsed_observation.kind.value, expected_kind)
                self.assertEqual(item.kind.value, expected_kind)
                self.assertEqual(item.observation.kind.value, expected_kind)
                self.assertTrue(item.evidence_id.startswith("ev_"))

    def test_provenance_discriminators_preserve_origin_and_require_matching_derivation(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import EvidenceItem

        observation = OBSERVATION_PAYLOADS["population"]
        cases = (
            (
                "user",
                {
                    "kind": "user",
                    "actor_id": "usr_researcher_001",
                    "submitted_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                    "confirmation_method": "attested",
                    "citation_ids": ("PMID:12345678",),
                },
            ),
            (
                "derived",
                {
                    "kind": "derived",
                    "derivation_name": "normalized_population_frequency",
                    "component_version": "1.0.0",
                    "input_evidence_ids": ("ev_" + "b" * 64,),
                    "generated_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                },
            ),
            (
                "review",
                {
                    "kind": "review",
                    "review_id": "review_" + "c" * 32,
                    "reviewer_id": "agent_literature_reviewer",
                    "reviewed_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                    "input_evidence_ids": ("ev_" + "b" * 64,),
                },
            ),
        )
        for derivation, provenance in cases:
            with self.subTest(derivation=derivation):
                payload = evidence_payload(observation)
                payload["derivation"] = derivation
                payload["provenance"] = provenance
                payload["raw_snapshot_ref"] = None
                item = EvidenceItem.model_validate(payload)
                self.assertEqual(item.derivation.value, derivation)
                self.assertEqual(item.provenance.kind.value, derivation)

        mismatch = evidence_payload(observation)
        mismatch["derivation"] = "review"
        with self.assertRaises(ValidationError):
            EvidenceItem.model_validate(mismatch)

    def test_source_provenance_normalizes_aware_times_and_rejects_naive_times(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import SourceProvenance

        taipei = timezone(timedelta(hours=8))
        provenance = SourceProvenance.model_validate(
            {
                **source_provenance(),
                "retrieved_at": datetime(2026, 7, 11, 16, 0, tzinfo=taipei),
            }
        )

        self.assertEqual(provenance.retrieved_at.tzinfo, UTC)
        self.assertEqual(provenance.retrieved_at.hour, 8)
        with self.assertRaises(ValidationError):
            SourceProvenance.model_validate(
                {
                    **source_provenance(),
                    "retrieved_at": datetime(2026, 7, 11, 8, 0),
                }
            )

    def test_null_numeric_observations_remain_unknown_and_invalid_counts_fail(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import PopulationObservation

        unknown = PopulationObservation.model_validate(
            {
                "kind": "population",
                "source_release": "gnomad-r4.1",
                "allele_count": None,
                "allele_number": None,
                "allele_frequency": None,
                "homozygote_count": None,
                "hemizygote_count": None,
                "coverage": None,
                "filter_status": "unknown",
            }
        )
        self.assertIsNone(unknown.allele_count)
        self.assertIsNone(unknown.allele_frequency)
        rounded = PopulationObservation.model_validate(
            {
                "kind": "population",
                "source_release": "gnomad-r4.1",
                "allele_count": 1,
                "allele_number": 100,
                "allele_frequency": 0.0100005,
                "filter_status": "pass",
            }
        )
        zero_count = PopulationObservation.model_validate(
            {
                "kind": "population",
                "source_release": "gnomad-r4.1",
                "allele_count": 0,
                "allele_number": 100,
                "allele_frequency": 0.0,
                "filter_status": "pass",
            }
        )
        self.assertEqual(zero_count.allele_frequency, 0.0)

        self.assertEqual(rounded.allele_frequency, 0.0100005)

        for invalid in (
            {"allele_count": "1"},
            {"allele_count": 2, "allele_number": 1},
            {"allele_count": 1, "allele_number": 100, "allele_frequency": 0.2},
            {"allele_count": 1, "homozygote_count": 1},
            {"allele_count": 1, "hemizygote_count": 2},
            {"allele_frequency": math.nan},
            {"allele_frequency": 1.1},
            {"allele_count": 0, "allele_number": 100, "allele_frequency": 0.0000001},
        ):
            with self.subTest(invalid=invalid), self.assertRaises(ValidationError):
                PopulationObservation.model_validate(
                    {
                        "kind": "population",
                        "source_release": "gnomad-r4.1",
                        "filter_status": "pass",
                        **invalid,
                    }
                )

    def test_segregation_counts_require_positive_informative_meioses(self) -> None:
        from acmg_classifier.domain.evidence import SegregationObservation

        for counts in (
            {"informative_meioses": 2, "co_segregations": 2, "non_segregations": 1},
            {"informative_meioses": 0, "non_segregations": 1},
            {"informative_meioses": 0, "co_segregations": 0},
        ):
            with self.subTest(counts=counts), self.assertRaises(ValidationError):
                SegregationObservation(
                    kind="segregation",
                    family_count=1,
                    phenotype_defined=True,
                    **counts,
                )

    def test_items_are_strict_variant_and_context_scoped_and_verify_supplied_ids(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import EvidenceItem

        payload = evidence_payload(OBSERVATION_PAYLOADS["population"])
        first = EvidenceItem.model_validate(payload)
        second = EvidenceItem.model_validate(payload)
        self.assertEqual(first.evidence_id, second.evidence_id)

        invalid_payloads = (
            {**payload, "unknown_fact": "must fail"},
            {**payload, "variant_key": "   "},
            {
                **payload,
                "context_scope": {"genome_build": "hg19"},
            },
            {
                **payload,
                "kind": "functional",
            },
            {**payload, "evidence_id": "ev_" + "0" * 64},
        )
        for invalid_payload in invalid_payloads:
            with (
                self.subTest(invalid_payload=invalid_payload),
                self.assertRaises(ValidationError),
            ):
                EvidenceItem.model_validate(invalid_payload)

    def test_review_provenance_cannot_be_mislabeled_as_a_source_record(self) -> None:
        from acmg_classifier.domain.evidence import ReviewProvenance

        with self.assertRaises(ValidationError):
            ReviewProvenance.model_validate(
                {
                    "kind": "review",
                    "review_id": "review_" + "c" * 32,
                    "reviewer_id": "agent_literature_reviewer",
                    "reviewed_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                    "input_evidence_ids": ("ev_" + "b" * 64,),
                    "source_id": "clinvar",
                }
            )

    def test_evidence_snapshot_is_order_independent_and_fact_set_is_typed(self) -> None:
        from acmg_classifier.domain.evidence import (
            EvidenceItem,
            EvidenceSnapshot,
            FactSet,
        )

        population = EvidenceItem.model_validate(
            evidence_payload(OBSERVATION_PAYLOADS["population"])
        )
        clinical = EvidenceItem.model_validate(
            evidence_payload(OBSERVATION_PAYLOADS["clinical_assertion"])
        )
        created_at = datetime(2026, 7, 11, 10, 0, tzinfo=UTC)
        statuses = (
            {
                "source_id": "clinvar",
                "status": "fresh",
                "checked_at": created_at,
            },
            {
                "source_id": "gnomad",
                "status": "fresh",
                "checked_at": created_at,
            },
        )
        first = EvidenceSnapshot.model_validate(
            {
                "evidence_ids": (population.evidence_id, clinical.evidence_id),
                "source_statuses": statuses,
                "policy": {"mode": "live", "max_age_seconds": 3600},
                "created_at": created_at,
            }
        )
        second = EvidenceSnapshot.model_validate(
            {
                "evidence_ids": (clinical.evidence_id, population.evidence_id),
                "source_statuses": tuple(reversed(statuses)),
                "policy": {"mode": "live", "max_age_seconds": 3600},
                "created_at": created_at,
            }
        )

        self.assertEqual(first.snapshot_id, second.snapshot_id)
        self.assertEqual(first.evidence_ids, tuple(sorted(first.evidence_ids)))
        facts = FactSet.from_evidence((clinical, population))
        self.assertEqual(facts.population, (population.observation,))
        self.assertEqual(facts.clinical_assertions, (clinical.observation,))

    def test_canonical_ids_accept_only_matching_supplied_values(self) -> None:
        from acmg_classifier.domain.canonical import canonical_hash
        from acmg_classifier.domain.evidence import EvidenceItem, EvidenceSnapshot

        item = EvidenceItem.model_validate(
            evidence_payload(OBSERVATION_PAYLOADS["population"])
        )
        self.assertEqual(
            item.evidence_id, f"ev_{canonical_hash(item.canonical_content())}"
        )
        supplied_item = EvidenceItem.model_validate(
            {
                **evidence_payload(OBSERVATION_PAYLOADS["population"]),
                "evidence_id": item.evidence_id,
            }
        )
        self.assertEqual(supplied_item.evidence_id, item.evidence_id)

        snapshot = EvidenceSnapshot.model_validate(
            {
                "evidence_ids": (item.evidence_id,),
                "source_statuses": (
                    {
                        "source_id": "gnomad",
                        "status": "fresh",
                        "checked_at": datetime(2026, 7, 11, 10, 0, tzinfo=UTC),
                    },
                ),
                "policy": {"mode": "live"},
                "created_at": datetime(2026, 7, 11, 10, 0, tzinfo=UTC),
            }
        )
        supplied_snapshot = EvidenceSnapshot.model_validate(
            {
                "snapshot_id": snapshot.snapshot_id,
                "evidence_ids": (item.evidence_id,),
                "source_statuses": (
                    {
                        "source_id": "gnomad",
                        "status": "fresh",
                        "checked_at": datetime(2026, 7, 11, 10, 0, tzinfo=UTC),
                    },
                ),
                "policy": {"mode": "live"},
                "created_at": datetime(2026, 7, 12, 10, 0, tzinfo=UTC),
            }
        )
        self.assertEqual(supplied_snapshot.snapshot_id, snapshot.snapshot_id)
        with self.assertRaises(ValidationError):
            EvidenceSnapshot.model_validate(
                {
                    **supplied_snapshot.model_dump(mode="python"),
                    "snapshot_id": "es_" + "0" * 64,
                }
            )

    def test_direct_construction_assigns_content_ids_without_warnings(self) -> None:
        from acmg_classifier.domain.evidence import EvidenceItem, EvidenceSnapshot

        with warnings.catch_warnings():
            warnings.simplefilter("error")
            item = EvidenceItem(**evidence_payload(OBSERVATION_PAYLOADS["population"]))
            snapshot = EvidenceSnapshot(
                evidence_ids=(item.evidence_id,),
                source_statuses=(
                    {
                        "source_id": "gnomad",
                        "status": "fresh",
                        "checked_at": datetime(2026, 7, 11, 10, 0, tzinfo=UTC),
                    },
                ),
                policy={"mode": "live"},
                created_at=datetime(2026, 7, 11, 10, 0, tzinfo=UTC),
            )

        self.assertTrue(item.evidence_id.startswith("ev_"))
        self.assertTrue(snapshot.snapshot_id.startswith("es_"))

    def test_unknown_discriminators_and_criterion_fields_are_rejected(self) -> None:
        from acmg_classifier.domain.evidence import EvidenceItem, Observation

        adapter = TypeAdapter(Observation)
        with self.assertRaises(ValidationError):
            adapter.validate_python({"kind": "unrecognized"})

        for smuggled_field in (
            "criterion_code",
            "criterion_status",
            "criterion_score",
            "classification",
            "strength",
            "applied_strength",
        ):
            with (
                self.subTest(smuggled_field=smuggled_field),
                self.assertRaises(ValidationError),
            ):
                EvidenceItem.model_validate(
                    {
                        **evidence_payload(OBSERVATION_PAYLOADS["population"]),
                        smuggled_field: "PM2",
                    }
                )
            with (
                self.subTest(nested_smuggled_field=smuggled_field),
                self.assertRaises(ValidationError),
            ):
                EvidenceItem.model_validate(
                    evidence_payload(
                        {
                            **OBSERVATION_PAYLOADS["population"],
                            smuggled_field: "PM2",
                        }
                    )
                )

        malformed = evidence_payload(OBSERVATION_PAYLOADS["population"])
        malformed["observation"] = {
            **OBSERVATION_PAYLOADS["population"],
            "kind": "unrecognized",
        }
        with self.assertRaises(ValidationError):
            EvidenceItem.model_validate(malformed)

    def test_provenance_isolation_and_raw_snapshot_rules_fail_closed(self) -> None:
        from acmg_classifier.domain.evidence import (
            EvidenceItem,
            ReviewProvenance,
            SourceProvenance,
        )

        with self.assertRaises(ValidationError):
            SourceProvenance.model_validate(
                {**source_provenance(), "actor_id": "agent_not_a_source"}
            )
        with self.assertRaises(ValidationError):
            ReviewProvenance.model_validate(
                {
                    "kind": "review",
                    "review_id": "review_" + "c" * 32,
                    "reviewer_id": "agent_literature_reviewer",
                    "reviewed_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                    "input_evidence_ids": ("ev_" + "b" * 64,),
                    "source_record_id": "SCV000123456",
                    "retrieved_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
                }
            )

        source_without_raw = evidence_payload(OBSERVATION_PAYLOADS["population"])
        source_without_raw["raw_snapshot_ref"] = None
        with self.assertRaises(ValidationError):
            EvidenceItem.model_validate(source_without_raw)

        user_with_raw = evidence_payload(OBSERVATION_PAYLOADS["population"])
        user_with_raw["derivation"] = "user"
        user_with_raw["provenance"] = {
            "kind": "user",
            "actor_id": "usr_researcher_001",
            "submitted_at": datetime(2026, 7, 11, 9, 0, tzinfo=UTC),
            "confirmation_method": "attested",
        }
        with self.assertRaises(ValidationError):
            EvidenceItem.model_validate(user_with_raw)

    def test_non_population_numeric_unknowns_are_null_and_non_finite_values_fail(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import (
            CaseControlObservation,
            ComputationalObservation,
            FunctionalObservation,
            SegregationObservation,
        )

        computational = ComputationalObservation.model_validate(
            {
                **OBSERVATION_PAYLOADS["computational"],
                "score": None,
                "threshold": None,
            }
        )
        functional = FunctionalObservation.model_validate(
            {
                **OBSERVATION_PAYLOADS["functional"],
                "effect_size": None,
                "confidence_interval_lower": None,
                "confidence_interval_upper": None,
            }
        )
        segregation = SegregationObservation.model_validate(
            {**OBSERVATION_PAYLOADS["segregation"], "lod_score": None}
        )
        case_control = CaseControlObservation.model_validate(
            {
                **OBSERVATION_PAYLOADS["case_control"],
                "case_count": None,
                "control_count": None,
                "case_allele_count": None,
                "control_allele_count": None,
                "odds_ratio": None,
                "p_value": None,
            }
        )
        self.assertIsNone(computational.score)
        self.assertIsNone(functional.effect_size)
        self.assertIsNone(segregation.lod_score)
        self.assertIsNone(case_control.p_value)

        invalid_cases: tuple[tuple[type[object], dict[str, object]], ...] = (
            (
                ComputationalObservation,
                {**OBSERVATION_PAYLOADS["computational"], "score": math.nan},
            ),
            (
                FunctionalObservation,
                {**OBSERVATION_PAYLOADS["functional"], "effect_size": math.inf},
            ),
            (
                SegregationObservation,
                {**OBSERVATION_PAYLOADS["segregation"], "family_count": "2"},
            ),
            (
                CaseControlObservation,
                {**OBSERVATION_PAYLOADS["case_control"], "p_value": math.nan},
            ),
            (
                CaseControlObservation,
                {**OBSERVATION_PAYLOADS["case_control"], "p_value": 1.1},
            ),
        )
        for model, payload in invalid_cases:
            with (
                self.subTest(model=model.__name__, payload=payload),
                self.assertRaises(ValidationError),
            ):
                model.model_validate(payload)

    def test_snapshot_policy_status_empty_and_duplicate_id_invariants(self) -> None:
        from acmg_classifier.domain.evidence import EvidenceSnapshot, SourceStatus

        first_id = "ev_" + "a" * 64
        second_id = "ev_" + "b" * 64
        checked_at = datetime(2026, 7, 11, 10, 0, tzinfo=UTC)
        base = {
            "evidence_ids": (second_id, first_id, first_id),
            "source_statuses": (
                {
                    "source_id": "gnomad",
                    "status": "fresh",
                    "checked_at": checked_at,
                    "source_version": "4.1",
                },
            ),
            "policy": {"mode": "live", "max_age_seconds": 3600},
            "created_at": checked_at,
        }
        snapshot = EvidenceSnapshot.model_validate(base)
        self.assertEqual(snapshot.evidence_ids, (first_id, second_id))
        self.assertEqual(
            snapshot.snapshot_id,
            EvidenceSnapshot.model_validate(
                {
                    **base,
                    "evidence_ids": (first_id, second_id),
                    "created_at": checked_at + timedelta(days=1),
                }
            ).snapshot_id,
        )
        self.assertNotEqual(
            snapshot.snapshot_id,
            EvidenceSnapshot.model_validate(
                {**base, "policy": {"mode": "cache", "max_age_seconds": 3600}}
            ).snapshot_id,
        )
        self.assertNotEqual(
            snapshot.snapshot_id,
            EvidenceSnapshot.model_validate(
                {
                    **base,
                    "source_statuses": (
                        {
                            "source_id": "gnomad",
                            "status": "stale",
                            "checked_at": checked_at,
                            "source_version": "4.1",
                        },
                    ),
                }
            ).snapshot_id,
        )
        for status in (
            "fresh",
            "cached",
            "stale",
            "unavailable",
            "rate_limited",
            "timeout",
            "schema_changed",
            "ineligible_cache",
        ):
            with self.subTest(status=status):
                self.assertEqual(
                    SourceStatus.model_validate(
                        {
                            "source_id": "gnomad",
                            "status": status,
                            "checked_at": checked_at,
                        }
                    ).status.value,
                    status,
                )
        with self.assertRaises(ValidationError):
            EvidenceSnapshot.model_validate({**base, "evidence_ids": ()})
        unavailable = EvidenceSnapshot.model_validate(
            {
                **base,
                "evidence_ids": (),
                "source_statuses": (
                    {
                        "source_id": "gnomad",
                        "status": "unavailable",
                        "checked_at": checked_at,
                    },
                ),
            }
        )
        self.assertEqual(unavailable.evidence_ids, ())
        self.assertTrue(unavailable.snapshot_id.startswith("es_"))
        with self.assertRaises(ValidationError):
            EvidenceSnapshot.model_validate(
                {
                    **base,
                    "source_statuses": (
                        *base["source_statuses"],
                        {
                            "source_id": "gnomad",
                            "status": "fresh",
                            "checked_at": checked_at,
                        },
                    ),
                }
            )

    def test_evidence_application_requires_exact_variant_and_compatible_context_scope(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import EvidenceContextScope, EvidenceItem

        source_scoped_payload = evidence_payload(OBSERVATION_PAYLOADS["population"])
        source_scoped_payload["context_scope"] = {"genome_build": "GRCh38"}
        item = EvidenceItem.model_validate(source_scoped_payload)
        requested_scope = EvidenceContextScope.model_validate(
            {
                "genome_build": "GRCh38",
                "transcript": "NM_007294.4",
                "disease_id": "MONDO:0011450",
                "inheritance": "autosomal_dominant",
            }
        )
        item.assert_applies_to(
            variant_key="ga4gh:VA.example", context_scope=requested_scope
        )
        with self.assertRaises(ValueError):
            item.assert_applies_to(
                variant_key="ga4gh:VA.other-allele",
                context_scope=requested_scope,
            )
        with self.assertRaises(ValueError):
            item.assert_applies_to(
                variant_key="ga4gh:VA.example",
                context_scope=EvidenceContextScope.model_validate(
                    {
                        "genome_build": "GRCh37",
                        "transcript": "NM_007294.4",
                        "disease_id": "MONDO:0011450",
                        "inheritance": "autosomal_recessive",
                    }
                ),
            )

    def test_only_source_evidence_may_use_a_narrower_context_scope(self) -> None:
        from acmg_classifier.domain.evidence import EvidenceContextScope, EvidenceItem

        requested_scope = EvidenceContextScope.model_validate(
            {
                "genome_build": "GRCh38",
                "transcript": "NM_007294.4",
                "disease_id": "MONDO:0011450",
                "inheritance": "autosomal_dominant",
            }
        )
        for derivation, provenance in (
            (
                "user",
                {
                    "kind": "user",
                    "submitted_at": datetime(2026, 7, 11, tzinfo=UTC),
                    "confirmation_method": "laboratory report",
                    "actor_id": "usr_clinician-1",
                },
            ),
            (
                "derived",
                {
                    "kind": "derived",
                    "derivation_name": "evidence synthesis",
                    "component_version": "1.0",
                    "input_evidence_ids": ("ev_" + "a" * 64,),
                    "generated_at": datetime(2026, 7, 11, tzinfo=UTC),
                },
            ),
            (
                "review",
                {
                    "kind": "review",
                    "review_id": "review_" + "a" * 32,
                    "reviewer_id": "usr_reviewer-1",
                    "reviewed_at": datetime(2026, 7, 11, tzinfo=UTC),
                    "input_evidence_ids": ("ev_" + "a" * 64,),
                },
            ),
        ):
            with self.subTest(derivation=derivation):
                payload = evidence_payload(OBSERVATION_PAYLOADS["population"])
                payload["context_scope"] = {"genome_build": "GRCh38"}
                payload["derivation"] = derivation
                payload["provenance"] = provenance
                payload.pop("raw_snapshot_ref")
                item = EvidenceItem.model_validate(payload)
                with self.assertRaises(ValueError):
                    item.assert_applies_to(
                        variant_key="ga4gh:VA.example",
                        context_scope=requested_scope,
                    )


if __name__ == "__main__":
    unittest.main()
