#!/usr/bin/env python3
"""Exercise a history-only API-key assignment that a full-history scanner sees.

The production gate uses Gitleaks. This regression creates a disposable Git
repository in which an API-key assignment is committed and then deleted. It
fails unless the value remains discoverable through complete Git history,
which protects the workflow's full-checkout requirement from a current-tree
only regression.
"""

from __future__ import annotations

import re
import subprocess
import sys
import tempfile
from pathlib import Path

_SECRET_ASSIGNMENT = re.compile(r"(?m)^\+?API_KEY\s*=\s*[^\s]+$")


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


def _history_detects_deleted_api_key(repository: Path) -> bool:
    history = _run(repository, "log", "--all", "--format=", "-p").stdout
    return _SECRET_ASSIGNMENT.search(history) is not None


def main() -> int:
    try:
        with tempfile.TemporaryDirectory(
            prefix="history-scan-regression-"
        ) as directory:
            repository = Path(directory)
            _run(repository, "init", "--quiet")
            _run(repository, "config", "user.email", "security-test@example.invalid")
            _run(repository, "config", "user.name", "Security Regression")

            credential_file = repository / "settings.env"
            credential_file.write_text(
                "API_KEY=history-regression-secret-value\n", encoding="utf-8"
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
            if not _history_detects_deleted_api_key(repository):
                raise RuntimeError(
                    "The deleted test credential was absent from Git history"
                )
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"history-scan regression failed: {error}", file=sys.stderr)
        return 1

    print("history-scan regression detected a credential removed from the working tree")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
