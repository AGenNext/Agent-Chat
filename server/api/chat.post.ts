import { randomUUID } from 'node:crypto'
import type { ChatMessage, ChatRequest, ChatResponse } from '~~/shared/types/chat'

function getLatestUserMessage(messages: ChatMessage[]) {
  return [...messages].reverse().find((message) => message.role === 'user')
}

export default defineEventHandler(async (event): Promise<ChatResponse> => {
  const config = useRuntimeConfig(event)
  const body = await readBody<ChatRequest>(event)
  const runtimeStatus = getRuntimeStatus(config)
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
      'x-agent-graph-schema': runtimeStatus.graphSchema,
      'x-agent-runtime-repository': runtimeStatus.runtimeRepository
    }

    if (authorization) {
      headers.authorization = authorization
    }

    if (config.agentAuthUrl) {
      headers['x-agent-auth-url'] = config.agentAuthUrl
    }

    if (config.agentIdentityUrl) {
      headers['x-agent-identity-url'] = config.agentIdentityUrl
    }

    if (config.agentRuntimeApiKey) {
      headers['x-agent-runtime-key'] = config.agentRuntimeApiKey
    }

    try {
      const response = await $fetch<ChatResponse>(`${runtimeUrl}/v1/chat`, {
        method: 'POST',
        timeout: 30000,
        headers,
        body: {
          conversationId,
          messages,
          context: {
            graphSchema: runtimeStatus.graphSchema,
            runtimeRepository: runtimeStatus.runtimeRepository,
            surrealdbConfigured: runtimeStatus.surrealdbConfigured
          }
        }
      })

      return {
        ...response,
        runtime: {
          ...runtimeStatus,
          runtimeConfigured: true
        }
      }
    } catch (err) {
      throw createError({
        statusCode: 502,
        statusMessage: err instanceof Error ? err.message : 'Agent-Runtime request failed'
      })
    }
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
    runtime: runtimeStatus
  }
})
