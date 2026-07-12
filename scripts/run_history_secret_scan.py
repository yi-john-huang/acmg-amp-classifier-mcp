#!/usr/bin/env python3
"""Run the repository's pinned-config Gitleaks history scan."""

from __future__ import annotations

import argparse
import os
import subprocess
from pathlib import Path

_REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
_CONFIG_PATH = _REPOSITORY_ROOT / ".gitleaks.toml"


def _history_range(
    repository: Path,
    *,
    before: str | None,
    head: str,
    base_ref: str | None,
) -> str:
    if base_ref:
        completed = subprocess.run(
            ["git", "merge-base", base_ref, head],
            cwd=repository,
            check=False,
            capture_output=True,
            text=True,
        )
        if completed.returncode != 0:
            raise RuntimeError(f"Unable to determine merge base for {base_ref}")
        return f"{completed.stdout.strip()}..{head}"
    if before and before != "0" * 40:
        return f"{before}..{head}"
    return "--all"


def run_history_scan(
    repository: Path,
    *,
    before: str | None = None,
    head: str = "HEAD",
    base_ref: str | None = None,
) -> subprocess.CompletedProcess[bytes]:
    """Scan the selected repository history with the shared Gitleaks policy."""
    command = os.environ.get("GITLEAKS_COMMAND", "gitleaks")
    log_options = _history_range(
        repository,
        before=before,
        head=head,
        base_ref=base_ref,
    )
    return subprocess.run(
        [
            command,
            "git",
            "--no-banner",
            "--config",
            str(_CONFIG_PATH),
            f"--log-opts={log_options}",
            str(repository),
        ],
        check=False,
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", type=Path, default=_REPOSITORY_ROOT)
    parser.add_argument("--before")
    parser.add_argument("--head", default="HEAD")
    parser.add_argument("--base-ref")
    arguments = parser.parse_args()
    return run_history_scan(
        arguments.repository,
        before=arguments.before,
        head=arguments.head,
        base_ref=arguments.base_ref,
    ).returncode


if __name__ == "__main__":
    raise SystemExit(main())
