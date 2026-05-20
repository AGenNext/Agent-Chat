# Agent Chat

Nuxt frontend for AGenNext agent conversations.

## Role

Agent Chat is the user-facing chat application. It talks to the authentication and identity layers, then sends chat turns to [Agent-Runtime](https://github.com/AGenNext/Agent-Runtime). Runtime owns live behavior and state in the SurrealDB plane. Agent Graph remains the schema and contract source loaded into that plane.

```text
Agent-Chat
  -> Agent-Auth
  -> Agent-Identity
  -> Agent-Runtime
  -> SurrealDB with Agent-Graph schema
```

## Setup

```bash
npm install
npm run dev
```

Copy `.env.example` to `.env` and set the service URLs when those services are available.

## Runtime configuration

| Variable | Purpose |
| --- | --- |
| `NUXT_AGENT_RUNTIME_URL` | Agent-Runtime `/v1/chat` backend URL |
| `NUXT_AGENT_RUNTIME_API_KEY` | Optional server-side runtime API key |
| `NUXT_AGENT_AUTH_URL` | Server-side Agent Auth URL |
| `NUXT_PUBLIC_AGENT_AUTH_URL` | Browser-visible Agent Auth URL |
| `NUXT_AGENT_IDENTITY_URL` | Server-side Agent Identity URL |
| `NUXT_PUBLIC_AGENT_IDENTITY_URL` | Browser-visible Agent Identity URL |
| `NUXT_SURREALDB_URL` | Server-side SurrealDB endpoint marker |
| `NUXT_PUBLIC_AGENT_GRAPH_SCHEMA` | Active graph schema label |
| `NUXT_PUBLIC_AGENT_RUNTIME_REPOSITORY` | Agent-Runtime repository reference |
