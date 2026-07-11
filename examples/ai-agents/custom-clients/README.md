# Custom clients

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

No custom client library is released or validated by this repository. Use an MCP
client that can launch the installed stdio command:

```json
{
  "command": "acmg-mcp",
  "args": []
}
```

A client should call only the current primary tools:

1. `classify_variant` for one supported, de-identified input and required context.
2. `explain_classification` for a stored immutable classification ID.
3. `submit_feedback` for anchored scientist review.

Treat every structured status as authoritative workflow state. Do not fan out
legacy source-query tools, invent evidence, or turn `BUNDLE_UNAVAILABLE`,
`needs_context`, `degraded`, `conflict`, `unsupported`, or `failed` into a
classification.

The older JavaScript/Python sample files in this directory are not validated for
the migrated Python release and are unavailable as supported integration code.
See [the current MCP surface](../../../docs/api-documentation.md) and the
[capability matrix](../../../docs/release/capabilities.md).
