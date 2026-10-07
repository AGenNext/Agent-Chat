# Autonomyx — Architecture / Stack

**DRAFT · unsigned.** Provenance: the founder (@fractionalpm). External capabilities are pinned
from real sources (see REFERENCES); the agent did not invent them.

## The stack (top to bottom)

| Layer | Name | Notes |
|---|---|---|
| System / project | **Open Autonomyx** | `platform.openautonomyx.com` (origin; not fetched — egress-blocked) |
| Platform = product | **Arithmetic** | the platform is the product, sold as itself |
| Runtime | **Fabric** | the live operating substrate (state, real-time, events, access) |
| Database | **modeled on Apache JDO** | POJOs; datastore-agnostic; separation of concerns |
| Backend | **SurrealDB** | `surrealdb.com`; open-source multi-model DB |

Modes and functions of Arithmetic: **to be specified by the founder** (deferred placeholder).

## Database — modeled on Apache JDO

- **Pin:** JDO **3.2.1** (spec **JSR-243**). © Apache Software Foundation, 2005–2022.
- Uses **plain old Java objects (POJOs)**. Separates data manipulation (Java members in domain
  objects) from database manipulation (JDO interface methods) → independence of the domain view
  from the datastore view. The backend can change without touching the domain model.
- **Core interfaces:** PersistenceManager (lifecycle, Query factory, Transaction access),
  Query (query the datastore), Transaction (initiate/complete transactions).
- **Class types:** PersistenceCapable (persistable, enhanced, core), PersistenceAware
  (manipulates persistable instances, minimal metadata), Normal (not persistable, unchanged).

## Identity — Decentralized Identity (DCI)

The minimum every system fulfils (per Gartner, "Features of Decentralized Identity",
Nov 2025 — see REFERENCES):

- **Mandatory:** DIDs · an identity wallet · verifiable credentials.
- **Optional:** identity trust fabric (typically a blockchain) · a verifier interface
  (validate claims) · an issuer interface (issue VCs to wallet holders).

One Operator = one canonical identity → multiple bounded profiles hang off it. The profile must
be verifiable, against trusted real-world agencies.

## Access

- **Uniform access layer** — one consistent access model across everything.
- **Universal just-in-time (JIT) access** — granted on-demand, time-bound, then revoked; no
  standing privilege.
- **Fine-grained access control** — granular least-privilege.
- **Resource sovereignty** — every agency/operator reserves complete rights to their resources;
  access never transfers ownership.

## Cross-cutting

- **Clear separation of concerns** across all layers (identity · access · data fabric ·
  evidence · governance/gate · the loop). Each does one thing; each is independently verifiable.
- **Conformance** at input/processing nodes; **compliance** at output ports.
- **Evidence** produced at defined points, bounded by the hardware's capability.
