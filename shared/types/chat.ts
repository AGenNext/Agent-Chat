import type { RuntimeStatus } from './runtime'

export type ChatRole = 'system' | 'user' | 'assistant'

export interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  createdAt?: string
  source?: string
}

export interface ChatRequest {
  conversationId?: string
  messages: ChatMessage[]
}

export interface ChatResponse {
  conversationId: string
  message: ChatMessage
  runtime: RuntimeStatus
}
