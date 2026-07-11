#!/usr/bin/env python3
"""Prove the shared Gitleaks scan detects a deleted history-only credential."""

from __future__ import annotations

import subprocess
import sys
import tempfile
from pathlib import Path

from run_history_secret_scan import run_history_scan


def _run(repository: Path, *args: str) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(
        ["git", *args],
        cwd=repository,
        check=False,
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        raise RuntimeError("Git setup for the history-scan regression failed")
    return completed




def main() -> int:
    try:
        with tempfile.TemporaryDirectory(
            prefix="history-scan-regression-"
        ) as directory:
            repository = Path(directory)
            _run(repository, "init", "--quiet")
            _run(repository, "config", "user.email", "security-test@example.invalid")
            _run(repository, "config", "user.name", "Security Regression")

            credential_file = repository / "removed-history-fixture.env"
            credential = "A" + "KIA" + "".join(("QWER", "TYUI", "OPAS", "DFGH"))
            credential_file.write_text(
                "credential=" + credential + "\n", encoding="utf-8"
            )
            _run(repository, "add", credential_file.name)
            _run(repository, "commit", "--quiet", "-m", "add test credential")
            _run(repository, "rm", "--quiet", credential_file.name)
            _run(repository, "commit", "--quiet", "-m", "remove test credential")

            current_tree = subprocess.run(
                ["git", "ls-files", "--error-unmatch", credential_file.name],
                cwd=repository,
                check=False,
                capture_output=True,
                text=True,
            )
            if current_tree.returncode == 0:
                raise RuntimeError("The test credential unexpectedly remains tracked")
            if run_history_scan(repository).returncode == 0:
                raise RuntimeError(
                    "Gitleaks did not detect the deleted test credential"
                )
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"history-scan regression failed: {error}", file=sys.stderr)
        return 1

    print("gitleaks detected a credential removed from the working tree")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
