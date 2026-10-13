# Production Readiness — agennext-chat / Autonomyx

**Status of the launch-critical technical gates.** ✅ = done in this repo · ⚠️ = partial ·
⬜ = needs ops/compliance work outside the code.

## Application
- ✅ Hardened container: distroless, nonroot, read-only rootfs, drop ALL caps, RuntimeDefault seccomp.
- ✅ Health & readiness probes (`/healthz`, `/readyz`).
- ✅ **Observability**: `/metrics` (Prometheus text format, pure stdlib) — requests, in-flight, turns, escalated, errors. Tested.
- ✅ Graceful shutdown; bounded request body; no internal detail bled on errors.
- ✅ Zero third-party dependencies (conformance-enforced).
- ⚠️ **Reasoner/invoker are stubs** — real model + tool bindings not wired (fenced exception). *(Product gate.)*
- ⬜ Durable cross-crash state (in-memory stores today).

## Chart / deployment
- ✅ HA defaults: 3 replicas, PodDisruptionBudget, `maxUnavailable: 0` rollout, topology spread.
- ✅ **NetworkPolicy** (default-deny + DNS egress; add your egress rules).
- ✅ **HorizontalPodAutoscaler** (autoscaling/v2, CPU target).
- ✅ **ServiceMonitor** (behind flag, for Prometheus Operator).
- ✅ `values-prod.yaml` — autoscaling on, isolation on, metrics on, required spread, resource headroom.
- ⬜ Pin `image.tag` to the released **@sha256 digest** (reminder in values-prod).

## Cluster (k3s)
- ✅ **CIS hardening** artifacts in `deploy/hardening/` — `harden.sh` + `verify.sh` + assets
  (sysctls, audit policy, Pod Security Admission `restricted`, default-deny NetworkPolicies,
  secrets encryption, PKI perms). *Run on the server node; cannot be applied from CI (egress).*
- ⬜ Run **kube-bench** for a real CIS score; the scripts apply changes, they don't benchmark.

## Supply chain
- ✅ Release workflow: cosign keyless signing, SLSA provenance, SBOM.
- ✅ GitHub Actions pinned to commit SHAs; signed-forward commits.
- ⬜ Register the signing key on GitHub for "Verified"; enable branch protection (require signed commits + green CI).

## Build / deploy
- ✅ One-pass installer (`deploy/install-k3s.sh`) and in-cluster build (`deploy/kpack/build.yaml`).
- ✅ CI green on the branch.

## Still outside code (ops / compliance / legal — launch gates)
- ⬜ Security: pen test; secrets management (external secrets / Vault).
- ⬜ Compliance: SOC 2 / ISO 27001 path; GDPR/DPA; finalize Privacy/Terms with entity + jurisdiction + counsel.
- ⬜ Identity: DCI (DID + wallet + verifiable credentials) wired for real onboarding.
- ⬜ Support/SLA, status page, incident process; billing/metering.
- ⬜ Licence decision (open-core vs commercial) → LICENSE + EULA.

---
*Updated by the production-hardening pass. The code/chart/cluster gates above are done and
tested here; the remaining items need the live cluster, counsel, or a product decision.*
