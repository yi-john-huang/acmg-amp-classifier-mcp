# Local state, records, and bundles

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

The documented default runtime uses application-owned SQLite files. It does not
require an operator-managed database service.

## What is stored

SQLite state supports bootstrap state, immutable classification records, drafts,
and anchored feedback records. Classification records preserve canonical request
content, normalized variant, interpretation context, evidence snapshot identity,
ruleset identity, decision, explanation, and software/bundle versions. Feedback
is append-only and does not change a stored decision.

## Bundles and cache state

A compatible bundle is verified before activation. Verification covers archive
boundaries, hashes, signatures, and atomic staging/activation. Candidate metadata
is available under `data_builder/`, but it has a controlled-release-key
requirement and is not a substitute for an installed signed catalog.

The normal first-use state in this experimental checkout is
`BUNDLE_UNAVAILABLE`. Use `acmg doctor --format json` and `acmg data status
--format json` to inspect readiness. Do not manually edit SQLite records or copy
unverified data into the application directory.

## Offline and replay

Stored snapshot replay is deterministic and does not query providers. Offline
classification/degraded operation requires compatible local bundle/cache state;
it never turns an unavailable source into absence evidence.

See [onboarding](onboarding.md), [known limits](known-limits.md), and the
[capability support matrix](release/capabilities.md).
