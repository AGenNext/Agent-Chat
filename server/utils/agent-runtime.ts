import { randomUUID } from 'node:crypto'
import type { H3Event } from 'h3'
import type { ChatMessage, ChatRequest, ChatResponse, RuntimeChatResponse } from '~~/shared/types/chat'
import type { RuntimeStatus } from '~~/shared/types/runtime'

export function getLatestUserMessage(messages: ChatMessage[]) {
  return [...messages].reverse().find((message) => message.role === 'user')
}

export function createFallbackChatResponse(
  request: ChatRequest,
  runtimeStatus: RuntimeStatus,
  latestUserMessage: ChatMessage
): ChatResponse {
  return {
    conversationId: request.conversationId || randomUUID(),
    message: {
      id: randomUUID(),
      role: 'assistant',
      content: `Agent-Runtime is not configured yet. I received: "${latestUserMessage.content}". Set NUXT_AGENT_RUNTIME_URL to route this chat through the SurrealDB-backed runtime plane.`,
      createdAt: new Date().toISOString(),
      source: 'local-fallback'
    },
    runtime: runtimeStatus
  }
}

export async function sendToAgentRuntime(
  event: H3Event,
  request: ChatRequest,
  runtimeStatus: RuntimeStatus
): Promise<ChatResponse> {
  const config = useRuntimeConfig(event)
  const runtimeUrl = trimTrailingSlash(config.agentRuntimeUrl)
  const conversationId = request.conversationId || randomUUID()

  if (!runtimeUrl) {
    const latestUserMessage = getLatestUserMessage(request.messages)

    if (!latestUserMessage) {
      throw createError({
        statusCode: 400,
        statusMessage: 'A user message is required'
      })
    }

    return createFallbackChatResponse(
      { ...request, conversationId },
      runtimeStatus,
      latestUserMessage
    )
  }

  const response = await requestAgentRuntime(event, runtimeUrl, {
    ...request,
    conversationId
  }, runtimeStatus)

  return normalizeRuntimeResponse(response, conversationId, runtimeStatus)
}

async function requestAgentRuntime(
  event: H3Event,
  runtimeUrl: string,
  request: ChatRequest,
  runtimeStatus: RuntimeStatus
) {
  const config = useRuntimeConfig(event)
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
    return await $fetch<RuntimeChatResponse>(`${runtimeUrl}/v1/chat`, {
      method: 'POST',
      timeout: 30000,
      headers,
      body: {
        conversationId: request.conversationId,
        messages: request.messages,
        context: {
          graphSchema: runtimeStatus.graphSchema,
          runtimeRepository: runtimeStatus.runtimeRepository,
          surrealdbConfigured: runtimeStatus.surrealdbConfigured
        }
      }
    })
  } catch (err) {
    throw createError({
      statusCode: 502,
      statusMessage: err instanceof Error ? err.message : 'Agent-Runtime request failed'
    })
  }
}

function normalizeRuntimeResponse(
  response: RuntimeChatResponse,
  conversationId: string,
  runtimeStatus: RuntimeStatus
): ChatResponse {
  const message = response.message || response.messages?.at(-1)

  if (!message?.content) {
    throw createError({
      statusCode: 502,
      statusMessage: 'Agent-Runtime returned no assistant message'
    })
  }

  return {
    conversationId: response.conversationId || conversationId,
    message: {
      id: message.id || randomUUID(),
      role: message.role || 'assistant',
      content: message.content,
      createdAt: message.createdAt || new Date().toISOString(),
      source: message.source || 'Agent-Runtime'
    },
    runtime: {
      ...runtimeStatus,
      runtimeConfigured: true
    }
  }
}
