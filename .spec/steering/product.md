# Product Steering

## Product scope

ACMG AMP Classifier is a research and educational tool for reproducible,
scientist-facing interpretation of supported germline Mendelian SNVs and small
indels. It is not approved for clinical diagnosis, patient care, or treatment
decisions.

## Primary workflow

A researcher submits a supported variant through the `acmg` CLI or the
`classify_variant` MCP tool. A configured release catalog is required before the
system can normalize input, gather versioned evidence, evaluate criteria, and
store an immutable result. The current wheel ships without that catalog, so
first use returns typed `BUNDLE_UNAVAILABLE` rather than a classification.

## Product constraints

- Runtime interaction is local CLI and stdio MCP; no hosted HTTP API is
  supported.
- SQLite and compatible signed data bundles are required local state.
- Outputs must preserve provenance, uncertainty, limitations, and non-clinical
  disclaimer language.
- Feedback is append-only and non-authoritative.
- Raw source payloads are advanced-only and require explicit references.

## Out of scope

Somatic, CNV, structural, fusion, mitochondrial, pharmacogenomic, and clinical
workflow automation are unsupported. Legacy Go transport, PostgreSQL, Redis,
Docker Swarm, and Kubernetes deployment are retired.
