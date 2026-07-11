# AI-agent integration examples

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

These examples describe the current Python stdio MCP surface. They do not
provide a hosted bridge, patient-data workflow, or clinical recommendation.

## Configure a client

Use the installed `acmg-mcp` command:

```json
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "acmg-mcp",
      "args": []
    }
  }
}
```

The primary tools are `classify_variant`, `explain_classification`, and
`submit_feedback`. `get_raw_snapshot` is advanced mode only. The routine skill
uses one classification workflow after required context is known; it does not
orchestrate legacy low-level evidence queries.

The current experimental runtime has no configured controlled signed catalog, so
an attempted classification may return `BUNDLE_UNAVAILABLE`. This is not a
variant conclusion and must not be rewritten into a result by an agent.

## Safe behavior

- Use de-identified research inputs only.
- Do not send credentials, patient identifiers, treatment requests, or raw
  source payloads.
- Preserve structured `status`, error codes, limitations, and source impacts.
- Treat `needs_context`, `degraded`, `conflict`, `unsupported`, and `failed` as
  distinct workflow states, not conclusions to infer around.

See [onboarding](../../docs/onboarding.md),
[capabilities](../../docs/release/capabilities.md), and
[safety and privacy](../../docs/safety-and-privacy.md).
