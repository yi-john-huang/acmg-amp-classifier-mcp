from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path


def _gitleaks_stub(tmp_path: Path, exit_code: int) -> Path:
    command = tmp_path / "gitleaks"
    command.write_text(f"#!/bin/sh\nexit {exit_code}\n", encoding="utf-8")
    command.chmod(0o755)
    return command

def test_history_scan_regression_accepts_gitleaks_known_pattern_detection(
    tmp_path: Path,
) -> None:
    root = Path(__file__).resolve().parents[2]
    environment = os.environ | {
        "GITLEAKS_COMMAND": str(_gitleaks_stub(tmp_path, exit_code=1))
    }

    completed = subprocess.run(
        [sys.executable, root / "scripts" / "verify_history_scan_regression.py"],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
        env=environment,
    )

    assert completed.returncode == 0, completed.stderr
    assert (
        "gitleaks detected a credential removed from the working tree"
        in completed.stdout
    )


def test_history_scan_regression_rejects_a_clean_gitleaks_result(
    tmp_path: Path,
) -> None:
    root = Path(__file__).resolve().parents[2]
    environment = os.environ | {
        "GITLEAKS_COMMAND": str(_gitleaks_stub(tmp_path, exit_code=0))
    }

    completed = subprocess.run(
        [sys.executable, root / "scripts" / "verify_history_scan_regression.py"],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
        env=environment,
    )

    assert completed.returncode == 1
    assert "Gitleaks did not detect the deleted test credential" in completed.stderr


def _run_git(repository: Path, *args: str) -> str:
    completed = subprocess.run(
        ["git", *args],
        cwd=repository,
        check=True,
        capture_output=True,
        text=True,
    )
    return completed.stdout.strip()


def test_shared_history_scanner_limits_pushes_to_the_event_range(
    tmp_path: Path,
) -> None:
    root = Path(__file__).resolve().parents[2]
    repository = tmp_path / "repository"
    repository.mkdir()
    _run_git(repository, "init", "--quiet")
    _run_git(repository, "config", "user.email", "security-test@example.invalid")
    _run_git(repository, "config", "user.name", "Security Regression")
    tracked_file = repository / "tracked.txt"
    tracked_file.write_text("before\n", encoding="utf-8")
    _run_git(repository, "add", tracked_file.name)
    _run_git(repository, "commit", "--quiet", "-m", "before")
    before = _run_git(repository, "rev-parse", "HEAD")
    tracked_file.write_text("after\n", encoding="utf-8")
    _run_git(repository, "commit", "--quiet", "-am", "after")
    head = _run_git(repository, "rev-parse", "HEAD")

    arguments_path = tmp_path / "gitleaks-arguments.txt"
    command = tmp_path / "gitleaks"
    command.write_text(
        "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$GITLEAKS_ARGUMENTS_PATH\"\n",
        encoding="utf-8",
    )
    command.chmod(0o755)
    environment = os.environ | {
        "GITLEAKS_ARGUMENTS_PATH": str(arguments_path),
        "GITLEAKS_COMMAND": str(command),
    }

    completed = subprocess.run(
        [
            sys.executable,
            root / "scripts" / "run_history_secret_scan.py",
            "--repository",
            str(repository),
            "--before",
            before,
            "--head",
            head,
        ],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
        env=environment,
    )

    assert completed.returncode == 0, completed.stderr
    assert f"--log-opts={before}..{head}" in arguments_path.read_text(
        encoding="utf-8"
    )
