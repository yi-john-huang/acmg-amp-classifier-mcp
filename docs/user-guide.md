# Research workflow guide

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

This guide is intentionally narrower than a clinical-laboratory workflow. The
software records deterministic scientific inputs and workflow states for review;
it does not provide a web interface, clinical authorization, or treatment
recommendations.

## Install from a source checkout

```sh
uv tool install .
acmg --help
```

The documented commands create local SQLite state automatically; no service
endpoint, database schema, cache address, or credential is required.

## 1. Verify readiness

```sh
acmg doctor --format json
acmg data status --format json
```

On clean first use, `BUNDLE_UNAVAILABLE` is an honest readiness state: the
current Python package has no configured controlled signed catalog or trusted
release keys. Do not replace it with an unverified archive or a synthetic
fixture. `doctor --repair` is diagnostic only in the default wheel.

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

## 5. Other commands and offline use

```sh
acmg feedback CLASSIFICATION_ID --type agreement --rationale 'review note' --actor-id research-user --format json
acmg feedback-export --format json
acmg data status --format json
```

`data install` deliberately rejects manual or unsigned archive installation.
`--offline` prevents remote normalization and supports replay/degraded behavior
only with compatible local data. Unavailable sources are reported with affected
criteria; missing evidence is never silently treated as absence evidence.

Read [onboarding](onboarding.md), [safety and privacy](safety-and-privacy.md),
and the [capability support matrix](release/capabilities.md) before use.
