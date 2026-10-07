# Knowledge — Kubecontainers

A **Kubecontainer** is knowledge made into a **content-addressed, signed container**: a
`pkg/box` Box whose payload is curated, sourced, *resolving* concepts. Its identity **is**
its content (`sha256` digest), so knowledge is versioned and tamper-evident; a human
signature confers **validity**. Verify, don't believe — applied to knowledge itself.

## The unit: a concept that resolves

```
signal      the observable symptom (status / alert / event)
context     where it applies
causes      what produces it
resolution  the action that resolves it
source      provenance URL(s) — within the declared scope
resolves    the check that the action actually clears the signal
```

A concept without a **source** and a **resolves** check is not admitted. That would be a
faked heartbeat of knowledge.

## Two axes that govern it

- **Scope is domain-bound (by provenance).** The source URL carries the domain, and the
  domain bounds the authority. A concept is admissible only if its source domain falls
  within the container's declared scope. Widening beyond the default domain is explicit and
  recorded. *(see `troubleshooting.v1.json` → `scope`)*
- **Context is work-bound (by current task).** Scope says what the knowledge *may* claim;
  context says what is *relevant now*. Which concept activates is set by the current work —
  e.g. a pod stuck in `KubeContainerWaiting` selects the `image-pull-backoff` concept.

Context narrows; scope bounds; the intersection is the answer.

## Lifecycle: Define → Build → Sign → Admit

1. **Define / curate** the concepts (`troubleshooting.v1.json`), each within domain scope.
2. **Build** the content-addressed Box — `boxctl` builds, it **does not sign**:
   ```bash
   go run ./cmd/boxctl build \
     -t application/vnd.agennext.knowledge.kube.troubleshooting.v1+json \
     -s agennext.knowledge.concept/v1 \
     -f knowledge/kube/troubleshooting.v1.json \
     -o knowledge/kube/troubleshooting.v1.box
   ```
3. **Sign** — a human signs the Box; realness is rooted in a human key holder. Validity is
   the signature, not the file.
4. **Admit** — the signed Box is published to the registry and declared as the capability
   `kube.diagnose` (`capability.json`), which the Cortex consults, Temporal records, and the
   gate scopes by domain.

## This container

| | |
|---|---|
| Domain | `kubernetes/troubleshooting` |
| Scope (domains) | `kubernetes.io` (default), `runbooks.prometheus-operator.dev` |
| Concepts | 8 — image-pull-backoff · crash-loop-backoff · pending-unschedulable · oom-killed · create-container-config-error · readiness-probe-failing · node-pressure-eviction · init-container-stuck |
| Box digest | `sha256:623f0b515dfbf242d57da12d35940ec7a05ccb1eef2bfe10125321d795593613` |
| Validity | **UNSIGNED** — awaiting a human signature |

> Build only. The agent curated and built; only a human signature makes it real.
