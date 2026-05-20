import { randomUUID } from 'node:crypto'
import type { ChatMessage, ChatRequest, ChatResponse } from '~~/shared/types/chat'

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, '')
}

function getLatestUserMessage(messages: ChatMessage[]) {
  return [...messages].reverse().find((message) => message.role === 'user')
}

export default defineEventHandler(async (event): Promise<ChatResponse> => {
  const config = useRuntimeConfig(event)
  const body = await readBody<ChatRequest>(event)
  const messages = Array.isArray(body?.messages) ? body.messages : []
  const latestUserMessage = getLatestUserMessage(messages)

  if (!latestUserMessage?.content?.trim()) {
    throw createError({
      statusCode: 400,
      statusMessage: 'A user message is required'
    })
  }

  const conversationId = body.conversationId || randomUUID()
  const runtimeUrl = trimTrailingSlash(config.agentRuntimeUrl)

  if (runtimeUrl) {
    const authorization = getHeader(event, 'authorization')
    const headers: Record<string, string> = {
      'content-type': 'application/json',
      'x-agent-graph-schema': config.public.agentGraphSchema,
      'x-agent-auth-url': config.agentAuthUrl,
      'x-agent-identity-url': config.agentIdentityUrl
    }

    if (authorization) {
      headers.authorization = authorization
    }

    if (config.agentRuntimeApiKey) {
      headers['x-agent-runtime-key'] = config.agentRuntimeApiKey
    }

    return await $fetch<ChatResponse>(`${runtimeUrl}/v1/chat`, {
      method: 'POST',
      headers,
      body: {
        conversationId,
        messages,
        context: {
          graphSchema: config.public.agentGraphSchema,
          surrealdbConfigured: Boolean(config.surrealdbUrl)
        }
      }
    })
  }

  return {
    conversationId,
    message: {
      id: randomUUID(),
      role: 'assistant',
      content: `Agent-Runtime is not configured yet. I received: "${latestUserMessage.content}". Set NUXT_AGENT_RUNTIME_URL to route this chat through the SurrealDB-backed runtime plane.`,
      createdAt: new Date().toISOString(),
      source: 'local-fallback'
    },
    runtime: {
      graphSchema: config.public.agentGraphSchema,
      authConfigured: Boolean(config.agentAuthUrl),
      identityConfigured: Boolean(config.agentIdentityUrl),
      runtimeConfigured: false,
      surrealdbConfigured: Boolean(config.surrealdbUrl)
    }
  }
})
