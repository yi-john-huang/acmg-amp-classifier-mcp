from __future__ import annotations

import subprocess
import sys
from pathlib import Path


def test_history_scan_regression_detects_an_added_then_deleted_credential() -> None:
    root = Path(__file__).resolve().parents[2]

    completed = subprocess.run(
        [sys.executable, root / "scripts" / "verify_history_scan_regression.py"],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
    )

    assert completed.returncode == 0, completed.stderr
    assert "detected a credential removed from the working tree" in completed.stdout
