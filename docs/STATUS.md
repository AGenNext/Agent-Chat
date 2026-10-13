# Status

A snapshot of what is built and verified versus what is open. This is a working
status, not a release claim — nothing here is published until a signed tag
exists (see [`RELEASE.md`](../RELEASE.md)).

_As of commit `d72d545`, branch `claude/practical-maxwell-47EDa` (PR #18)._

## Verify this snapshot

```sh
go build ./...
go vet ./...
go test ./...                              # 92 test functions, all packages
go test -race ./pkg/loop/                  # concurrency-sensitive paths
test "$(grep -c require go.mod)" -eq 0     # zero third-party deps in core
go run ./cmd/agennextd -demo               # one turn end-to-end, prints the trace
```

## Done — in the tree, tested, pushed

| Area | What | Where |
|------|------|-------|
| Agent loop | bounded reason→guard→act→screen→observe→persist→exit; budget-bounded; always-exit/escalate | `pkg/loop` |
| ACT fence | panic recovery + hung-tool deadline abandonment; chaos/fault-injection tests | `pkg/loop` (`safeInvoke`, `chaos_test.go`) |
| Edge gate | authn → authz → derived scope (grant ∩ contract) → injection screen | `pkg/edge` |
| Capability-as-contract | kernel admission + registry; scope validation | `pkg/capability`, `pkg/kernel` |
| Authorizer | ReBAC, deny-by-default, add-only, no deny-rules | `pkg/authz` |
| Policy decider | rule-based, deny-by-default, add-only; reasons on every decision | `pkg/guard` |
| Reasoner | grounded, model-free, evidence-only, escalates rather than fabricate | `pkg/reason` |
| Durable state | file-backed context + memory; atomic writes; traversal-safe | `pkg/store` |
| Tool transports | MCP (JSON-RPC 2.0) and plain HTTP; output screened as untrusted | `pkg/tools` |
| Channels | Slack + Mattermost, crossing in only through the gate | `channels/` |
| HTTP + metrics | edge server; Prometheus `/metrics` | `pkg/server` |
| Deploy | Helm chart (NetworkPolicy/HPA/ServiceMonitor), k3s CIS hardening, buildpacks build | `deploy/` |
| Release pipeline | tag-triggered image (cosign + SBOM + provenance) and cross-platform binaries | `.github/workflows/release.yaml` |
| Install | Homebrew formula + curl installer | `install/` |
| Supply chain | Actions pinned to SHAs; conformance bans third-party deps and the faked heartbeat | `internal/conformance` |
| Docs | architecture, loop, protocol, threat model, principles, configuration, changelog, runbook | `docs/`, `CHANGELOG.md`, `RELEASE.md` |

All code is pure Go standard library; the zero-dependency invariant is enforced
by a conformance test, not by convention.

## Open — maintainer-gated (cannot be done from a work session)

1. **Merge PR #18** into the default branch.
2. **Register the SSH signing key on GitHub** so tags (and commits) verify.
   Current commits show "Unverified" only because the key is not registered —
   they are signed.
3. **Sign and push `v0.1.0`** — the publish decision. This triggers the image +
   binary pipeline. Steps in [`RELEASE.md`](../RELEASE.md) §4.
4. **Founder decisions:** licence (the Homebrew formula assumes `Apache-2.0`
   with a TODO), legal entity, jurisdiction. Resolve before a public 1.0.

## Open — larger / outside the zero-dep core

These are deliberate non-goals for the pure-stdlib core; each needs a dependency
or a research effort and belongs behind an existing seam, not in core.

- **LLM reasoner binding.** The production reasoner (a model via an inference
  gateway) plugs into the `loop.Reasoner` seam that `pkg/reason` already
  defines. It needs a network/model client, so it lives outside the zero-dep
  core, as an adapter.
- **Real authz/policy backends.** OpenFGA (authorization) and OPA (policy) are
  the production bindings; the in-process `pkg/authz` and `pkg/guard` deciders
  model them behind the same seams.
- **Production state backends.** Valkey (context) and PostgreSQL (memory) behind
  the `store` interfaces, for multi-node deployments; the file-backed stores
  cover single-node.
- **Concept / self-learning** and **notarized macOS signing** — tracked in
  `docs/autonomyx/BACKLOG.md`.

## Known fixed in this stream

- Install + kpack references pointed at a non-existent `AGenNextHub/Agent-Chat`;
  corrected to `AGenNext/Agent-Chat` (release downloads and the in-cluster build
  clone would otherwise have 404'd).
