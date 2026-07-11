# Other AI-system integration status

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

No third-party AI-system integration is validated by this repository beyond a
standards-compliant client launching the installed `acmg-mcp` stdio command.

A client must preserve the server's structured response state and must not use a
language model as authority for normalization, evidence strength, criterion
application, or final classification. Optional specialist review is
non-authoritative and cannot modify a deterministic result.

Do not send patient identifiers, credentials, treatment requests, or raw source
payloads. Do not add a network bridge or service configuration based on older
examples. See [agent integration](README.md),
[onboarding](../../docs/onboarding.md), and
[safety and privacy](../../docs/safety-and-privacy.md).
