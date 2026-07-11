"""Deterministic bundle compatibility selection."""

from dataclasses import dataclass

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest


@dataclass(frozen=True, slots=True)
class BundleRequirements:
    """Runtime capabilities required from a data bundle."""

    application_version: str
    schema_version: str
    genome_build: GenomeBuild | str
    ruleset_id: str
    ruleset_version: str
    pinned_bundle_version: str | None = None


@dataclass(frozen=True, slots=True)
class BundleRejection:
    """A rejected candidate and every incompatibility found."""

    bundle_version: str
    reasons: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class BundleSelection:
    """Best compatible candidate and incompatible alternatives."""

    selected: BundleManifest | None
    rejected: tuple[BundleRejection, ...]


def select_bundle(
    manifests: tuple[BundleManifest, ...],
    requirements: BundleRequirements,
) -> BundleSelection:
    """Select the newest compatible bundle and explain every rejection."""
    compatible: list[BundleManifest] = []
    rejected: list[BundleRejection] = []
    for manifest in manifests:
        reasons = _incompatibilities(manifest, requirements)
        if reasons:
            rejected.append(BundleRejection(manifest.bundle_version, reasons))
        else:
            compatible.append(manifest)
    selected = max(
        compatible, key=lambda item: _version(item.bundle_version), default=None
    )
    return BundleSelection(
        selected=selected,
        rejected=tuple(
            sorted(rejected, key=lambda item: _version(item.bundle_version))
        ),
    )


def _incompatibilities(
    manifest: BundleManifest,
    requirements: BundleRequirements,
) -> tuple[str, ...]:
    reasons: list[str] = []
    if (
        requirements.pinned_bundle_version is not None
        and manifest.bundle_version != requirements.pinned_bundle_version
    ):
        reasons.append(
            f"bundle version {manifest.bundle_version} does not match pin "
            f"{requirements.pinned_bundle_version}"
        )
    if not _in_range(
        requirements.application_version,
        manifest.application_version.minimum,
        manifest.application_version.maximum,
    ):
        reasons.append(
            f"application version {requirements.application_version} is outside "
            f"{manifest.application_version.minimum} to "
            f"{manifest.application_version.maximum}"
        )
    if not _in_range(
        requirements.schema_version,
        manifest.schema_version.minimum,
        manifest.schema_version.maximum,
    ):
        reasons.append(
            f"schema version {requirements.schema_version} is outside "
            f"{manifest.schema_version.minimum} to {manifest.schema_version.maximum}"
        )
    required_build = GenomeBuild(requirements.genome_build)
    if required_build not in manifest.genome_builds:
        reasons.append(f"genome build {required_build.value} is not included")
    if not any(
        ruleset.identifier == requirements.ruleset_id
        and ruleset.version == requirements.ruleset_version
        for ruleset in manifest.rulesets
    ):
        reasons.append(
            f"ruleset {requirements.ruleset_id} {requirements.ruleset_version} "
            "is not included"
        )
    return tuple(reasons)


def _in_range(value: str, minimum: str, maximum: str) -> bool:
    return _version(minimum) <= _version(value) <= _version(maximum)


def _version(value: str) -> tuple[int, int, int]:
    parts = [int(part) for part in value.split(".")]
    padded = [*parts, 0, 0, 0][:3]
    return padded[0], padded[1], padded[2]
