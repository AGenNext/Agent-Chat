import type { ChatRequest, ChatResponse } from '~~/shared/types/chat'

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

  return await sendToAgentRuntime(event, {
    conversationId: body.conversationId,
    messages
  }, runtimeStatus)
})
