# Autonomyx — Lexicon

**DRAFT · unsigned.** Provenance: the founder (@fractionalpm). Plain definitions, real
referents only (no analogy).

## Actors
- **Operator** — the individual who operates the system. Their ability bounds it, and through
  them it is accountable.
- **Organization** — the entity that defines its own systems, processes, deliverables,
  timelines, cost, and budget. The system works within these.
- **Actor** — the human who acts, decides, signs, and is accountable.
- **Tool** — the agent/system; an instrument the Actor uses. Never an Actor.
- **Trusted real-world agency** — a real, accountable authority the Operator configures as the
  source of reference (truth is anchored outside the system).

## Identity (DCI — Decentralized Identity)
- **DCI** — the minimum identity requirement every system fulfils. Decentralizes the storage
  and use of identity data. (Source: Gartner — see REFERENCES.)
- **DID (Decentralized Identifier)** — establishes and maintains the relationship between a
  wallet holder and a service provider. Mandatory.
- **Identity wallet** — a wallet, held by the Operator, that stores the DID and credentials.
  Mandatory.
- **Verifiable Credential (VC)** — an identity attribute used to prove a claim; attested by a
  trusted agency. Mandatory.
- **Profile** — a bounded role/context frame for an Operator. Must be verifiable. One Operator
  may hold multiple profiles; one canonical identity underlies them.

## Governance
- **Scope** — what may be touched/done; least-privilege, derived, domain-bound.
- **Boundary / Constraint** — the declared limit of operation. An infinite boundary is an
  invalid contract.
- **Conformance** — adherence to the defined contracts/references at every input/processing
  node (internal correctness).
- **Compliance** — adherence to the governing law/jurisdiction/dictum at every output port
  (external lawfulness).
- **Dictum** — the governing rules the Operator declares for their instance.
- **Pin** — to admit a thing for use with an immutable id + scope + provenance + maturity +
  signature.
- **Provenance** — the traceable source/authority chain. No provenance → myth.
- **Validity** — binding force; conferred only by a human signature.

## Epistemic
- **Data** — the real, observed; admissible evidence but fallible.
- **Information** — data + context.
- **Knowledge** — information that resolves.
- **Coherence ("the math makes sense")** — the formal arbiter that catches real-but-wrong data.
- **Myth** — a claim with no source/version/boundary/signature/evidence, or one claiming
  totality. Inadmissible.
- **Evidence** — proof shown, not claimed; bounded by the hardware that produces it.
- **Horizon** — the bound of the founder's/operator's knowledge and accessible tools.

## Product & stack (see ARCHITECTURE)
- **Autonomyx / Open Autonomyx** — the system / project.
- **Arithmetic** — the platform, and the product (sold as itself).
- **Fabric** — the runtime.
- **Database** — modeled on Apache JDO, using POJOs.
- **SurrealDB** — the backend.

## Behaviour
- **Agent As An Assistant** — the agent augments and supports the Operator within their work
  fabric; never replaces or takes over.
- **Recovery path / escalation** — what an unresolved or failed step must offer (escalate to a
  human; roll back). No faked heartbeat without it.
- **Promise of intent** — a good-faith commitment to aim and seek, shown with evidence — not an
  assurance of outcome.
