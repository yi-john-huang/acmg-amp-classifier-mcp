# ACMG/AMP Classifier documentation

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This documentation describes the current Python package and its explicit
limitations. It does not describe a deployed clinical service.

## Start here

1. [Scientist onboarding](onboarding.md): install from source, check readiness,
   run CLI/MCP commands, and interpret structured states.
2. [Capability support matrix](release/capabilities.md): source-controlled
   implementation, validation, and release-blocker status.
3. [Known limits](known-limits.md): scope, unavailable paths, and external
   prerequisites.
4. [Safety and privacy](safety-and-privacy.md): no-PHI and no-patient-care
   boundaries.
5. [Migration guide](migration.md): legacy Go contract decisions.

## Reference guides

- [CLI and MCP reference](api-documentation.md)
- [Python application architecture](architecture.md)
- [Local SQLite state and bundles](database.md)
- [Troubleshooting](maintenance-troubleshooting.md)
- [Security posture](security-compliance.md)
- [Support policy](support-policy.md)

The default runtime is local Python/SQLite with stdio MCP. It has no configured
controlled signed catalog in this repository checkout, so a first-use
classification truthfully returns `BUNDLE_UNAVAILABLE` rather than a scientific
conclusion. See the [root README](../README.md) for the short entry point.
