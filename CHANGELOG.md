# Changelog

All notable changes to this project are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Entries accrue under **[Unreleased]**. They become a numbered release only when
a maintainer signs the release tag (see [`RELEASE.md`](RELEASE.md)); nothing
here is a published release until that tag exists.

## [Unreleased]

Prepared as **0.1.0** — the initial release — pending a signed `v0.1.0` tag.

### Added

- **Core agent loop.** Bounded, event-driven reason→guard→act→screen→observe→
  persist→exit cycle. Every turn is budget-bounded (iterations, tokens,
  wall-clock) and always has an exit: an unresolved turn escalates to a human
  rather than emitting a dead or partial result.
- **Edge gate.** The one boundary that terminates external traffic, admitting
  events only through a fixed sequence: authenticate → authorize → derive scope
  (grant ∩ contract, never caller-supplied) → L1 injection screen.
- **Capability-as-contract.** Capabilities declare what they provide, require,
  and may touch; a capability kernel admits contracts and the registry is the
  source of truth for scope validation.
- **Relationship-based authorizer** (`pkg/authz`). Deny-by-default, add-only,
  no deny-rules. Satisfies the gate's authorization and scope-derivation seams.
  Configured with `AGENNEXT_GRANTS`.
- **Rule-based policy decider** (`pkg/guard`). Deny-by-default in-process policy
  decision point matching tenant, principal, capability, and action; every
  decision carries a reason. Configured with `AGENNEXT_POLICY`.
- **Reasoners** (`pkg/reason`). A deterministic, model-free grounded reasoner
  that answers only from evidence and escalates rather than fabricate (adds an
  explicit `escalate` action to the loop); and a model reasoner that binds a
  language model over an OpenAI-compatible chat-completions endpoint
  (`AGENNEXT_MODEL_URL`), self-fencing so a model fault degrades to escalation.
- **Durable state** (`pkg/store`). File-backed context and memory stores with
  atomic writes and hashed, traversal-safe filenames. Enabled with
  `AGENNEXT_STATE_DIR`; in-memory otherwise.
- **Tool transports** (`pkg/tools`). ACT-step bindings that call real
  capabilities and tag their output untrusted for in-loop screening: an MCP
  transport (JSON-RPC 2.0 `tools/call`, `AGENNEXT_MCP`) and a plain HTTP
  transport (`AGENNEXT_TOOLS`).
- **Channels.** Slack (`POST /slack/events`) and Mattermost
  (`POST /mattermost/hooks`) adapters that cross into the core only through the
  gate. Mounted when their signing secret/token is configured.
- **HTTP transport & observability.** Edge HTTP server (`/healthz`, `/v1/chat`)
  with a Prometheus `/metrics` endpoint.
- **Deployment.** Helm chart with NetworkPolicy (default-deny), HPA,
  ServiceMonitor, and a production values file; k3s CIS hardening scripts;
  in-cluster Cloud Native Buildpacks build (Podman, no Docker).
- **Release pipeline.** Tag-triggered workflow building and signing the
  container image (keyless cosign, SBOM + provenance) and cross-platform
  binaries (linux/macOS/windows). macOS install via Homebrew formula and a curl
  installer.
- **Supply chain.** GitHub Actions pinned to immutable commit SHAs; a
  conformance suite enforcing zero third-party dependencies in core and banning
  a faked heartbeat.
- **Documentation.** Architecture, loop, protocol, capabilities, threat model,
  principles, stack, supply chain, spec/scope/roadmap, and a configuration
  reference ([`docs/CONFIGURATION.md`](docs/CONFIGURATION.md)).

### Security

- Zero third-party dependencies in the core (pure Go standard library),
  enforced by a conformance test.
- Indirect prompt-injection (T2) defence: untrusted tool/retrieval output is
  screened inside the loop before it re-enters reasoning; the outbound answer is
  screened before it leaves (egress shield), redacting to a safe handoff.
- Scope is always derived, never caller-supplied; a request cannot widen its own
  authority.

[Unreleased]: https://github.com/AGenNext/Agent-Chat/commits/main
