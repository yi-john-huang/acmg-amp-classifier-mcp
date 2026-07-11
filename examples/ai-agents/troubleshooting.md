# Agent integration troubleshooting

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

## Client cannot start the server

Confirm the client launches the installed `acmg-mcp` command over stdio. Do not
supply service URLs, database credentials, API keys, or a legacy executable path.
Use `acmg-mcp --help` locally to confirm the console entry point is installed.

## The request returns `BUNDLE_UNAVAILABLE`

This is the expected safe state when the experimental package has no configured
controlled signed catalog. Report the status and repair guidance. Do not create a
fixture, mock evidence, or substitute archive to force a classification.

## The request returns another non-completed status

- `needs_context`: ask only for the requested interpretation context.
- `degraded`: show unavailable source and criterion-impact information.
- `conflict`: preserve the conflict; optional review is not a decision override.
- `unsupported` or `failed`: show the stable error code and next action.

Do not include patient identifiers, credentials, or patient-care/treatment
requests in a retry. See [current onboarding](../../docs/onboarding.md) and
[safety guidance](../../docs/safety-and-privacy.md).
