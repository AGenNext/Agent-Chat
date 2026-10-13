# Autonomyx — Legal Notes (drafting capture)

**DRAFT · unsigned · NOT legal advice.** Drafting *form* only; binding validity needs a named
entity, a governing-law jurisdiction, counsel review, and execution. Provenance: the founder
(@fractionalpm).

## The instrument
- **Software sold as software → a Software Licence (EULA), not a SaaS Terms of Service.** The
  licensee obtains the software and runs it themselves (on their own Kubernetes); the provider
  grants a right to use, and does not host or operate it.
- This matches the model: OCI artifact, run on your own VPS, no vendor/license lock,
  open-source.

## Fundamentals
- **Name a thing as itself; sell it as that thing only.** A platform as a platform; software as
  software. Mislabelling (e.g. "SaaS", "platform-as-product") breaks the definition and the
  enforceability.
- **Disclose the limits.** Sold as itself with limits stated → no complaint on hitting them.
- **Enactable by law = stability at scale.** A system not enforceable under the law of the land
  causes instability at scale. Legal enactability is the stability mechanism, not bureaucracy.

## Sample definitions (legal form)
> **"Software"** means the AGenNext Chat software made available by the Provider, comprising the
> runtime core that hosts and executes the Agents, the security gate through which all
> communications are authenticated, authorised, scoped and screened, and the associated
> components, together with any updates and successor versions made generally available.

(Other capitalised terms — "Agent", "Operator", "Provider" — are defined in their own clauses.)

## What makes it enactable (required structure)
Identified parties with capacity · governing law + jurisdiction · formation (offer, acceptance,
consideration) · certainty of terms (vague = void) · lawful purpose · operative obligations and
remedies (limitation of liability, warranties/disclaimers, indemnities) · boilerplate
(severability, entire agreement, dispute resolution, assignment, notices).

## Permissible vs impermissible claims
- **Impermissible (absolute):** "just works", "always", "guaranteed", "100% secure", "flawless",
  "the solution". Unsubstantiable, misleading, unenforceable.
- **Permissible (bounded):** "designed to…", "provided **as is**", "no warranty that it will be
  uninterrupted or error-free", "subject to the stated limitations".

## Operational-claim disclosure (mandatory seven parts)
> does [what] · under [conditions] · within [constraints] · for [duration] ·
> consuming [resources/energy] · reporting to [whom] · escalating to [whom, or not at all].

## The Promise (warranty scope)
- **Promised:** the best **verifiable** information from **user-configured trusted sources**, and
  the **most efficient tools** for a given ask/constraint/environment — a **promise of intent**.
- **Not promised:** solutions · a single imposed "truth" · a guaranteed optimum · certainty.

## Accountability
- **Agent As An Assistant** → clean separation of **Actor** (the human, accountable) and **Tool**
  (the agent, wielded). Accountability pins on the **verified human Operator** (via their DID).
- Only a human can bear accountability at law — which is what makes the whole thing enforceable.

## Still needed to make it real
Legal entity name · governing-law jurisdiction · licence choice (open-source Apache/MIT vs
commercial) · acceptance mechanism · counsel review.
