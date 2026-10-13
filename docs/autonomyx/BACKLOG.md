# Autonomyx — Backlog (defined, not built)

**DRAFT.** Things discussed in-session that are **not implemented**. Captured so they aren't
lost. None of these exist as software yet — do not claim them as features.

## Identity & access
- DCI implementation: DID + identity wallet + verifiable credentials (mandatory floor), per
  Gartner. Optional: trust fabric, verifier interface, issuer interface.
- Uniform access layer: universal just-in-time access, fine-grained access control, resource
  sovereignty.

## Data / runtime / platform
- **Fabric** runtime extending **SurrealDB** (backend), with clear separation of concerns.
- **Database** modeled on **Apache JDO** (3.2.1 / JSR-243), POJOs.
- **Arithmetic** platform — its **modes and functions are still to be specified** (deferred).
- **HertzBeat** as the heartbeat / monitoring / evidence layer.

## Governance & safety
- Contract-parser / loophole-finder: anchor on Cedar (formal proof) + OPA/Gatekeeper +
  Slither-style detectors; human review terminal. (Design only — see ANALYSIS.md.)
- Enforce "infinite boundary = invalid contract" and DCI-minimum as admission checks.
- Safety: only allow building what is buildable with existing tools/protocols (universal).

## Engineering gaps (AGenNext Chat repo)
- MCP-at-the-edge (speak the universal tool standard, wrapped by the gate).
- Durable cross-crash state (replace in-memory stores; event-sourcing).
- Real OPA / OpenFGA policy binding (currently stubbed).
- `/metrics` (Prometheus) endpoint + load/soak harness (observability currently absent).
- A chaos / failure-injection test (resiliency proven only at loop level).
- Real model + tool bindings (reasoner/invoker are deterministic stubs).
- Concept / self-learning primitive (learn → validate-by-resolution → admit → sign).

## Definitions to redo (real, non-metaphor)
- **Cortex** and **Temporal** — captured as metaphor in PRIMITIVES.md; need real referents
  (and resolve the "Temporal" name clash with temporal.io).

## Decisions outstanding (from the founder)
- Licence (open-source Apache/MIT vs commercial) · legal entity · governing-law jurisdiction.
- Org slug: AGenNext vs AGenNextHub.
- Versions to pin: SurrealDB, HertzBeat, Buildpacks builder, Gartner Insights citation.
- Egress allowlist (or pasted content) to verify external sources.
