from __future__ import annotations

import pytest


def test_service_exposes_parser_without_network_or_provider_dependencies() -> None:
    from acmg_classifier.application.normalization import VariantNormalizationService

    service = VariantNormalizationService()

    result = service.parse("NM_000492.4:c.1521_1523delCTT")

    assert result.scope == "supported"
    assert result.normalized_input == "NM_000492.4:c.1521_1523delCTT"


@pytest.mark.parametrize(
    ("value", "scope", "reason"),
    [
        ("NM_000492.3:c.1A>GG", "invalid", "malformed_syntax"),
        ("NC_000007.14:g.117199644delinsGG", "unsupported", "delins_not_supported"),
    ],
)
def test_service_returns_rejected_parse_outcomes_without_provider_work(
    value: str, scope: str, reason: str
) -> None:
    from acmg_classifier.application.normalization import VariantNormalizationService

    result = VariantNormalizationService().parse(value)

    assert result.scope == scope
    assert result.reason == reason
