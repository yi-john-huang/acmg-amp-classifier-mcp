#!/usr/bin/env python3
"""Render deterministic benchmark evidence without making live measurements."""

from __future__ import annotations

import argparse
import json
import platform
from collections.abc import Sequence
from pathlib import Path

from acmg_classifier.application.review import DEFAULT_REVIEW_TOKEN_CAP
from acmg_classifier.validation.benchmarks import (
    BenchmarkFixture,
    TimingKind,
    ToolCall,
    evaluate_benchmark_fixture,
    render_benchmark_report,
)


def build_synthetic_report() -> dict[str, object]:
    """Build the documented synthetic fixture and label its limitations."""
    report = render_benchmark_report(
        evaluate_benchmark_fixture(
            BenchmarkFixture(
                fixture_type="cached-classification",
                compact_result="{}",
                routine_tool_calls=(ToolCall(name="classify_variant"),),
                review_packet_token_cap=DEFAULT_REVIEW_TOKEN_CAP,
                latency_samples_ms=tuple(range(1, 21)),
                latency_p95_budget_ms=19,
                timing_kind=TimingKind.SYNTHETIC,
            )
        )
    )
    latency = report["latency"]
    if not isinstance(latency, dict):
        raise RuntimeError("benchmark renderer returned invalid latency metadata")
    exclusions = report["exclusions"]
    if not isinstance(exclusions, list):
        raise RuntimeError("benchmark renderer returned invalid exclusions")
    return {
        **report,
        "timing_kind": TimingKind.SYNTHETIC.value,
        "platform": platform.platform(aliased=True),
        "python_version": platform.python_version(),
        "sample_count": latency["sample_count"],
        "method": "deterministic synthetic fixture; no live workload execution",
        "exclusions": [
            *exclusions,
            "Synthetic evidence does not satisfy the live-performance release gate.",
        ],
    }


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(build_synthetic_report(), indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
