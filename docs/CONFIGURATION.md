# Configuration

Every knob `agennextd` reads: command-line flags and environment variables.
The daemon is configured entirely at startup; there is no config file and no
runtime reconfiguration. Defaults are deny-by-default and in-memory, so a bare
`agennextd` starts, serves, and persists nothing until you grant it authority
and state.

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| `-addr` | `:8080` | HTTP listen address for the edge transport. |
| `-demo` | `false` | Run one turn end-to-end, print the inspectable trace as JSON, and exit — instead of serving. |

## Environment variables

All are optional. An unset variable selects the safe default named below; it
never widens authority.

| Variable | Default | Effect |
|----------|---------|--------|
| `AGENNEXT_AUTH_SECRET` | unset → insecure dev fallback | Shared secret for signed-token authentication at the gate. When set, callers must present a token the platform signed. When unset, the caller's principal is trusted (dev only; logged loudly). |
| `AGENNEXT_STATE_DIR` | unset → in-memory | Directory for durable, file-backed context and memory. Set it to survive restarts. |
| `AGENNEXT_GRANTS` | unset → one demo grant | JSON array of relationship grants (who may invoke what, with which scope). Deny-by-default. |
| `AGENNEXT_POLICY` | unset → one `rag.retrieve` rule | JSON array of allow-rules for the in-process policy decision point. Deny-by-default. |
| `AGENNEXT_MODEL_URL` | unset → grounded reasoner | OpenAI-compatible chat-completions URL. When set, the REASON step asks a language model; otherwise the deterministic grounded reasoner is used. |
| `AGENNEXT_MODEL_NAME` | unset | Model identifier passed to the endpoint. Required when `AGENNEXT_MODEL_URL` is set. |
| `AGENNEXT_MODEL_KEY` | unset | Bearer token for the model endpoint. Optional (many gateways and local servers need none). |
| `AGENNEXT_MCP` | unset | JSON map of capability → MCP tool. When set, the ACT step calls MCP tools over JSON-RPC 2.0. Takes precedence over `AGENNEXT_TOOLS`. |
| `AGENNEXT_TOOLS` | unset → demo invoker | JSON map of capability → HTTP endpoint URL. When set (and `AGENNEXT_MCP` is not), the ACT step calls plain HTTP tools. |
| `SLACK_SIGNING_SECRET` | unset → channel off | Slack request-signing secret. When set, mounts the Slack channel at `POST /slack/events`. |
| `SLACK_BOT_TOKEN` | unset | Slack bot token used to post replies. Only meaningful with `SLACK_SIGNING_SECRET`. |
| `MATTERMOST_TOKEN` | unset → channel off | Mattermost outgoing-webhook token. When set, mounts the Mattermost channel at `POST /mattermost/hooks`. |

Store secrets (`SLACK_*`, `MATTERMOST_TOKEN`, any tool auth headers) as
Kubernetes Secrets or your platform's secret manager — never in the image or
in version control.

## Authentication — `AGENNEXT_AUTH_SECRET`

The gate's first step — establishing who the caller is. Unset, the daemon runs
an insecure dev fallback that trusts the `principal` the caller puts on the
event; it is logged loudly on startup and must not be used in production.

Set `AGENNEXT_AUTH_SECRET` to a shared secret to require signed-token
authentication. The caller presents a credential the platform signed (carried
in the event's `credential` extension); the gate verifies the HMAC-SHA256
signature and expiry and derives the principal **from the token**, never from
the caller's claim. A missing, forged, expired, or wrong-tenant token is denied.

```sh
AGENNEXT_AUTH_SECRET="$(openssl rand -hex 32)" agennextd
```

Tokens are minted by a trusted issuer sharing the secret (the daemon only ever
verifies). The token binds principal, tenant, and an expiry; it is compact and
URL-safe (`base64url(payload).base64url(hmac)`). Rotate the secret by issuing
under the new one before retiring the old. This models the production binding
(mTLS/SPIFFE identity terminated at the overlay) in a single-node, dependency-
free form.

> When token auth is enabled, the Slack and Mattermost channel bridges must mint
> a credential for the events they inject, or their requests are denied at the
> gate. Wiring the channels to mint tokens is follow-up work.

## State — `AGENNEXT_STATE_DIR`

Unset, context and memory live in memory and are lost on exit. Set it to a
writable directory and both are persisted as JSON, one file per session, under:

- `<dir>/context/<hash>.json` — working context (conversation history);
- `<dir>/memory/<hash>.json` — durable memory items.

Session ids are hashed to fixed filenames, so a caller-supplied id cannot
traverse outside the directory. Writes are atomic (temp file + fsync + rename),
so a crash mid-write does not corrupt existing state. Directories are created
with `0700`.

```sh
AGENNEXT_STATE_DIR=/var/lib/agennext agennextd
```

This is the single-node durability binding. For multi-node deployments the
production bindings (Valkey for context, PostgreSQL for memory) sit behind the
same interfaces; mount a `ReadWriteOnce` volume only when running one replica.

## Authority — `AGENNEXT_GRANTS`

Who may invoke which capability, for which tenant, and with what scope. The
authorizer is deny-by-default and add-only: with no grants, nothing is
authorized. A grant is a tuple `(principal, tenant, capability) → scope`.

```json
[
  {
    "principal": "user-1",
    "tenant": "acme",
    "capability": "rag.retrieve",
    "tenants": ["acme"],
    "data": ["tenant://acme/kb/*"],
    "tools": []
  }
]
```

| Field | Meaning |
|-------|---------|
| `principal` | The authenticated caller. `"*"` grants to any principal. |
| `tenant` | The tenant the grant applies to. `"*"` = any. |
| `capability` | The capability the grant permits. `"*"` = any. |
| `tenants` | Scope: tenants the capability may act for. `"*"` = any. |
| `data` | Scope: data globs it may read (e.g. `tenant://acme/kb/*`). |
| `tools` | Scope: capability names it may invoke. |

Wildcards are honored only on the grant side; a request can never ask as `"*"`.
The granted scope is intersected with the capability contract at the gate, so a
grant can only ever narrow to what the contract already allows — it cannot widen
a caller's authority.

```sh
AGENNEXT_GRANTS='[{"principal":"user-1","tenant":"acme","capability":"rag.retrieve","tenants":["acme"],"data":["tenant://acme/kb/*"]}]' agennextd
```

## Policy — `AGENNEXT_POLICY`

The in-process policy decision point (the GUARD step), shared by the gate and
the loop. Deny-by-default and add-only: a request is permitted only when some
rule matches. There is no deny-rule — authority is added, never subtracted.

```json
[
  { "tenant": "acme", "principal": "", "capability": "rag.retrieve", "action": "invoke", "reason": "acme kb read" }
]
```

| Field | Meaning |
|-------|---------|
| `tenant` | Must equal the request tenant. `""` = any. |
| `principal` | Must equal the request principal. `""` = any. |
| `capability` | Must equal the requested capability. `""` = any. |
| `action` | Must equal the action (e.g. `invoke`). `""` = any. |
| `reason` | Recorded on a match, for the inspectable trace. |

An empty field is a wildcard on the rule side only; an empty request field never
matches a concrete rule field. Every decision carries a reason, so the GUARD
step stays inspectable.

Authority (`AGENNEXT_GRANTS`) and policy (`AGENNEXT_POLICY`) are independent and
both must pass: a grant without a matching policy rule is still denied, and a
policy rule without a matching grant is still denied. Defence in depth.

## Reasoner — `AGENNEXT_MODEL_URL`

The REASON step — how the loop decides the next action. Unset, a deterministic,
model-free grounded reasoner is used: it answers only from retrieved evidence
and escalates to a human rather than fabricate.

Set `AGENNEXT_MODEL_URL` to an OpenAI-compatible chat-completions endpoint to
bind a language model instead. The same wire format is spoken by a LiteLLM
gateway, vLLM, Ollama, and OpenAI, so one setting serves them all.
`AGENNEXT_MODEL_NAME` is required; `AGENNEXT_MODEL_KEY` is an optional Bearer
token.

```sh
AGENNEXT_MODEL_URL=https://gateway.internal/v1/chat/completions \
AGENNEXT_MODEL_NAME=my-model \
AGENNEXT_MODEL_KEY=sk-... \
agennextd
```

The model is instructed to emit one JSON action (`answer`, `invoke`, or
`escalate`). It is told to answer only from the evidence it is given. The
reasoner is self-fencing: a transport failure, a timeout, a bad body, or an
unparseable reply all degrade to escalation — a model outage becomes a human
handoff, never a fabricated answer or a hang. An `invoke` is always scoped to
the turn's admitted scope; the reasoner never widens authority. Default per-call
timeout is 20s.

## Tools — `AGENNEXT_MCP` and `AGENNEXT_TOOLS`

The ACT step — how the loop calls a real capability. Unset both and a benign
in-process demo invoker is used. Tool output is always tagged untrusted and
screened inside the loop before it re-enters reasoning, whichever binding is
chosen. A capability with no configured tool is an error, so the loop escalates
to a human rather than inventing data.

**MCP (preferred, interoperable).** `AGENNEXT_MCP` maps each capability to an
MCP server URL and tool name. The invoker speaks JSON-RPC 2.0 `tools/call` over
Streamable HTTP. A JSON-RPC error, an MCP tool error (`isError`), or an HTTP
failure all surface as an error and escalate.

```sh
AGENNEXT_MCP='{"rag.retrieve":{"url":"https://kb.internal/mcp","tool":"search"}}' agennextd
```

**Plain HTTP.** `AGENNEXT_TOOLS` maps each capability to an endpoint URL; the
invoker POSTs the input and returns the response body. Used only when
`AGENNEXT_MCP` is not set.

```sh
AGENNEXT_TOOLS='{"rag.retrieve":"https://kb.internal/retrieve"}' agennextd
```

Both bindings default to a 10s timeout and a 1 MiB response cap per call.

## Turn budget

Every turn is bounded so the loop always terminates: 6 iterations, ~8k tokens,
30s wall-clock (`loop.DefaultBudget`). These are compiled defaults, not yet
environment-configurable; a turn that hits any bound exits to a human rather
than looping.

## Example — durable, MCP-backed, single tenant

```sh
AGENNEXT_STATE_DIR=/var/lib/agennext \
AGENNEXT_GRANTS='[{"principal":"user-1","tenant":"acme","capability":"rag.retrieve","tenants":["acme"],"data":["tenant://acme/kb/*"]}]' \
AGENNEXT_POLICY='[{"tenant":"acme","capability":"rag.retrieve","action":"invoke","reason":"acme kb read"}]' \
AGENNEXT_MCP='{"rag.retrieve":{"url":"https://kb.internal/mcp","tool":"search"}}' \
agennextd -addr :8080
```

See [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) for the layer model,
[`docs/PROTOCOL.md`](PROTOCOL.md) for the gate's admission sequence, and
[`docs/LOOP.md`](LOOP.md) for the agent loop.
