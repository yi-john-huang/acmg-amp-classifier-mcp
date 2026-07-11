# External chat integration status

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

No HTTP bridge, browser integration, or hosted chat deployment is a supported
release capability. A standards-compliant MCP client may launch `acmg-mcp` over
stdio using the configuration in [the integration overview](README.md).

The client must surface `BUNDLE_UNAVAILABLE`, `needs_context`, `degraded`,
`conflict`, `unsupported`, and `failed` exactly as returned. It must not invent
evidence, diagnose a patient, or suggest treatment.

See [the capability matrix](../../docs/release/capabilities.md) for current
interface status.
