<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import type { ChatMessage, ChatResponse } from '~~/shared/types/chat'

const config = useRuntimeConfig()
const input = ref('')
const isSending = ref(false)
const error = ref('')
const conversationId = ref<string | undefined>()
const messageList = ref<ChatMessage[]>([
  {
    id: 'welcome',
    role: 'assistant',
    content: 'Agent Chat is ready. Connect Agent-Runtime to answer through the SurrealDB-backed Agent-Graph runtime plane.',
    createdAt: new Date().toISOString(),
    source: 'local'
  }
])

const messagesEl = ref<HTMLElement | null>(null)

const serviceItems = computed(() => [
  { label: 'Auth', value: config.public.agentAuthUrl ? 'configured' : 'pending' },
  { label: 'Identity', value: config.public.agentIdentityUrl ? 'configured' : 'pending' },
  { label: 'Graph', value: config.public.agentGraphSchema },
  { label: 'Runtime', value: conversationId.value ? 'active' : 'ready' }
])

async function sendMessage() {
  const content = input.value.trim()

  if (!content || isSending.value) {
    return
  }

  error.value = ''
  input.value = ''

  const userMessage: ChatMessage = {
    id: crypto.randomUUID(),
    role: 'user',
    content,
    createdAt: new Date().toISOString()
  }

  messageList.value.push(userMessage)
  isSending.value = true
  await scrollToBottom()

  try {
    const response = await $fetch<ChatResponse>('/api/chat', {
      method: 'POST',
      body: {
        conversationId: conversationId.value,
        messages: messageList.value
      }
    })

    conversationId.value = response.conversationId
    messageList.value.push(response.message)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Message failed'
  } finally {
    isSending.value = false
    await scrollToBottom()
  }
}

async function scrollToBottom() {
  await nextTick()
  messagesEl.value?.scrollTo({ top: messagesEl.value.scrollHeight, behavior: 'smooth' })
}
</script>

<template>
  <main class="min-h-screen bg-[#f6f7f9] text-[#14171f]">
    <div class="mx-auto flex min-h-screen w-full max-w-7xl flex-col px-4 py-4 sm:px-6 lg:px-8">
      <header class="flex flex-col gap-3 border-b border-[#d8dde7] pb-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p class="text-xs font-semibold uppercase text-[#4c6f7f]">AGenNext</p>
          <h1 class="text-2xl font-semibold tracking-normal text-[#14171f]">Agent Chat</h1>
        </div>

        <div class="flex flex-wrap gap-2">
          <UBadge
            v-for="item in serviceItems"
            :key="item.label"
            variant="soft"
            color="neutral"
            class="rounded-md"
          >
            {{ item.label }}: {{ item.value }}
          </UBadge>
        </div>
      </header>

      <section class="grid flex-1 gap-4 py-4 lg:grid-cols-[280px_1fr]">
        <aside class="border-r border-[#d8dde7] pr-0 lg:pr-4">
          <div class="space-y-4">
            <div>
              <h2 class="text-sm font-semibold text-[#14171f]">Runtime Plane</h2>
              <p class="mt-2 text-sm leading-6 text-[#56606f]">
                Nuxt talks to Agent-Runtime through the local API proxy. Runtime keeps state in the SurrealDB plane and uses Agent-Graph as the schema contract.
              </p>
            </div>

            <div class="space-y-2 text-sm">
              <div class="flex items-center justify-between border-b border-[#d8dde7] py-2">
                <span class="text-[#56606f]">Authentication</span>
                <span class="font-medium text-[#14171f]">Agent-Auth</span>
              </div>
              <div class="flex items-center justify-between border-b border-[#d8dde7] py-2">
                <span class="text-[#56606f]">Identity</span>
                <span class="font-medium text-[#14171f]">Agent-Identity</span>
              </div>
              <div class="flex items-center justify-between border-b border-[#d8dde7] py-2">
                <span class="text-[#56606f]">Schema</span>
                <span class="font-medium text-[#14171f]">Agent-Graph</span>
              </div>
              <div class="flex items-center justify-between border-b border-[#d8dde7] py-2">
                <span class="text-[#56606f]">Backend</span>
                <span class="font-medium text-[#14171f]">SurrealDB</span>
              </div>
            </div>
          </div>
        </aside>

        <div class="flex min-h-[calc(100vh-9rem)] flex-col">
          <div
            ref="messagesEl"
            class="flex-1 space-y-4 overflow-y-auto rounded-md border border-[#d8dde7] bg-white p-4"
          >
            <div
              v-for="message in messageList"
              :key="message.id"
              class="flex gap-3"
              :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <div
                class="max-w-[min(760px,85%)] rounded-md px-4 py-3 text-sm leading-6"
                :class="message.role === 'user'
                  ? 'bg-[#143d59] text-white'
                  : 'border border-[#d8dde7] bg-[#f9fafb] text-[#14171f]'"
              >
                <p class="whitespace-pre-wrap break-words">{{ message.content }}</p>
                <p
                  v-if="message.source"
                  class="mt-2 text-xs"
                  :class="message.role === 'user' ? 'text-[#d9edf5]' : 'text-[#6b7280]'"
                >
                  {{ message.source }}
                </p>
              </div>
            </div>
          </div>

          <p v-if="error" class="mt-3 text-sm text-red-600">{{ error }}</p>

          <form class="mt-4 flex gap-3" @submit.prevent="sendMessage">
            <UTextarea
              v-model="input"
              :rows="2"
              autoresize
              placeholder="Message Agent-Runtime..."
              class="min-w-0 flex-1"
              @keydown.enter.exact.prevent="sendMessage"
            />
            <UButton
              type="submit"
              icon="i-lucide-send"
              :loading="isSending"
              :disabled="!input.trim()"
              class="h-11 self-end rounded-md"
            >
              Send
            </UButton>
          </form>
        </div>
      </section>
    </div>
  </main>
</template>
