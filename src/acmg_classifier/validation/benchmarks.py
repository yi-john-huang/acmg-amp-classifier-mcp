"""Deterministic, fixture-based release budget evaluation.

This module evaluates supplied measurements. It deliberately does not collect timings,
make network calls, or infer production performance from synthetic fixtures.
"""

from __future__ import annotations

import math
from collections.abc import Sequence
from dataclasses import dataclass
from enum import StrEnum

from acmg_classifier.application.review import (
    DEFAULT_REVIEW_TOKEN_CAP as REVIEW_PACKET_TOKEN_CAP,
)

COMPACT_RESULT_MAX_BYTES = 16 * 1024
ROUTINE_CLASSIFY_TOOL_CALL_BUDGET = 1


class BudgetStatus(StrEnum):
    """The outcome of one measured release budget."""

    PASS = "pass"
    FAIL = "fail"
    UNAVAILABLE = "unavailable"


class TimingKind(StrEnum):
    """Provenance label for latency samples supplied to a benchmark fixture."""

    SYNTHETIC = "synthetic"
    EXTERNAL = "external"


type JsonScalar = str | int | float | bool | None
type JsonContent = JsonScalar | list[JsonContent] | dict[str, JsonContent]


@dataclass(frozen=True, slots=True)
class ToolCall:
    """One recorded tool invocation in the context-complete routine workflow."""

    name: str

    def __post_init__(self) -> None:
        if not self.name:
            raise ValueError("tool call name must not be empty")


@dataclass(frozen=True, slots=True)
class BudgetEvaluation:
    """A scalar budget outcome with its explicit measurement methodology."""

    metric: str
    observed: int | float | None
    budget: int | float
    unit: str
    status: BudgetStatus
    methodology: str
    exclusion: str | None = None

    def __post_init__(self) -> None:
        if not self.metric or not self.unit or not self.methodology:
            raise ValueError("budget evaluation requires metric, unit, and methodology")
        if self.status is BudgetStatus.UNAVAILABLE:
            if self.observed is not None or not self.exclusion:
                raise ValueError(
                    "unavailable budget evaluation requires no observation and "
                    "an exclusion"
                )
        elif self.observed is None:
            raise ValueError("available budget evaluation requires an observation")

    def to_content(self) -> dict[str, JsonContent]:
        """Return stable JSON-safe metric content without the measured payload."""
        return {
            "metric": self.metric,
            "observed": self.observed,
            "budget": self.budget,
            "unit": self.unit,
            "status": self.status.value,
            "methodology": self.methodology,
            "exclusion": self.exclusion,
        }


@dataclass(frozen=True, slots=True)
class LatencyBudgetEvaluation:
    """A supplied latency sample evaluated without timing runtime execution."""

    fixture_type: str
    sample_count: int
    p50_ms: float | None
    p95_ms: float | None
    budget_ms: float | None
    status: BudgetStatus
    timing_kind: TimingKind
    methodology: str
    exclusions: tuple[str, ...]

    def __post_init__(self) -> None:
        if not self.fixture_type:
            raise ValueError("latency evaluation requires a fixture type")
        if self.sample_count < 0 or (self.budget_ms is not None and self.budget_ms < 0):
            raise ValueError("latency sample count and budget must be non-negative")
        if not self.methodology:
            raise ValueError("latency evaluation requires a methodology")
        if self.status is BudgetStatus.UNAVAILABLE:
            if (
                self.sample_count != 0
                or self.p50_ms is not None
                or self.p95_ms is not None
            ):
                raise ValueError(
                    "unavailable latency evaluation must not contain samples"
                )
            if not self.exclusions:
                raise ValueError("unavailable latency evaluation requires an exclusion")
        elif (
            self.sample_count == 0
            or self.p50_ms is None
            or self.p95_ms is None
            or self.budget_ms is None
        ):
            raise ValueError(
                "available latency evaluation requires samples and a budget"
            )

    def to_content(self) -> dict[str, JsonContent]:
        """Return stable JSON-safe latency metric content."""
        return {
            "fixture_type": self.fixture_type,
            "sample_count": self.sample_count,
            "p50_ms": self.p50_ms,
            "p95_ms": self.p95_ms,
            "budget_ms": self.budget_ms,
            "status": self.status.value,
            "timing_kind": self.timing_kind.value,
            "methodology": self.methodology,
            "exclusions": list(self.exclusions),
        }


@dataclass(frozen=True, slots=True)
class BenchmarkFixture:
    """A complete, reproducible fixture input for the release budget gates.

    Each field is required for a passing report. Missing values are represented as
    unavailable measurements rather than substituted defaults.
    """

    fixture_type: str
    compact_result: str | bytes | None = None
    routine_tool_calls: tuple[ToolCall, ...] | None = None
    review_packet_token_cap: int | None = None
    latency_samples_ms: tuple[int | float, ...] | None = None
    latency_p95_budget_ms: int | float | None = None
    timing_kind: TimingKind = TimingKind.SYNTHETIC

    def __post_init__(self) -> None:
        if not self.fixture_type:
            raise ValueError("benchmark fixture type must not be empty")
        if not isinstance(self.timing_kind, TimingKind):
            raise TypeError("benchmark fixture timing kind must be a TimingKind")


@dataclass(frozen=True, slots=True)
class BenchmarkReport:
    """Machine-readable results for all required release budget measurements."""

    fixture_type: str
    compact_result: BudgetEvaluation
    routine_tool_calls: BudgetEvaluation
    review_packet: BudgetEvaluation
    latency: LatencyBudgetEvaluation
    status: BudgetStatus
    exclusions: tuple[str, ...]

    def __post_init__(self) -> None:
        if not self.fixture_type:
            raise ValueError("benchmark report requires a fixture type")
        expected_status = (
            BudgetStatus.PASS
            if all(
                evaluation.status is BudgetStatus.PASS
                for evaluation in (
                    self.compact_result,
                    self.routine_tool_calls,
                    self.review_packet,
                    self.latency,
                )
            )
            else BudgetStatus.FAIL
        )
        if self.status is not expected_status:
            raise ValueError("benchmark report status must fail closed")


def evaluate_compact_result_budget(
    serialized_output: str | bytes,
    *,
    maximum_bytes: int = COMPACT_RESULT_MAX_BYTES,
) -> BudgetEvaluation:
    """Evaluate encoded payload size against the compact-result byte budget."""
    if maximum_bytes < 0:
        raise ValueError("compact result byte budget must be non-negative")
    observed = _utf8_byte_size(serialized_output)
    return _evaluate_scalar_budget(
        metric="compact_result_utf8_bytes",
        observed=observed,
        budget=maximum_bytes,
        unit="bytes",
        methodology="UTF-8 encoded payload bytes without transport framing",
    )


def evaluate_routine_classify_tool_budget(
    tool_calls: Sequence[ToolCall],
    *,
    classify_tool_name: str = "classify_variant",
    maximum_calls: int = ROUTINE_CLASSIFY_TOOL_CALL_BUDGET,
) -> BudgetEvaluation:
    """Count routine classify-tool calls after context completion."""
    if not classify_tool_name:
        raise ValueError("classify tool name must not be empty")
    if maximum_calls < 0:
        raise ValueError("classify tool call budget must be non-negative")
    observed = sum(call.name == classify_tool_name for call in tool_calls)
    return BudgetEvaluation(
        metric="routine_classify_tool_calls",
        observed=observed,
        budget=maximum_calls,
        unit="calls",
        status=BudgetStatus.PASS if observed == maximum_calls else BudgetStatus.FAIL,
        methodology=(
            "Count ToolCall entries named 'classify_variant' after context completion; "
            "the routine contract requires exactly one call"
        ),
    )


def evaluate_review_packet_token_budget(
    declared_token_cap: int,
    *,
    maximum_tokens: int = REVIEW_PACKET_TOKEN_CAP,
) -> BudgetEvaluation:
    """Evaluate the declared specialist-review output cap, not model usage."""
    if isinstance(declared_token_cap, bool) or declared_token_cap < 0:
        raise ValueError(
            "declared review packet token cap must be a non-negative integer"
        )
    if isinstance(maximum_tokens, bool) or maximum_tokens < 0:
        raise ValueError("review packet token budget must be a non-negative integer")
    return _evaluate_scalar_budget(
        metric="review_packet_declared_token_cap",
        observed=declared_token_cap,
        budget=maximum_tokens,
        unit="tokens",
        methodology=(
            "Declared review packet token_cap policy value; not model token usage"
        ),
    )


def evaluate_latency_budget(
    *,
    fixture_type: str,
    samples_ms: Sequence[int | float] | None,
    p95_budget_ms: int | float,
    timing_kind: TimingKind,
) -> LatencyBudgetEvaluation:
    """Evaluate supplied samples with deterministic nearest-rank percentiles.

    This function performs no timing. Synthetic fixtures remain explicitly excluded
    from production-latency claims.
    """
    if not fixture_type:
        raise ValueError("latency fixture type must not be empty")
    _validate_non_negative_finite(p95_budget_ms, name="latency p95 budget")
    if not isinstance(timing_kind, TimingKind):
        raise TypeError("timing kind must be a TimingKind")
    methodology = "Nearest-rank percentiles over supplied millisecond samples"
    provenance_exclusion = _timing_exclusion(timing_kind)
    if samples_ms is None or not samples_ms:
        return LatencyBudgetEvaluation(
            fixture_type=fixture_type,
            sample_count=0,
            p50_ms=None,
            p95_ms=None,
            budget_ms=float(p95_budget_ms),
            status=BudgetStatus.UNAVAILABLE,
            timing_kind=timing_kind,
            methodology=methodology,
            exclusions=(
                "Missing required measurement: latency samples",
                provenance_exclusion,
            ),
        )

    samples = tuple(_validated_milliseconds(value) for value in samples_ms)
    p50 = _nearest_rank(samples, percentile=0.50)
    p95 = _nearest_rank(samples, percentile=0.95)
    return LatencyBudgetEvaluation(
        fixture_type=fixture_type,
        sample_count=len(samples),
        p50_ms=p50,
        p95_ms=p95,
        budget_ms=float(p95_budget_ms),
        status=BudgetStatus.PASS if p95 <= p95_budget_ms else BudgetStatus.FAIL,
        timing_kind=timing_kind,
        methodology=methodology,
        exclusions=(provenance_exclusion,),
    )


def evaluate_benchmark_fixture(fixture: BenchmarkFixture) -> BenchmarkReport:
    """Evaluate every required fixture measurement and fail closed if any is absent."""
    compact_result = (
        evaluate_compact_result_budget(fixture.compact_result)
        if fixture.compact_result is not None
        else _unavailable_scalar(
            metric="compact_result_utf8_bytes",
            budget=COMPACT_RESULT_MAX_BYTES,
            unit="bytes",
            methodology="UTF-8 encoded payload bytes without transport framing",
            exclusion="Missing required measurement: compact result payload",
        )
    )
    routine_tool_calls = (
        evaluate_routine_classify_tool_budget(fixture.routine_tool_calls)
        if fixture.routine_tool_calls is not None
        else _unavailable_scalar(
            metric="routine_classify_tool_calls",
            budget=ROUTINE_CLASSIFY_TOOL_CALL_BUDGET,
            unit="calls",
            methodology=(
                "Count ToolCall entries named 'classify_variant' after context "
                "completion; the routine contract requires exactly one call"
            ),
            exclusion="Missing required measurement: routine tool calls",
        )
    )
    review_packet = (
        evaluate_review_packet_token_budget(fixture.review_packet_token_cap)
        if fixture.review_packet_token_cap is not None
        else _unavailable_scalar(
            metric="review_packet_declared_token_cap",
            budget=REVIEW_PACKET_TOKEN_CAP,
            unit="tokens",
            methodology=(
                "Declared review packet token_cap policy value; not model token usage"
            ),
            exclusion="Missing required measurement: review packet token cap",
        )
    )
    latency = _evaluate_fixture_latency(fixture)
    status = (
        BudgetStatus.PASS
        if all(
            evaluation.status is BudgetStatus.PASS
            for evaluation in (
                compact_result,
                routine_tool_calls,
                review_packet,
                latency,
            )
        )
        else BudgetStatus.FAIL
    )
    exclusions = _unique_exclusions(
        (
            *_scalar_exclusions(compact_result),
            *_scalar_exclusions(routine_tool_calls),
            *_scalar_exclusions(review_packet),
            *latency.exclusions,
        )
    )
    return BenchmarkReport(
        fixture_type=fixture.fixture_type,
        compact_result=compact_result,
        routine_tool_calls=routine_tool_calls,
        review_packet=review_packet,
        latency=latency,
        status=status,
        exclusions=exclusions,
    )


def render_benchmark_report(report: BenchmarkReport) -> dict[str, JsonContent]:
    """Render a stable JSON-safe report suitable for later release documentation."""
    return {
        "schema_version": "1.0",
        "fixture_type": report.fixture_type,
        "status": report.status.value,
        "compact_result": report.compact_result.to_content(),
        "routine_tool_calls": report.routine_tool_calls.to_content(),
        "review_packet": report.review_packet.to_content(),
        "latency": report.latency.to_content(),
        "exclusions": list(report.exclusions),
    }


def _evaluate_fixture_latency(fixture: BenchmarkFixture) -> LatencyBudgetEvaluation:
    if fixture.latency_p95_budget_ms is None:
        return LatencyBudgetEvaluation(
            fixture_type=fixture.fixture_type,
            sample_count=0,
            p50_ms=None,
            p95_ms=None,
            budget_ms=None,
            status=BudgetStatus.UNAVAILABLE,
            timing_kind=fixture.timing_kind,
            methodology="Nearest-rank percentiles over supplied millisecond samples",
            exclusions=("Missing required measurement: latency p95 budget",),
        )
    return evaluate_latency_budget(
        fixture_type=fixture.fixture_type,
        samples_ms=fixture.latency_samples_ms,
        p95_budget_ms=fixture.latency_p95_budget_ms,
        timing_kind=fixture.timing_kind,
    )


def _evaluate_scalar_budget(
    *,
    metric: str,
    observed: int | float,
    budget: int | float,
    unit: str,
    methodology: str,
) -> BudgetEvaluation:
    return BudgetEvaluation(
        metric=metric,
        observed=observed,
        budget=budget,
        unit=unit,
        status=BudgetStatus.PASS if observed <= budget else BudgetStatus.FAIL,
        methodology=methodology,
    )


def _unavailable_scalar(
    *,
    metric: str,
    budget: int | float,
    unit: str,
    methodology: str,
    exclusion: str,
) -> BudgetEvaluation:
    return BudgetEvaluation(
        metric=metric,
        observed=None,
        budget=budget,
        unit=unit,
        status=BudgetStatus.UNAVAILABLE,
        methodology=methodology,
        exclusion=exclusion,
    )


def _utf8_byte_size(serialized_output: str | bytes) -> int:
    if isinstance(serialized_output, bytes):
        return len(serialized_output)
    if isinstance(serialized_output, str):
        return len(serialized_output.encode("utf-8"))
    raise TypeError("compact result must be str or bytes")


def _validate_non_negative_finite(value: int | float, *, name: str) -> None:
    if isinstance(value, bool) or not math.isfinite(value) or value < 0:
        raise ValueError(f"{name} must be a non-negative finite number")


def _validated_milliseconds(value: int | float) -> float:
    _validate_non_negative_finite(value, name="latency sample")
    return float(value)


def _nearest_rank(samples: Sequence[float], *, percentile: float) -> float:
    sorted_samples = sorted(samples)
    rank = max(1, math.ceil(percentile * len(sorted_samples)))
    return sorted_samples[rank - 1]


def _timing_exclusion(timing_kind: TimingKind) -> str:
    if timing_kind is TimingKind.SYNTHETIC:
        return (
            "Synthetic fixture timings are deterministic and do not establish "
            "production latency."
        )
    return "External timing samples require independent runtime and platform evidence."


def _scalar_exclusions(evaluation: BudgetEvaluation) -> tuple[str, ...]:
    return (evaluation.exclusion,) if evaluation.exclusion is not None else ()


def _unique_exclusions(exclusions: Sequence[str]) -> tuple[str, ...]:
    return tuple(dict.fromkeys(exclusions))
