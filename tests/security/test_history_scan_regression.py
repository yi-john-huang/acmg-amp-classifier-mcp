from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest


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


def test_history_scanner_does_not_exempt_regression_script_path(
    tmp_path: Path,
) -> None:
    """A credential in the regression script path must remain detectable."""
    gitleaks_command = shutil.which("gitleaks")
    if gitleaks_command is None:
        pytest.skip("Gitleaks is required to verify the shared scanner policy")

    root = Path(__file__).resolve().parents[2]
    repository = tmp_path / "repository"
    fixture_path = repository / "scripts" / "verify_history_scan_regression.py"
    fixture_path.parent.mkdir(parents=True)
    credential = "".join(("A", "KIA", "QWERTYUIOPASDFGH"))
    fixture_path.write_text(
        (root / "scripts" / fixture_path.name).read_text(encoding="utf-8")
        + f'\nfixture_credential = "{credential}"\n',
        encoding="utf-8",
    )
    _run_git(repository, "init", "--quiet")
    _run_git(repository, "config", "user.email", "security-test@example.invalid")
    _run_git(repository, "config", "user.name", "Security Regression")
    _run_git(repository, "add", fixture_path.relative_to(repository).as_posix())
    _run_git(repository, "commit", "--quiet", "-m", "add fixture credential")

    environment = os.environ | {"GITLEAKS_COMMAND": gitleaks_command}
    completed = subprocess.run(
        [
            sys.executable,
            root / "scripts" / "run_history_secret_scan.py",
            "--repository",
            str(repository),
        ],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
        env=environment,
    )

    assert completed.returncode != 0, completed.stderr