# k3s CIS Hardening — Autonomyx / agennext-chat

Ready-to-run artifacts that take a default k3s install to the **CIS Kubernetes Benchmark**,
per the official guide: https://docs.k3s.io/security/hardening-guide. Default namespace is
`agennext`; PSA enforce level is `restricted`.

Run **on each k3s server node**:

```bash
cd deploy/hardening
sudo ./harden.sh --dry-run                 # preview every change, no writes
sudo ./harden.sh --app-ns=agennext         # apply, then restart k3s
sudo ./verify.sh                           # confirm it took
```

Covers: CIS sysctls, API-server audit policy, Pod Security Admission (restricted) +
EventRateLimit, default-deny NetworkPolicies with DNS/metrics/Traefik carve-outs, secrets
encryption, PKI permission tightening, and SA-token automount off.

## Two things that will bite you (read first)
- After `restricted` PSA, **new non-compliant pods are rejected** (existing keep running).
  The agennext-chat chart already sets a compliant `securityContext` (nonroot, read-only
  rootfs, drop ALL caps, RuntimeDefault seccomp), so it passes. Test with `--enforce=baseline`
  if unsure about other workloads.
- **Every new namespace needs its own intra-namespace NetworkPolicy + DNS allow**, or its
  pods can't resolve names. The script seeds the ones you pass via `--app-ns`.

## Verify properly
File/flag presence ≠ a CIS score. Run **kube-bench** against the version-matched CIS
self-assessment for a real benchmark result. `verify.sh` checks that the changes applied,
not the full benchmark.

Source: generated from the k3s-harden skill, which mirrors the official k3s hardening guide.
