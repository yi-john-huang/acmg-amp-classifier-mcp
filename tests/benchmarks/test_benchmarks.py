from __future__ import annotations

import pytest

from acmg_classifier.validation.benchmarks import (
    COMPACT_RESULT_MAX_BYTES,
    REVIEW_PACKET_TOKEN_CAP,
    BenchmarkFixture,
    BudgetStatus,
    TimingKind,
    ToolCall,
    evaluate_benchmark_fixture,
    evaluate_compact_result_budget,
    evaluate_latency_budget,
    evaluate_review_packet_token_budget,
    evaluate_routine_classify_tool_budget,
    render_benchmark_report,
)


def test_compact_result_budget_measures_utf8_bytes_at_the_16_kib_boundary() -> None:
    within_limit = evaluate_compact_result_budget("x" * COMPACT_RESULT_MAX_BYTES)
    over_limit = evaluate_compact_result_budget("€" * 5462)

    assert within_limit.observed == 16 * 1024
    assert within_limit.status is BudgetStatus.PASS
    assert over_limit.observed == 16_386
    assert over_limit.status is BudgetStatus.FAIL
    assert (
        over_limit.methodology
        == "UTF-8 encoded payload bytes without transport framing"
    )


def test_routine_classify_tool_budget_counts_only_classify_variant_calls() -> None:
    evaluation = evaluate_routine_classify_tool_budget(
        (
            ToolCall(name="resolve_context"),
            ToolCall(name="classify_variant"),
            ToolCall(name="explain_classification"),
        )
    )
    repeated = evaluate_routine_classify_tool_budget(
        (ToolCall(name="classify_variant"), ToolCall(name="classify_variant"))
    )
    missing = evaluate_routine_classify_tool_budget(())

    assert evaluation.observed == 1
    assert evaluation.status is BudgetStatus.PASS
    assert repeated.observed == 2
    assert repeated.status is BudgetStatus.FAIL
    assert missing.observed == 0
    assert missing.status is BudgetStatus.FAIL


def test_review_packet_budget_enforces_declared_1200_token_cap() -> None:
    within_limit = evaluate_review_packet_token_budget(REVIEW_PACKET_TOKEN_CAP)
    over_limit = evaluate_review_packet_token_budget(REVIEW_PACKET_TOKEN_CAP + 1)

    assert within_limit.status is BudgetStatus.PASS
    assert over_limit.status is BudgetStatus.FAIL
    assert (
        within_limit.methodology
        == "Declared review packet token_cap policy value; not model token usage"
    )


def test_latency_budget_uses_deterministic_nearest_rank_percentiles() -> None:
    evaluation = evaluate_latency_budget(
        fixture_type="cached-classification",
        samples_ms=tuple(range(20, 0, -1)),
        p95_budget_ms=19,
        timing_kind=TimingKind.SYNTHETIC,
    )
    exceeded = evaluate_latency_budget(
        fixture_type="cached-classification",
        samples_ms=tuple(range(1, 21)),
        p95_budget_ms=18,
        timing_kind=TimingKind.SYNTHETIC,
    )

    assert evaluation.sample_count == 20
    assert evaluation.p50_ms == 10
    assert evaluation.p95_ms == 19
    assert evaluation.status is BudgetStatus.PASS
    assert exceeded.status is BudgetStatus.FAIL
    assert "synthetic" in evaluation.exclusions[0].lower()


def test_fixture_report_is_machine_readable_and_marks_synthetic_timings() -> None:
    report = evaluate_benchmark_fixture(
        BenchmarkFixture(
            fixture_type="cached-classification",
            compact_result="{}",
            routine_tool_calls=(ToolCall(name="classify_variant"),),
            review_packet_token_cap=REVIEW_PACKET_TOKEN_CAP,
            latency_samples_ms=tuple(range(1, 21)),
            latency_p95_budget_ms=19,
            timing_kind=TimingKind.SYNTHETIC,
        )
    )

    rendered = render_benchmark_report(report)

    assert rendered["fixture_type"] == "cached-classification"
    assert rendered["status"] == "pass"
    assert rendered["latency"]["sample_count"] == 20
    assert rendered["latency"]["p50_ms"] == 10
    assert rendered["latency"]["p95_ms"] == 19
    assert rendered["latency"]["budget_ms"] == 19
    assert rendered["latency"]["timing_kind"] == "synthetic"
    assert "synthetic" in rendered["exclusions"][0].lower()
    assert rendered["compact_result"]["methodology"] == (
        "UTF-8 encoded payload bytes without transport framing"
    )


def test_fixture_report_fails_closed_when_required_measurements_are_missing() -> None:
    report = evaluate_benchmark_fixture(
        BenchmarkFixture(
            fixture_type="cached-classification",
            latency_p95_budget_ms=5_000,
        )
    )

    rendered = render_benchmark_report(report)

    assert report.status is BudgetStatus.FAIL
    assert rendered["compact_result"]["status"] == "unavailable"
    assert rendered["routine_tool_calls"]["status"] == "unavailable"
    assert rendered["review_packet"]["status"] == "unavailable"
    assert rendered["latency"]["status"] == "unavailable"
    assert "missing required measurement" in rendered["exclusions"][0].lower()


def test_latency_report_labels_external_samples_as_needing_runtime_evidence() -> None:
    evaluation = evaluate_latency_budget(
        fixture_type="cached-classification",
        samples_ms=(1, 2, 3, 4),
        p95_budget_ms=4,
        timing_kind=TimingKind.EXTERNAL,
    )

    assert evaluation.status is BudgetStatus.PASS
    assert evaluation.timing_kind is TimingKind.EXTERNAL
    assert "runtime and platform evidence" in evaluation.exclusions[0]


def test_fixture_report_keeps_an_unprovided_latency_budget_unavailable() -> None:
    report = evaluate_benchmark_fixture(
        BenchmarkFixture(
            fixture_type="cached-classification",
            compact_result="{}",
            routine_tool_calls=(ToolCall(name="classify_variant"),),
            review_packet_token_cap=REVIEW_PACKET_TOKEN_CAP,
            latency_samples_ms=(1, 2, 3),
        )
    )

    rendered = render_benchmark_report(report)

    assert report.status is BudgetStatus.FAIL
    assert rendered["latency"]["status"] == "unavailable"
    assert rendered["latency"]["budget_ms"] is None
    assert rendered["latency"]["exclusions"] == [
        "Missing required measurement: latency p95 budget"
    ]


def test_metric_utilities_reject_invalid_measurements() -> None:
    with pytest.raises(ValueError, match="tool call name"):
        ToolCall(name="")
    with pytest.raises(ValueError, match="byte budget"):
        evaluate_compact_result_budget(b"{}", maximum_bytes=-1)
    with pytest.raises(ValueError, match="token cap"):
        evaluate_review_packet_token_budget(-1)
    with pytest.raises(ValueError, match="finite"):
        evaluate_latency_budget(
            fixture_type="cached-classification",
            samples_ms=(float("nan"),),
            p95_budget_ms=5_000,
            timing_kind=TimingKind.SYNTHETIC,
        )
