# Release readiness decision

> **Current decision: NOT READY for a scientific package release.**
>
> **Research and educational use only. Not for clinical diagnostic use, patient
> care, or treatment decisions.**

The repository-controlled quality gates are green, but they are engineering
evidence only. The canonical matrix remains `release_status: experimental` and
the release evaluator intentionally fails closed until the scientific, signing,
security, platform, usability, performance, and release-owner prerequisites are
complete.

## Check the decision

From the repository root:

```sh
python scripts/check_release_readiness.py --format text
python scripts/check_release_readiness.py --format json
```

Exit status:

| Exit | Meaning |
| --- | --- |
| `0` | Every required gate is explicitly complete and the controlled release state is ready. |
| `1` | The matrix is valid, but one or more release gates remain blocked. |
| `2` | The matrix is missing, malformed, contradictory, or incompatible with the readiness schema. |

The evaluator reads metadata from
[`capabilities.json`](capabilities.json). It does not read raw source payloads,
patient data, credentials, or private keys, and it never upgrades a release gate
because a local test passed.

## Repository-controlled evidence

The following controls are implemented and checked:

- Frozen Python quality checks on Python 3.12 and 3.13.
- Ruff, mypy, lockfile validation, and package builds.
- Direct HTTPS transport coverage with certificate validation, DNS/IP pinning,
  redirect validation, bounded streaming, and proxy controls.
- Locked dependency audit and fail-closed tracked/history secret scans.
- Cross-platform package smoke workflow for Linux, Windows, and macOS.
- Retained synthetic benchmark reports that explicitly state their exclusions.
- A release workflow that runs the readiness evaluator before any future
  publication step and currently has no publication step.
- Protected `develop` and `master` branches requiring review and passing gates.

## Protected branch policy

The GitHub API verification for both `develop` and `master` confirms:

- pull requests and one approval are required;
- stale approvals are dismissed;
- strict required checks include `Quality (Python 3.12)`, `Quality (Python
  3.13)`, `Locked Python dependency audit`, `Fail-closed local secret scan`, and
  `Fail-closed complete Git history secret scan`;
- the six `Package smoke` matrix jobs and `Retain synthetic benchmark evidence`
  are also required;
- administrator enforcement and conversation resolution are enabled;
- force pushes and branch deletion are disabled.

These checks prove reproducible engineering behavior. They do not prove
scientific validity, certification, participant usability, or live production
performance.

## Blocking evidence ledger

| Gate | Owner role | Required evidence | Current state |
| --- | --- | --- | --- |
| 10.1 Legacy compatibility | Release owner | Approved compatibility-window version and end date | Blocked |
| 10.2 Scientific validation | Scientific validation owner | Provenance-approved independently curated validation dataset and retained review | Blocked |
| 10.3 Offline/degraded runtime | Controlled release owner | Compatible installed signed bundle and cache artifacts | Blocked |
| 10.4 Packaging and first use | Platform and usability owner | Linux/Windows clean-install evidence and participant usability record | Blocked |
| 10.5 Security and privacy | Security owner | Independent external security assessment or certification report | Blocked |
| 10.6 Performance and token budgets | Performance owner | Retained live workload and platform benchmark samples | Blocked |
| Candidate bundle | Controlled release owner | Controlled Ed25519 signature, trusted catalog, publication, retention, and revocation records | Blocked |

The synthetic validation fixture, deterministic benchmark suite, local TLS
server, CI runs, and package smoke jobs must remain labeled engineering evidence.
They cannot be substituted for the external artifacts above.

## Controlled release sequence

1. Supply and independently review every evidence artifact in the ledger.
2. Update the corresponding matrix gate `state`, evidence reference, review
   record, candidate signature status, and scientific release gate through the
   controlled release process.
3. Run the evaluator and retain its JSON output with the release record.
4. Run the complete quality, security, package, platform, and benchmark gates.
5. Obtain the release-owner approval and confirm branch protections and review
   history.
6. Only then may a separately authorized publication workflow build, sign, and
   publish an artifact.

No private signing key, restricted source payload, patient data, or invented
external assessment belongs in this repository.
