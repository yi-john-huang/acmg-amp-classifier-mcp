# Safety and privacy

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This project supports scientific review workflows. It does not authorize a
clinical diagnosis, patient-care decision, therapy selection, or treatment
recommendation.

## Do not submit sensitive data

The default workflow does not need patient identifiers. Do not submit names,
medical-record numbers, dates of birth, addresses, credential strings, access
tokens, authentication headers, database URLs with credentials, or treatment
questions. A PHI-like field is rejected where the presentation contract accepts
it, and report-boundary redaction removes recognized credential/PHI-like keys
from untrusted rendered content.

Redaction is a safety control, not a permission to submit sensitive information.
Use only de-identified research inputs and follow your institution's approved
privacy process.

## Implemented controls

- Input models constrain supported scope and return structured error codes.
- Source HTTP policy requires certificate validation and constrains redirects,
  response size, and error details.
- Bundle verification validates archive boundaries, hashes, signatures, and
  staging/activation state before use.
- Report-boundary redaction and resource-size controls bound untrusted output.
- The repository contains focused abuse tests and an automated locked-dependency
  audit plus reviewed secret-detection workflow.

Evidence is recorded in the [capability support matrix](release/capabilities.md).
These controls do not establish an external security certification, institutional
approval, or clinical compliance posture.

## Reporting a vulnerability

Do not include a secret, patient identifier, or exploit payload in an issue.
Contact the project maintainer through the repository's private security contact
or your organization’s approved reporting channel. Until a published response
and update policy is adopted, the support-policy fields remain an external
prerequisite; see [support policy](support-policy.md).

## Interpretation safety

A `completed` workflow response records a deterministic software decision for
review; it is not clinical authorization. A `degraded`, `conflict`,
`needs_context`, `unsupported`, or `failed` response must not be coerced into a
five-tier conclusion. In particular, `BUNDLE_UNAVAILABLE` means a scientific
runtime was not available.

See [known limits](known-limits.md) and [scientist onboarding](onboarding.md)
for current status handling.
