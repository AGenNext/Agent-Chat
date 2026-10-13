# AGenNext Chat

A **composable, Kubernetes-native, multi-tenant SaaS platform** for secure RAG-based agent
chat — a **stable runtime core** of agents, with the **edge (the gate)** guarding every
boundary. Runs on any conformant Kubernetes (microk8s, k3s, …).

Its layered RAG security is grounded in *"Securing LLM-as-a-Service for Small Businesses"*
(Xie et al., RMIT University, AISC 2026, arXiv:2601.15528v1); generalized here into a
multi-tenant SaaS core (not edge-specific). See [`docs/REFERENCES.md`](docs/REFERENCES.md).

> **Status — v0.1.x (the loop spine):** the agent-loop core is implemented in **pure Go
> standard library (zero third-party dependencies)**, tested, with a runnable headless
> daemon. Production bindings (NATS, OPA, OpenFGA, KServe, PostgreSQL) and the k8s operator
> are next — see [`docs/ROADMAP.md`](docs/ROADMAP.md).

## What's built

Implemented today, tested, zero third-party dependencies:

| Component | What it is |
|---|---|
| `pkg/loop` | the agent loop: bounded reason→guard→act→screen→observe→persist→exit; guard-before-act, tool-output screening, transactional turn, idempotent replay, always-exit to a human |
| `pkg/edge` | the Protocol Gate: authn → authz → **derived scope** → injection screen |
| `pkg/box` | content-addressed boxes: define → build → sign → publish; Merkle-DAG resolve |
| `pkg/capability` | capability-as-contract: `Contract`, `Scope`, `Registry` |
| `pkg/kernel` | control-loop admission (reconcile desired contracts) |
| `pkg/chat` | the chat runtime core (edge-free) |
| `pkg/store` · `pkg/guard` · `pkg/event` | context+memory · injection screener/policy · CloudEvents envelope |
| `pkg/server` + `cmd/agennextd` | headless HTTP daemon (`GET /healthz`, `POST /v1/chat`) |
| `cmd/boxctl` | the box builder CLI (build / digest / verify / resolve) |
| `proto/` · `deploy/crds/` | proto-first contract IDL · kernel CRDs |
| `deploy/` | HA Helm chart, Argo CD app, distroless Dockerfile, self-hosted Zot registry |
| `packages/design-tokens` | brand + W3C DTCG design tokens (npm + OCI) |

## The model

```
                tenants · clients · channels
                                  │
                ┌─────────────────▼─────────────────┐
                │  EDGE — Protocol Gate (pkg/edge)   │  guards the boundary: authN/Z · scope · screen
                └─────────────────┬─────────────────┘
                ┌─────────────────▼─────────────────┐
                │  RUNTIME CORE — the loop (pkg/loop)│  the stable core: reason→guard→act→screen→persist
                └─────────────────┬─────────────────┘
                ┌─────────────────▼─────────────────┐
                │  KERNEL — k8s control loop         │  scheduling · tenant isolation · policy
                └────────────────────────────────────┘
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`docs/CONCEPTS.md`](docs/CONCEPTS.md).

## Quickstart

```bash
go run ./cmd/agennextd                 # serve the headless API on :8080
curl localhost:8080/healthz            # {"status":"ok"}
curl -XPOST localhost:8080/v1/chat -H 'content-type: application/json' \
  -d '{"id":"1","session":"s1","tenant":"acme","principal":"u1","capability":"rag.retrieve","message":"return policy?"}'

go run ./cmd/agennextd -demo           # run one turn and print the inspectable trace
```

The daemon admits a chat event through the Edge Gate, runs the bounded agent loop, and
returns a JSON trace where every step is attributable. The request carries **no scope** —
authority is derived at the gate. Admission failures map to safe HTTP codes (401/403/422).

## Verify

```bash
make check     # tidy + go vet + golangci-lint + go test -race   (mirrors CI)
make cover     # coverage summary
make run       # the end-to-end demo
```

## Documentation

| Doc | Purpose |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) · [`docs/CONCEPTS.md`](docs/CONCEPTS.md) | the three-layer model; core concepts |
| [`docs/LOOP.md`](docs/LOOP.md) | the agent loop in detail |
| [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) | all `agennextd` flags and environment variables |
| [`docs/PROTOCOL.md`](docs/PROTOCOL.md) · [`docs/CAPABILITIES.md`](docs/CAPABILITIES.md) | gate contract; capability-as-contract |
| [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md) · [`docs/PRINCIPLES.md`](docs/PRINCIPLES.md) | threats + defences; governance tenets |
| [`docs/STACK.md`](docs/STACK.md) · [`docs/SUPPLY_CHAIN.md`](docs/SUPPLY_CHAIN.md) | CNCF stack; supply-chain policy |
| [`docs/SPEC.md`](docs/SPEC.md) · [`docs/SCOPE.md`](docs/SCOPE.md) · [`docs/ROADMAP.md`](docs/ROADMAP.md) · [`docs/adr/`](docs/adr/) | spec, scope, roadmap, ADRs |

## Related

This repo is the **chat platform**. The cross-cutting language and platform live upstream:

- **Agent Language Protocol** → [`AGenNext/Agent-Language`](https://github.com/AGenNext/Agent-Language)
- **AGenNext Platform** (platform agent) → [`AGenNext/Agent-Platform-MVP`](https://github.com/AGenNext/Agent-Platform-MVP)

## Contributing & security

See [`CONTRIBUTING.md`](CONTRIBUTING.md), [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md), and
[`SECURITY.md`](SECURITY.md).

## License

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
