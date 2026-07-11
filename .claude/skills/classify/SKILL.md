---
name: classify
description: Classify one ACMG/AMP variant through the primary MCP workflow.
---

# Classify a Variant

Use exactly one primary MCP tool: `classify_variant`. This is research use only; never present a result as clinical diagnosis.

## Start

Call with the scientist’s variant and only supplied interpretation context:

```json
{
  "variant": "<HGVS or supported notation>",
  "interactive": true
}
```

Omit optional fields rather than guessing them. Set `offline: true` only when the user requests offline operation. Set `agent_review: true` only when the scientist explicitly requests optional specialist synthesis and the host supports it. Do not invent evidence, context, a canonical allele, or a final classification.

## Continue required context

If status is `needs_context`, show the returned question and its reason. Ask for one answer. Resume with the opaque token exactly as returned:

```json
{
  "resume_token": "<returned token>",
  "answers": [
    {
      "field": "<returned field>",
      "state": "provided",
      "value": "<scientist answer>"
    }
  ]
}
```

For an unavailable answer, use `state` `unknown`, `unavailable`, or `not_applicable` and omit `value`. Never alter the token, draft ID, question, or answer schema. Repeat only while the response remains `needs_context`.

## Render the result

- `completed`: report classification, normalized variant, selected context, concise explanation, limitations, and the research-use disclaimer.
- `degraded`: report the provisional classification only with unavailable sources and limitations.
- `conflict`: report that no five-tier classification was produced and name the explicit conflict.
- `unsupported` or `failed`: report the stable reason/error code and next actionable step; do not speculate.

Use compact structured content by default. Load an explanation or resource by identifier only when the scientist asks for more detail.
