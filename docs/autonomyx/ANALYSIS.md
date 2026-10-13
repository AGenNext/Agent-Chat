# Autonomyx / AGenNext — Analysis (benchmark, scorecard, contract-parser)

**DRAFT.** Captured from in-session research (web-sourced where noted). External facts should be
re-verified at use; the agent did not fabricate the cited findings, but the live sources are not
re-fetchable here.

## Benchmark vs 2026 gold standards (architectural)

A numeric latency benchmark across these systems would be apples-to-oranges (a Go stdlib loop
vs a durable service vs a Python framework wrapping an LLM call). So this is an architectural
comparison per dimension.

| Dimension | Gold standard (2026) | AGenNext position | Verdict |
|---|---|---|---|
| Deterministic execution | **Temporal** — every LLM call lives in an "activity" outside the workflow | deterministic core; model is the fenced exception; idempotent replay | at parity, by design |
| Durable / crash-resume | Temporal / event-sourcing | in-memory (stub); replay is loop-level | behind |
| Tool standard | **MCP** (Linux Foundation, Dec 2025) | bespoke Capability contract; MCP has CVEs/auth gaps | behind on conformance, ahead on governance |
| Governance-as-eligibility | **OPA Gatekeeper** v3.22 | kernel admission + edge gate; OPA/OpenFGA planned (stubbed) | aligned in design; not wired |
| Supply chain | **SLSA L2–3 / Sigstore** | cosign keyless + provenance + SBOM + signed commits + SHA-pinned actions | at/near parity |
| Efficiency / attack surface | Python stacks (large dep trees) | 0 third-party deps; 6.5 MB static binary | ahead |
| Resiliency | Temporal durable retry | always-exit/escalation, no-bleed, bounded; no durable resume / chaos test | parity on recovery, behind on resume |

**Sources (in-session):** cordum.io/blog/temporal-vs-langgraph; langchain.com/resources/langgraph-vs-temporal;
chatforest.com (MCP 2026); appsecsanta.com/opa-gatekeeper; aquilax.ai (SLSA/Sigstore).

## Evaluation scorecard (measured)

| Dimension | Evidence | Grade |
|---|---|---|
| Performance | core turn ~27 µs/op, ~37k turns/s/core (BenchmarkCoreTurn) | core negligible; end-to-end model-bound, unmeasured |
| Conformance | 4 anti-corrosion guards proven to bite; suite green | strong |
| Efficiency | 3,425 B/op, 29 allocs/op; 6.5 MB binary; 0 deps | lean |
| Effectiveness | end-to-end turn resolves (demo + tests) | reference-path only (stubs) |
| Operational excellence | HA chart, probes, graceful shutdown, signed commits, SHA-pinned actions, release w/ SBOM+provenance+cosign | strong; no metrics endpoint yet |
| Freedom of brilliance | composable Adapter/Capability; deterministic core | open at edge, zero freedom to distort core |
| Governance-as-eligibility | kernel admits only valid contracts; derived scope | core strength |
| Resiliency | recovery-path enforced, race-clean; no chaos/soak | strong at loop/pod, unproven under chaos |

## Contract-parser / loophole-finder (design, not built)

"A contract parser finds the loopholes" — a recognized discipline:
- **Authorization analysis:** **Cedar** (AWS; CNCF Sandbox Jan 2026) is *analyzable* — reduces
  a policy to SMT and can **prove** "no over-permission / no infinite boundary." OPA/Gatekeeper
  (test-based), OpenFGA (relationship-based).
- **Static analysis:** **Slither** (92+ detectors, CI-native) is the detector-set template;
  Mythril/Halmos add symbolic execution.
- **Honesty finding:** best static-tool combos catch ~76.78% of issues (some measures 8–20%).
  Reliable path is layered: static → fuzz → symbolic → formal → **human review (terminal)**.
  A parser is necessary, not sufficient — which matches the doctrine: parser finds loopholes,
  human signs validity.

**Sources (in-session):** permit.io (OPA/Cedar/OpenFGA); goteleport.com (policy benchmarks);
arxiv.org/pdf/2403.04651 (Cedar); infoq.com/news/2026/01/cedar-joins-cncf-sandbox.
