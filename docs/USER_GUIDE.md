# Command user guide

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

Use the Python console commands from a source checkout:

```sh
uv tool install --from . acmg-classifier
acmg --help
acmg doctor --format json
```

The default application creates local SQLite state automatically. No manual
service endpoint, database schema, cache address, or credential is required for
the documented commands.

## Classify an input

```sh
acmg classify 'NM_000059.4(BRCA2):c.7008-1G>A' --no-interactive --format json
```

Inputs in intended scope are germline Mendelian SNVs and small indels. At present
the distributed runtime has no configured controlled signed catalog, so this
command returns `BUNDLE_UNAVAILABLE` rather than a scientific result. Do not
interpret that safety state as a variant classification.

When a compatible runtime is supplied, `needs_context`, `degraded`, `conflict`,
`unsupported`, and `failed` are distinct non-completed outcomes. Read their
limitations and stable error codes.

## Other commands

```sh
acmg explain CLASSIFICATION_ID --format json
acmg feedback CLASSIFICATION_ID --type agreement --rationale 'review note' --actor-id research-user --format json
acmg feedback-export --format json
acmg data status --format json
```

Feedback is anchored to immutable classifications; it is not a mechanism for
changing an existing decision. `data install` deliberately rejects manual or
unsigned archive installation. Follow a release administrator's verified bundle
procedure only when one is published.

## Offline use

`--offline` prevents remote normalization. It supports replay/degraded behavior
only with compatible local data. Unavailable sources are reported with affected
criteria; missing evidence is never silently treated as absence evidence.

See [onboarding](onboarding.md), [known limits](known-limits.md), and the
[capability support matrix](release/capabilities.md).
