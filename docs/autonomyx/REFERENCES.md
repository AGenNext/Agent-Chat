# Autonomyx — References (pinned sources)

**DRAFT · unsigned.** Honesty rule: each entry is an **origin**. A true version-pin needs a
specific version/edition or a dated snapshot. Items marked **(egress-blocked)** could not be
fetched by the agent; their content, where present, was **provided by the founder in-session**
(real source text), not recalled from the agent's memory.

## Identity (DCI)
- **Gartner — Decentralized Identity market.** `https://www.gartner.com/reviews/market/decentralized-identity`
  — the **RAG source** for DCI. **(egress-blocked.)**
- **Gartner — "Features of Decentralized Identity", updated November 2025** (provided in-session):
  - **Mandatory:** DIDs · identity wallet · verifiable credentials.
  - **Optional:** identity trust fabric (typically blockchain) · verifier interface · issuer interface.
- **Gartner Insights** — the advisory the project follows (proprietary/paywalled). Specific
  guidance must be pinned from real Gartner Insights material at time of use. **(egress-blocked.)**

## Database
- **Apache JDO.** `https://db.apache.org/jdo/` — **JDO 3.2.1 / JSR-243**, © 2005–2022 ASF
  (page content provided in-session).
  - Interfaces: PersistenceManager, Query, Transaction.
  - Class types: PersistenceCapable, PersistenceAware, Normal.
  - Uses POJOs; separation of concerns; datastore-agnostic.

## Backend
- **SurrealDB.** `https://surrealdb.com/` — backend; open-source multi-model database.
  Pin the specific version from SurrealDB's own docs. **(egress-blocked.)**

## Platform
- **Open Autonomyx.** `http://platform.openautonomyx.com/` — platform/product (Arithmetic)
  origin. **(egress-blocked; http.)**

## Supporting (chat platform context)
- **Kubernetes Overview** — uploaded snapshot of `https://kubernetes.io/docs/concepts/overview/_print/`,
  content digest `sha256:c5283d747daae9a67e90f74ef7ddbada39009857249c95019dc93e5175773941`
  (frozen artifact, dated 2026-06-13 — a valid version-pin by snapshot).

## Candidate (named, not yet pinned)
- Wikipedia "Information technology" — needs a specific `?oldid=` revision before it can be a
  pin. **(egress-blocked.)**
- W3C DID / Verifiable Credentials standards — referenced via the Gartner DCI definition; pin
  from the real W3C specs when used directly.

---
**What is verified vs not.** The JDO and Gartner-DCI feature content above is **real source
text the founder pasted in-session.** The live URLs were **not** reachable by the agent
(egress-blocked), so treat the URLs as origins and capture **dated snapshots / specific
versions** before any of this is relied upon or stated as a claim.
