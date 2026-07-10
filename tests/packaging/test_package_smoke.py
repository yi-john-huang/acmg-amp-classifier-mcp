from __future__ import annotations

import importlib
import tomllib
import unittest
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

    def test_package_exposes_a_version(self) -> None:
        package = importlib.import_module("acmg_classifier")

        self.assertRegex(package.__version__, r"^\d+\.\d+\.\d+$")

    def test_cli_entry_point_is_callable(self) -> None:
        cli = importlib.import_module("acmg_classifier.presentation.cli.app")

        self.assertTrue(callable(cli.main))
        self.assertEqual(cli.main(), 0)

    def test_mcp_entry_point_is_callable(self) -> None:
        server = importlib.import_module("acmg_classifier.presentation.mcp.server")

        self.assertTrue(callable(server.main))
        self.assertEqual(server.main(), 0)


if __name__ == "__main__":
    unittest.main()
