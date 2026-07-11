# Research workflow guide

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This guide is intentionally narrower than a clinical-laboratory workflow. The
software records deterministic scientific inputs and workflow states for review;
it does not provide a web interface, clinical authorization, or treatment
recommendations.

## 1. Verify readiness

```sh
acmg doctor --format json
acmg data status --format json
```

A `BUNDLE_UNAVAILABLE` result is an honest readiness state: the current Python
package has no configured controlled signed catalog. Do not replace it with an
unverified archive or a synthetic fixture.

## 2. Submit a de-identified research input

```sh
acmg classify 'NM_000059.4(BRCA2):c.7008-1G>A' --no-interactive --format json
```

Do not include a patient name, date of birth, medical-record number, credentials,
or a patient-care question. The intended scope is germline Mendelian SNVs and
small indels.

## 3. Interpret workflow status

A response may be `completed`, `needs_context`, `degraded`, `conflict`,
`unsupported`, or `failed`. Only a completed software workflow has a stored
classification record; it still requires qualified scientific review and is not
a clinical result. `degraded` reports unavailable sources and criterion impact.
`needs_context` tells you which interpretation context is missing.

## 4. Preserve review context

Use `acmg explain CLASSIFICATION_ID --format json` for a stored record and
anchored feedback commands for an auditable review note. Feedback does not alter
the stored decision.

Read [onboarding](onboarding.md), [safety and privacy](safety-and-privacy.md),
and the [capability support matrix](release/capabilities.md) before use.
