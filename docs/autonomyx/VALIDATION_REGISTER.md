# Autonomyx — Validation Register

**DRAFT · unsigned.** A register of reference protocols and the validation checks they
authorize, staged by maturity. **The founder pins; the system does not enforce ahead of the
pinned maturity.** Kubernetes' own alpha → beta → GA ladder is the maturity model. Items are
**candidate pins** until the founder pins them and (where external) a dated snapshot is captured.

## Reference #1 — Kubernetes (source: uploaded Overview snapshot `sha256:c5283d74…`)

| # | Validation protocol | Our doctrine | Enforce-at |
|---|---|---|---|
| 1 | Server-side validation + admission (`kubectl apply --dry-run=server`); OpenAPI schemas are incomplete, the API server is the real gate | the gate is the judge / contract parser | GA |
| 2 | Desired-state vs status reconcile; object = "record of intent" | Kube is mind at work; the loop | GA |
| 3 | Mixing management techniques = undefined behavior | gap-closure / no undefined behavior | beta |
| 4 | Identity: RFC 1123/1035 names; UID = UUID (ISO/IEC 9834-8, ITU-T X.667) | no dangling identities | GA |
| 5 | Reserved prefixes; namespace isolation; no cross-namespace owner refs | domain-bound scope; islands; infinite-boundary-invalid | beta |
| 6 | Node-lease heartbeats (kubelet → control plane detects failure) | heartbeat is evidence; no faked heartbeat | GA |
| 7 | Finalizers; "cannot resurrect, only make a new object" | sign-forward; recovery path; no bulldozing | beta |
| 8 | Single storage version; two stored versions = invalid | canonical core; ambiguity invalid | GA |
| 9 | Compatibility + deprecation policy; version skew ±1 | sign-forward; no breaking change | beta |
| 10 | alpha → beta → GA + feature gates | the maturity ladder itself | the framework |

## Root layer — Information Technology authorities (candidate pins)

The constraint: **CNCF-mature** components, tracing up to canonical IT standards.

| Authority | Standard | Where the platform embodies it | Enforce-at |
|---|---|---|---|
| NIST | FIPS 180-4 (SHA-2) | Box digest is `sha256:` | GA |
| NIST | SP 800-207 Zero Trust | default-deny gate; derived scope | GA |
| CNCF | CloudEvents | the core's one event contract | GA |
| CNCF | Cloud Native Buildpacks; Cedar; k8s; OPA | build + policy + runtime | beta |
| IETF | RFC 8259 JSON · 4253 SSH sig · 1123 DNS | wire format; signed commits; naming | GA |
| ISO/IEC JTC 1 | 9834-8 UUID · 25010 quality · 27001 ISMS · 42001 AI mgmt | ids; eval; governance | beta |
| OWASP | LLM Top 10 (prompt injection) | the injection screen | beta |
| SLSA / Sigstore | provenance L2–3; Fulcio/Rekor | the release pipeline | beta |

## Staging rule
Pin a check at **alpha** (advisory / warn-soft) → promote to **beta** (warn-loud) → **GA**
(hard-fail). Mirrors `kubectl --validate=ignore|warn|strict`.

## Honesty
Most external sources were **egress-blocked** in-session. Numbers/standard editions must be
verified and dated-snapshotted before any of this is relied upon or stated as a claim.
