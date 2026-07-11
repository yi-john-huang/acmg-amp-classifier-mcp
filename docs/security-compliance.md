# Security and privacy posture

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This page describes implemented local controls and their evidence. It does not
claim an external certification, regulatory approval, laboratory accreditation,
or institutional compliance assessment.

## Implemented, test-backed controls

- Input validation and structured failures for unsupported requests.
- Report-boundary handling for credential/PHI-like keys and bounded untrusted
  content.
- TLS certificate validation, redirect constraints, and source response-size
  limits in the HTTP policy.
- Archive/member/path, digest, signature, and staging checks for bundles.
- Focused abuse tests for malformed URLs, raw-resource limits, source transport,
  malicious bundles, and research-safety language.
- A locked runtime dependency audit and reviewed secret-detection workflow in
  continuous integration configuration.

See the evidence paths and limitations in the
[capability support matrix](release/capabilities.md).

## Not claimed

The project does not claim clinical authorization, patient-data approval,
independent penetration testing, external security certification, or a published
institutional incident-response SLA. Those require separately retained evidence
and owner decisions; see [support policy](support-policy.md).

## Operator and researcher duties

Do not send patient identifiers, credentials, or patient-care/treatment requests
to the default research workflow. Use de-identified research inputs, validate a
bundle before use, and preserve structured status/limitations with a review
record. Do not treat output redaction as permission to submit sensitive data.

Read [safety and privacy](safety-and-privacy.md) and
[known limits](known-limits.md).
