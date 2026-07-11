from __future__ import annotations

import importlib
import os
import subprocess
import sys
import tempfile
import tomllib
import unittest
import venv
from pathlib import Path

REPOSITORY_ROOT = Path(__file__).parents[2]


class PackageSmokeTests(unittest.TestCase):
    def test_project_declares_python_package_and_entry_points(self) -> None:
        pyproject_path = REPOSITORY_ROOT / "pyproject.toml"

        with pyproject_path.open("rb") as pyproject_file:
            project = tomllib.load(pyproject_file)["project"]

        self.assertEqual(project["name"], "acmg-classifier")
        self.assertEqual(
            project["scripts"]["acmg"],
            "acmg_classifier.presentation.cli.app:main",
        )
        self.assertEqual(
            project["scripts"]["acmg-mcp"],
            "acmg_classifier.presentation.mcp.server:main",
        )

    def test_docker_installs_dependencies_from_the_reviewed_lockfile(self) -> None:
        dockerfile = (REPOSITORY_ROOT / "Dockerfile").read_text()

        self.assertIn("COPY pyproject.toml uv.lock", dockerfile)
        self.assertIn("RUN uv sync --locked --no-dev --no-editable", dockerfile)
        self.assertNotIn("pip install", dockerfile)

    def test_installer_requires_an_immutable_hashed_wheel_artifact(self) -> None:
        installer = (REPOSITORY_ROOT / "scripts" / "install.sh").read_text()

        self.assertIn(
            'RELEASE_ARTIFACT="${ACMG_RELEASE_ARTIFACT:-}"',
            installer,
        )
        self.assertIn("#sha256=", installer)
        self.assertNotIn("git+", installer)
        self.assertNotIn("git clone", installer)

    def test_installer_preserves_version_validation_and_user_install(self) -> None:
        installer = (REPOSITORY_ROOT / "scripts" / "install.sh").read_text()

        self.assertIn(
            "if not (3, 12) <= sys.version_info[:2] < (3, 14):",
            installer,
        )
        self.assertIn(
            'python3 -m pip install --user --require-hashes --upgrade '
            '"$RELEASE_ARTIFACT"',
            installer,
        )

    def test_installer_fails_closed_without_a_verified_artifact(self) -> None:
        with tempfile.TemporaryDirectory() as temporary_directory:
            binary_directory = Path(temporary_directory)
            (binary_directory / "python3").symlink_to(sys.executable)
            environment = os.environ | {
                "PATH": f"{binary_directory}{os.pathsep}{os.environ['PATH']}",
            }
            result = subprocess.run(
                ["bash", str(REPOSITORY_ROOT / "scripts" / "install.sh")],
                check=False,
                capture_output=True,
                text=True,
                env=environment,
                timeout=30,
            )

        self.assertEqual(result.returncode, 1)
        self.assertIn("no verified immutable release wheel", result.stderr)

    def test_package_exposes_a_version(self) -> None:
        package = importlib.import_module("acmg_classifier")

        self.assertRegex(package.__version__, r"^\d+\.\d+\.\d+$")

    def test_console_entry_points_are_importable(self) -> None:
        cli = importlib.import_module("acmg_classifier.presentation.cli.app")
        server = importlib.import_module("acmg_classifier.presentation.mcp.server")

        self.assertTrue(callable(cli.main))
        self.assertTrue(callable(server.main))

    def test_built_wheel_installs_and_runs_console_help(self) -> None:
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            wheel_directory = temporary_path / "wheel"
            build = subprocess.run(
                [
                    "uv",
                    "build",
                    "--wheel",
                    "--out-dir",
                    str(wheel_directory),
                ],
                cwd=REPOSITORY_ROOT,
                check=False,
                capture_output=True,
                text=True,
                timeout=120,
            )
            self.assertEqual(build.returncode, 0, build.stderr)
            wheels = tuple(wheel_directory.glob("acmg_classifier-*.whl"))
            self.assertEqual(len(wheels), 1)

            environment_directory = temporary_path / "environment"
            venv.EnvBuilder(with_pip=True).create(environment_directory)
            python = environment_directory / (
                "Scripts/python.exe" if os.name == "nt" else "bin/python"
            )
            install = subprocess.run(
                [str(python), "-m", "pip", "install", str(wheels[0])],
                cwd=temporary_path,
                check=False,
                capture_output=True,
                text=True,
                timeout=120,
            )
            self.assertEqual(install.returncode, 0, install.stderr)

            console = environment_directory / (
                "Scripts/acmg.exe" if os.name == "nt" else "bin/acmg"
            )
            environment = os.environ.copy()
            environment.pop("PYTHONPATH", None)
            environment.pop("PYTHONHOME", None)
            help_result = subprocess.run(
                [str(console), "--help"],
                cwd=temporary_path,
                env=environment,
                check=False,
                capture_output=True,
                text=True,
                timeout=30,
            )
            self.assertEqual(help_result.returncode, 0, help_result.stderr)
            self.assertIn(
                "Research-use ACMG/AMP variant classification.",
                help_result.stdout,
            )


if __name__ == "__main__":
    unittest.main()
