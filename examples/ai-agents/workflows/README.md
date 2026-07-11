# Agent workflow examples

> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

The supported routine workflow is intentionally narrow:

1. Submit a supported de-identified variant through `classify_variant`.
2. Supply only requested context when the response is `needs_context`.
3. Preserve the structured result, limitations, and source impacts for
   scientific review.
4. Use `explain_classification` only for a stored immutable ID.
5. Use `submit_feedback` only for an anchored review note.

The current package may return `BUNDLE_UNAVAILABLE` because it has no configured
controlled signed catalog. An agent must report that state plainly and must not
invent a classification, evidence, or treatment implication.

The legacy long-form workflow examples in this directory are unavailable as
supported release procedures. They predate the current Python surface and must
not be used as an executable tool-call plan. See
[onboarding](../../../docs/onboarding.md) and the
[capability matrix](../../../docs/release/capabilities.md).
