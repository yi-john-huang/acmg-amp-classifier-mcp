"""SDK-free, opt-in translation decisions for retired Go MCP contracts."""

from __future__ import annotations

import json
import re
from collections.abc import Mapping
from importlib.resources import files
from typing import Any, cast

_SCHEMA_VERSION = "1.0"
_CURRENT_PRIMARY_TOOLS = (
    "classify_variant",
    "explain_classification",
    "submit_feedback",
)
_COMPATIBLE_TRANSCRIPT = re.compile(r"^(?:N[MR]_\d+\.\d+|ENST\d+\.\d+)$")
_SUPPORTED_LEGACY_VARIANT_TYPES = frozenset({"SNV", "indel"})
_UNSUPPORTED_LEGACY_VARIANT_TYPES = frozenset({"CNV", "SV", "fusion"})
_MOCK_FIELD_TOKENS = frozenset({"mock", "synthetic", "fixture", "placeholder"})
_FABRICATED_RESULT_FIELDS = frozenset(
    {
        "classification",
        "evidence",
        "evidence_summary",
        "recommendations",
        "applied_rules",
    }
)
_PHI_LIKE_FIELD_NAMES = frozenset(
    {
        "patient",
        "patientid",
        "patientname",
        "patientidentifier",
        "medicalrecordnumber",
        "mrn",
        "dateofbirth",
        "dob",
        "clinicalindication",
        "familyhistory",
        "ethnicity",
        "consanguinity",
        "referringphysician",
        "testdate",
    }
)
_REPORT_TOOLS = frozenset({"generate_report", "format_report", "validate_report"})
_LOW_LEVEL_EVIDENCE_TOOLS = frozenset(
    {
        "query_evidence",
        "batch_query_evidence",
        "query_clinvar",
        "query_gnomad",
        "query_cosmic",
    }
)


def _load_manifest() -> dict[str, Any]:
    """Load the source-controlled machine-readable compatibility decisions."""
    manifest_path = files(__package__).joinpath("legacy_go_manifest.json")
    loaded = json.loads(manifest_path.read_text(encoding="utf-8"))
    if not isinstance(loaded, dict):
        raise ValueError("legacy compatibility manifest must be an object")
    return cast(dict[str, Any], loaded)


class LegacyGoCompatibilityAdapter:
    """Classify, but never execute, legacy Go request compatibility decisions.

    The adapter deliberately has no SDK or application-service dependency. Callers
    explicitly invoke it and, for a mapped request, invoke the current workflow.
    """

    _manifest = _load_manifest()

    @property
    def manifest(self) -> Mapping[str, Any]:
        """Return the source-controlled decision inventory without MCP registration."""
        return self._manifest

    @property
    def public_legacy_tools(self) -> tuple[str, ...]:
        """Return every captured Go public tool in manifest order."""
        tools = self._manifest["tools"]
        assert isinstance(tools, Mapping)
        return tuple(tools)

    @property
    def primary_public_tools(self) -> tuple[str, ...]:
        """Expose the unchanged current primary tool surface for contract checks."""
        return _CURRENT_PRIMARY_TOOLS

    def adapt(
        self, legacy_tool: str, request: Mapping[str, object] | object
    ) -> dict[str, object]:
        """Return a migration envelope for one legacy request without executing it."""
        if not isinstance(request, Mapping):
            return self._rejected(
                legacy_tool,
                error_code="INVALID_LEGACY_REQUEST",
                decision="REJECT_INVALID_REQUEST_SHAPE",
                replacement=self._replacement("classify_variant"),
                limitations=("Legacy requests must be JSON objects.",),
            )

        if legacy_tool == "classify_variant":
            return self._adapt_classify_request(request)
        if legacy_tool == "submit_feedback":
            return self._adapt_feedback_request(request)
        if legacy_tool in _REPORT_TOOLS:
            return self._adapt_report_request(legacy_tool, request)
        if legacy_tool in _LOW_LEVEL_EVIDENCE_TOOLS:
            return self._rejected_from_manifest(
                legacy_tool,
                error_code="UNAVAILABLE_SOURCE",
            )
        if legacy_tool in self.public_legacy_tools:
            return self._rejected_from_manifest(
                legacy_tool,
                error_code="DEPRECATED_TOOL",
            )
        return self._rejected(
            legacy_tool,
            error_code="DEPRECATED_TOOL",
            decision="REJECT_UNKNOWN_LEGACY_TOOL",
            replacement=self._replacement("classify_variant"),
            limitations=(
                "This legacy tool is not part of the captured migration contract.",
            ),
        )

    def _adapt_classify_request(
        self, request: Mapping[str, object]
    ) -> dict[str, object]:
        field_names = frozenset(request)
        mock_or_fabricated_fields = tuple(
            sorted(
                field
                for field in field_names
                if field in _FABRICATED_RESULT_FIELDS
                or any(token in field.lower() for token in _MOCK_FIELD_TOKENS)
            )
        )
        if mock_or_fabricated_fields or request.get("include_evidence") is True:
            return self._rejected(
                "classify_variant",
                error_code="UNSUPPORTED_LEGACY_BEHAVIOR",
                decision="REJECT_MOCK_OR_FABRICATED_CONTENT",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Legacy mock evidence, result fields, and evidence-expanded "
                    "output are not preserved.",
                ),
            )

        if "gene_symbol_notation" in field_names or "gene_symbol" in field_names:
            return self._rejected(
                "classify_variant",
                error_code="AMBIGUOUS_LEGACY_REQUEST",
                decision="REJECT_AMBIGUOUS_VARIANT_INPUT",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Gene-symbol legacy inputs do not identify one supported "
                    "variant unambiguously.",
                ),
            )

        deprecated_fields = field_names & {
            "clinical_context",
            "variant",
            "transcript",
        }
        if deprecated_fields:
            return self._rejected(
                "classify_variant",
                error_code="DEPRECATED_FIELD",
                decision="REJECT_DEPRECATED_FIELD",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Deprecated legacy fields are not silently translated into "
                    "research context.",
                ),
            )

        unknown_fields = field_names - {
            "hgvs_notation",
            "transcript_id",
            "preferred_isoform",
            "variant_type",
            "include_evidence",
        }
        if unknown_fields:
            return self._rejected(
                "classify_variant",
                error_code="INVALID_LEGACY_REQUEST",
                decision="REJECT_UNRECOGNIZED_FIELD",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "The request contains fields without a safe migration decision.",
                ),
            )

        variant_type = request.get("variant_type")
        if variant_type in _UNSUPPORTED_LEGACY_VARIANT_TYPES:
            return self._rejected(
                "classify_variant",
                error_code="UNSUPPORTED_LEGACY_BEHAVIOR",
                decision="REJECT_UNSUPPORTED_VARIANT_TYPE",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Only germline Mendelian SNVs and small indels are in the "
                    "current release scope.",
                ),
            )
        if (
            variant_type is not None
            and variant_type not in _SUPPORTED_LEGACY_VARIANT_TYPES
        ):
            return self._rejected(
                "classify_variant",
                error_code="INVALID_LEGACY_REQUEST",
                decision="REJECT_INVALID_VARIANT_TYPE",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Legacy variant_type must be SNV or indel when supplied.",
                ),
            )

        variant = request.get("hgvs_notation")
        if not isinstance(variant, str) or not variant.strip():
            return self._rejected(
                "classify_variant",
                error_code="INVALID_LEGACY_REQUEST",
                decision="REJECT_MISSING_VARIANT",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "A non-empty hgvs_notation is required for legacy translation.",
                ),
            )

        transcript_result = self._compatible_transcript(request)
        if isinstance(transcript_result, dict):
            return transcript_result

        mapped_request: dict[str, object] = {"variant": variant}
        if transcript_result is not None:
            mapped_request["transcript"] = transcript_result
        limitations = list(self._manifest_limitations("classify_variant"))
        if request.get("include_evidence") is False:
            limitations.append(
                "include_evidence=false is ignored; evidence rendering is "
                "current-tool controlled."
            )
        if variant_type is not None:
            limitations.append("variant_type is scope-checked and not forwarded.")
        return self._mapped(
            "classify_variant",
            decision="MAP_SAFE_CLASSIFY_REQUEST",
            replacement=self._manifest_replacement("classify_variant"),
            mapped_request=mapped_request,
            limitations=tuple(limitations),
        )

    def _compatible_transcript(
        self, request: Mapping[str, object]
    ) -> str | dict[str, object] | None:
        supplied = {
            field: request[field]
            for field in ("transcript_id", "preferred_isoform")
            if field in request
        }
        if not supplied:
            return None
        if any(
            not isinstance(transcript, str)
            or _COMPATIBLE_TRANSCRIPT.fullmatch(transcript) is None
            for transcript in supplied.values()
        ):
            return self._rejected(
                "classify_variant",
                error_code="INVALID_LEGACY_REQUEST",
                decision="REJECT_INCOMPATIBLE_TRANSCRIPT_ALIAS",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Only NM_, NR_, and ENST transcript aliases with explicit "
                    "versions can be translated.",
                ),
            )
        values = set(supplied.values())
        if len(values) != 1:
            return self._rejected(
                "classify_variant",
                error_code="AMBIGUOUS_LEGACY_REQUEST",
                decision="REJECT_CONFLICTING_TRANSCRIPT_ALIASES",
                replacement=self._replacement("classify_variant"),
                limitations=(
                    "Conflicting transcript aliases require an explicit current "
                    "transcript selection.",
                ),
            )
        return cast(str, next(iter(values)))

    def _adapt_feedback_request(
        self, request: Mapping[str, object]
    ) -> dict[str, object]:
        if not isinstance(request.get("classification_id"), str):
            return self._rejected(
                "submit_feedback",
                error_code="MISSING_CLASSIFICATION_ID",
                decision="REJECT_UNANCHORED_FEEDBACK",
                replacement=self._manifest_replacement("submit_feedback"),
                limitations=self._manifest_limitations("submit_feedback"),
            )
        return self._rejected(
            "submit_feedback",
            error_code="DEPRECATED_TOOL",
            decision="REJECT_LEGACY_FEEDBACK_SCHEMA",
            replacement=self._manifest_replacement("submit_feedback"),
            limitations=(
                "Legacy feedback fields are not preserved; use the current "
                "anchored feedback schema.",
            ),
        )

    def _adapt_report_request(
        self, legacy_tool: str, request: Mapping[str, object]
    ) -> dict[str, object]:
        if self._contains_phi_like_field(request):
            return self._rejected(
                legacy_tool,
                error_code="PRIVACY_REJECTED",
                decision="REJECT_PHI_LIKE_REPORT_REQUEST",
                replacement=self._manifest_replacement(legacy_tool),
                limitations=(
                    "PHI-like report requests are rejected and their content is "
                    "not retained in the envelope.",
                ),
            )
        return self._rejected_from_manifest(legacy_tool, error_code="DEPRECATED_TOOL")

    def _rejected_from_manifest(
        self, legacy_tool: str, *, error_code: str
    ) -> dict[str, object]:
        tool_decision = self._tool_decision(legacy_tool)
        return self._rejected(
            legacy_tool,
            error_code=error_code,
            decision=tool_decision["decision"],
            replacement=tool_decision["replacement"],
            limitations=tool_decision["limitations"],
        )

    def _tool_decision(self, legacy_tool: str) -> Mapping[str, Any]:
        tools = self._manifest["tools"]
        assert isinstance(tools, Mapping)
        decision = tools[legacy_tool]
        assert isinstance(decision, Mapping)
        return decision

    def _manifest_replacement(self, legacy_tool: str) -> Mapping[str, Any]:
        replacement = self._tool_decision(legacy_tool)["replacement"]
        if not isinstance(replacement, Mapping):
            raise ValueError("legacy compatibility replacement must be an object")
        return cast(Mapping[str, Any], replacement)

    def _manifest_limitations(self, legacy_tool: str) -> tuple[str, ...]:
        limitations = self._tool_decision(legacy_tool)["limitations"]
        assert isinstance(limitations, list)
        return tuple(str(limitation) for limitation in limitations)

    @staticmethod
    def _replacement(tool: str) -> dict[str, str]:
        return {"tool": tool}

    @staticmethod
    def _contains_phi_like_field(value: object) -> bool:
        if isinstance(value, Mapping):
            return any(
                re.sub(r"[^a-z0-9]", "", str(key).lower()) in _PHI_LIKE_FIELD_NAMES
                or LegacyGoCompatibilityAdapter._contains_phi_like_field(child)
                for key, child in value.items()
            )
        if isinstance(value, (list, tuple)):
            return any(
                LegacyGoCompatibilityAdapter._contains_phi_like_field(child)
                for child in value
            )
        return False

    @staticmethod
    def _mapped(
        legacy_tool: str,
        *,
        decision: str,
        replacement: Mapping[str, Any],
        mapped_request: Mapping[str, object],
        limitations: tuple[str, ...],
    ) -> dict[str, object]:
        return {
            "schema_version": _SCHEMA_VERSION,
            "status": "mapped",
            "error_code": None,
            "legacy_tool": legacy_tool,
            "compatibility_decision": decision,
            "replacement": dict(replacement),
            "limitations": list(limitations),
            "mapped_request": dict(mapped_request),
        }

    @staticmethod
    def _rejected(
        legacy_tool: str,
        *,
        error_code: str,
        decision: str,
        replacement: Mapping[str, Any],
        limitations: tuple[str, ...],
    ) -> dict[str, object]:
        return {
            "schema_version": _SCHEMA_VERSION,
            "status": "rejected",
            "error_code": error_code,
            "legacy_tool": legacy_tool,
            "compatibility_decision": decision,
            "replacement": dict(replacement),
            "limitations": list(limitations),
        }
