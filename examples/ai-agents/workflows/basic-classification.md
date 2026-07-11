# Basic classification example status

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This legacy example is unavailable as an executable release workflow. It
predates the current Python MCP surface and must not be used to infer evidence,
classification, diagnosis, or treatment.

Use the current routine described in [agent workflow examples](README.md): one
`classify_variant` call after required context is known, followed by structured
status handling. In the current experimental runtime, a request can truthfully
return `BUNDLE_UNAVAILABLE` until a compatible signed catalog is supplied.

See [onboarding](../../../docs/onboarding.md) and
[known limits](../../../docs/known-limits.md).
