# Troubleshooting

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

Use structured command output first:

```sh
acmg doctor --format json
acmg data status --format json
```

## `BUNDLE_UNAVAILABLE`

The application state is usable but no compatible signed bundle/catalog is
available. This is expected for the current experimental wheel. Read the
reported repair action as diagnostic information; do not install a manually
assembled or unsigned archive. The shipped wheel has no catalog configuration
or trusted release keys, so it cannot self-repair into a scientific runtime.

## `needs_context`

Provide only the requested interpretation context (for example a transcript,
disease, inheritance, or build) through the supported CLI/MCP request. Do not
invent context to force a conclusion.

## `degraded`

One or more optional sources were unavailable. Inspect `unavailable_sources`,
`source_impacts`, and `limitations`. A degraded response is not proof that
missing evidence is benign or pathogenic.

## `unsupported` or `failed`

Confirm that the input is within intended germline Mendelian SNV/small-indel
scope and read the stable error code. Do not retry by sending patient identifiers,
credentials, raw source payloads, or treatment questions.

## MCP connection

Use `acmg-mcp` through stdio. Check that the client launches the installed console
entry point and that it does not write protocol noise to stdout. See
[onboarding](onboarding.md) and [API reference](api-documentation.md).

## Escalation

Capture the structured status, error code, and non-sensitive limitations. Do not
include patient information or credentials. See [safety and privacy](safety-and-privacy.md)
and [support policy](support-policy.md).
